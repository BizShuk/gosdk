.PHONY: build test generate run clean help

all: build

build:
	@echo "Building application..."
	go build -o bin/server main.go

test:
	@echo "Running tests..."
	go test -v ./...

generate:
	@echo "Generating code..."
	go generate ./...

run: build
	@echo "Running application..."
	./bin/server

clean:
	@echo "Cleaning up..."
	rm -rf bin/

help:
	@echo "Available targets:"
	@echo "  build    - Build the server binary"
	@echo "  test     - Run Go tests"
	@echo "  generate - Run go generate for stringer"
	@echo "  run      - Build and run the server"
	@echo "  clean    - Clean up build artifacts"
