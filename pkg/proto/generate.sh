#!/bin/bash

# Proto file generation script
# Usage: ./generate.sh [proto_file_path]

set -e

PROTO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${PROTO_DIR}/../.." && pwd)"

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed. Please install Protocol Buffers compiler."
    echo "  macOS: brew install protobuf"
    echo "  Ubuntu: apt-get install protobuf-compiler"
    exit 1
fi

# Check if protoc-gen-go is installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Error: protoc-gen-go is not installed. Please install it:"
    echo "  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    exit 1
fi

# Check if protoc-gen-go-grpc is installed
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Error: protoc-gen-go-grpc is not installed. Please install it:"
    echo "  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
    exit 1
fi

if [ $# -eq 0 ]; then
    echo "Generating all proto files..."
    find "${PROTO_DIR}" -name "*.proto" -exec protoc \
        --proto_path="${PROTO_DIR}" \
        --go_out="${PROTO_DIR}" \
        --go-grpc_out="${PROTO_DIR}" \
        --go_opt=paths=source_relative \
        --go-grpc_opt=paths=source_relative \
        {} \;
else
    echo "Generating proto file: $1"
    protoc \
        --proto_path="${PROTO_DIR}" \
        --go_out="${PROTO_DIR}" \
        --go-grpc_out="${PROTO_DIR}" \
        --go_opt=paths=source_relative \
        --go-grpc_opt=paths=source_relative \
        "$1"
fi

echo "Proto files generated successfully!"