#!/bin/bash
set -ex

# The paths below are relative to the repository root; make -C docreader proto runs from docreader/.
cd "$(dirname "$0")/../.."

# Directories
PROTO_DIR="docreader/proto"
PYTHON_OUT="docreader/proto"
GO_OUT="docreader/proto"

# Generate Python code
python3 -m grpc_tools.protoc -I${PROTO_DIR} \
    --python_out=${PYTHON_OUT} \
    --pyi_out=${PYTHON_OUT} \
    --grpc_python_out=${PYTHON_OUT} \
    ${PROTO_DIR}/docreader.proto

# Generate Go code (only when protoc-gen-go is available)
if command -v protoc-gen-go &> /dev/null; then
    protoc -I${PROTO_DIR} --go_out=${GO_OUT} \
        --go_opt=paths=source_relative \
        --go-grpc_out=${GO_OUT} \
        --go-grpc_opt=paths=source_relative \
        ${PROTO_DIR}/docreader.proto
else
    echo "protoc-gen-go not found, skipping Go code generation"
fi

# Fix the Python import path (macOS-compatible sed)
if [ "$(uname)" == "Darwin" ]; then
    # macOS
    sed -i '' 's/import docreader_pb2/from docreader.proto import docreader_pb2/g' ${PYTHON_OUT}/docreader_pb2_grpc.py
else
    # Linux
    sed -i 's/import docreader_pb2/from docreader.proto import docreader_pb2/g' ${PYTHON_OUT}/docreader_pb2_grpc.py
fi

echo "Proto files generated successfully!"