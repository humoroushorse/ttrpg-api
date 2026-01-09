.PHONY: help run build test clean

help:
	@echo "Available targets:"
	@echo "  run    - Run the auth service locally"
	@echo "  build  - Build the auth service binary"
	@echo "  test   - Run tests"
	@echo "  clean  - Clean build artifacts"

run:
	@echo "Running auth service..."
	go run cmd/server/main.go

build:
	@echo "Building auth service..."
	go build -o bin/auth-server cmd/server/main.go

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning..."
	rm -rf bin/
