.PHONY: all build run test test-race vet fmt fmt-check clean

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

clean:
	go clean
	rm -rf $(BIN_DIR) coverage.out coverage.html
