.PHONY: build build-full build-api build-cli build-web run-api run-cli clean dev-api dev-web install-deps tidy test test-verbose

BINARY_API=bin/reconx-api
BINARY_CLI=bin/reconx

# Build all (Go only — no frontend rebuild)
build: build-api build-cli

# Full build: web + api + cli
build-full: build-web build-api build-cli

build-web:
	cd web && npm run build

build-api:
	go build -o $(BINARY_API) ./cmd/api

build-cli:
	go build -o $(BINARY_CLI) ./cmd/cli

# Run
run-api: build-api
	./$(BINARY_API)

run-cli: build-cli
	./$(BINARY_CLI)

# Dev (hot reload — run both in separate terminals)
dev-api:
	air -c .air.toml

dev-web:
	cd web && npm run dev

# Dependencies
install-deps:
	go mod tidy
	cd web && npm install

tidy:
	go mod tidy

# Clean
clean:
	rm -rf bin/
	rm -rf web/dist/

# Test
test:
	go test ./...

test-verbose:
	go test -v ./...
