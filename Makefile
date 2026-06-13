.PHONY: build run run-gateway run-control-plane test vet lint tidy docker dev-up dev-down seed-keycloak
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
