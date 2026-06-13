.PHONY: build run test vet lint tidy docker dev-up dev-down
BIN ?= bin/gateway

build:
	go build -o $(BIN) ./services/gateway/cmd/gateway

run:
	go run ./services/gateway/cmd/gateway

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

dev-down:
	docker compose -f deploy/dev/compose.yaml down -v
