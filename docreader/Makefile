.PHONY: proto build run docker-build docker-run clean

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@sh ./scripts/generate_proto.sh

# Build the Go client
build:
	@echo "Building Go client..."
	@go build -o bin/client ./src/client

# Run the Python service
run:
	@echo "Running Python server..."
	@python src/server/server.py

# Clean up
clean:
	@echo "Cleaning up..."
	@rm -rf bin/
	@find . -name "*.pyc" -delete
	@find . -name "__pycache__" -delete 