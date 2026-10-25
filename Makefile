.PHONY: all build run test test-race vet fmt fmt-check clean frontend-dev frontend-build frontend-test frontend-browser-test e2e-test

# Default binary name and output directory
BINARY_NAME=ethiyo
BIN_DIR=bin

all: build

build:
	go build -v -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/server

run:
	go run ./cmd/server

test:
	go test -v ./...

test-race:
	go test -v -race ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Code formatting required on files:" && gofmt -l . && exit 1)

frontend-dev:
	npm --prefix frontend run dev

frontend-build:
	npm --prefix frontend run build

frontend-test:
	npm --prefix frontend test

frontend-browser-test:
	node frontend/browser-verification-suite.cjs

e2e-test:
	npm --prefix frontend run test:e2e

clean:
	go clean
	rm -rf $(BIN_DIR) coverage.out coverage.html frontend/dist
