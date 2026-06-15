.PHONY: build run run-gateway run-control-plane test vet lint tidy docker dev-up dev-down seed-keycloak seed-platform provision-tenant sandbox-image run-gateway-proof prove-isolation
BIN ?= bin/gateway

# Dev env for running the Go services against the local compose stack: points at
# the published ports, the local Keycloak realm, and exports traces to Jaeger.
DEV_ENV = MCP_BASE_DOMAIN=mcp.example.com MCP_LOG_FORMAT=console \
  MCP_POSTGRES_DSN='postgres://mcp:mcp@localhost:5432/mcp?sslmode=disable' \
  MCP_REDIS_ADDR=localhost:6379 MCP_VAULT_ADDR=http://localhost:8200 MCP_VAULT_TOKEN=dev-root \
  MCP_KEYCLOAK_ISSUER_TEMPLATE='http://localhost:8081/realms/%s' \
  MCP_OTLP_ENDPOINT=localhost:4318

build:
	go build -o $(BIN) ./services/gateway/cmd/gateway

# Data plane on :8080 (exec sandbox = UNSANDBOXED, dev only). `run` is an alias.
run: run-gateway
run-gateway:
	$(DEV_ENV) MCP_HTTP_ADDR=:8080 MCP_SANDBOX_RUNTIME=exec \
		MCP_RESOURCE_TEMPLATE='http://%s.mcp.example.com:8080/mcp' \
		go run ./services/gateway/cmd/gateway

# Admin/control plane on :8090 (CORS for the console; admin-API audience).
run-control-plane:
	$(DEV_ENV) MCP_HTTP_ADDR=:8090 \
		MCP_ADMIN_AUDIENCE=https://api.mcp.example.com MCP_CONSOLE_ORIGINS=http://localhost:5173 \
		go run ./services/control-plane/cmd/control-plane

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run

tidy:
	go mod tidy

docker:
	docker build -f services/gateway/Dockerfile -t acme/mcp-gateway:dev .

dev-up:
	docker compose -f deploy/dev/compose.yaml up -d
	@bash deploy/dev/seed-keycloak.sh || echo "⚠ Keycloak seed failed — run 'make seed-keycloak' once it is ready."

dev-down:
	docker compose -f deploy/dev/compose.yaml down -v

# Provision the dev Keycloak realm/client/user the admin console needs (idempotent).
seed-keycloak:
	@bash deploy/dev/seed-keycloak.sh

# Seed the platform realm + provisioner service account (003, DEV — prints the
# secret to export as MCP_KEYCLOAK_ADMIN_SECRET when running the control-plane).
seed-platform:
	@PLATFORM=1 bash deploy/dev/seed-keycloak.sh

# Provision a tenant via the platform API (003). The control-plane must be running
# with provisioning enabled and `make seed-platform` must have run. Usage:
#   make provision-tenant SLUG=globex NAME='Globex' ADMIN_EMAIL=ops@globex.example
provision-tenant:
	@OP=$$(curl -s http://localhost:8081/realms/$${PLATFORM_REALM:-_platform}/protocol/openid-connect/token \
	  -d grant_type=password -d client_id=mcp-platform -d username=$${OPERATOR:-operator} -d password=$${OPERATOR_PW:-operator} -d scope=openid \
	  | python3 -c 'import sys,json;print(json.load(sys.stdin).get("access_token",""))'); \
	[ -n "$$OP" ] || { echo "no operator token — run 'make seed-platform' and start the control-plane"; exit 1; }; \
	curl -s -X POST http://localhost:8090/v1/platform/tenants -H "Authorization: Bearer $$OP" \
	  -H 'Content-Type: application/json' \
	  -d "{\"slug\":\"$(SLUG)\",\"display_name\":\"$(NAME)\",\"admin_email\":\"$(ADMIN_EMAIL)\"}"; echo

# --- 004 Two-Tenant AWS-MCP Isolation Proof ----------------------------------
# Build the sandbox image with the AWS MCP server + AWS CLI pre-baked (004).
# Under gVisor/Lima this must build in the sandbox Docker daemon (set DOCKER_HOST
# to the Lima VM, or run `bash .claude/skills/dev-setup/scripts/sandbox-up.sh`).
sandbox-image:
	docker build -t $${MCP_SANDBOX_IMAGE:-acme/mcp-sandbox:dev} deploy/sandbox-images

# Run the gateway in PROOF mode: real gVisor sandbox + the emulator-only egress
# allowlist. Requires a gVisor Docker daemon (Lima on macOS) and the
# `mcp-sandbox-egress` network + ministack present in that daemon.
run-gateway-proof:
	$(DEV_ENV) MCP_HTTP_ADDR=:8080 MCP_SANDBOX_RUNTIME=gvisor \
		MCP_SANDBOX_EGRESS_NETWORK=mcp-sandbox-egress \
		MCP_RESOURCE_TEMPLATE='http://%s.mcp.example.com:8080/mcp' \
		go run ./services/gateway/cmd/gateway

# Run the end-to-end isolation proof harness (one command — FR-015/SC-001).
# Prereqs: dev stack up, `make seed-platform`, gVisor gateway via `run-gateway-proof`,
# ministack reachable. Pass flags via ARGS, e.g. ARGS="--keep --stress-seconds 60".
prove-isolation:
	@npm --prefix tests/isolation-proof install --no-audit --no-fund --silent
	@npm --prefix tests/isolation-proof start -- $(ARGS)
