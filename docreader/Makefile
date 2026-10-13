.PHONY: proto build run clean

# Generate protobuf code with the locked grpcio-tools (the Go stubs also need protoc and its plugins on PATH)
proto:
	@echo "Generating protobuf code..."
	@uv run --locked bash ./scripts/generate_proto.sh

# Compile the Go client and proto packages (libraries, so nothing is written)
build:
	@echo "Building Go client..."
	@go build ./client ./proto

# Run the Python service (docreader must be importable as a package, so run from the repository root)
run:
	@echo "Running Python server..."
	@cd .. && uv run --project docreader python -m docreader.main

# Clean up
clean:
	@echo "Cleaning up..."
	@find . -name "*.pyc" -delete
	@find . -name "__pycache__" -type d -prune -exec rm -rf {} + 