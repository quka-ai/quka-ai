# Proto Files

This directory contains Protocol Buffer definitions for external system integrations.

## Structure

```
proto/
├── [system_name]/     # Directory for each external system
│   ├── *.proto       # Proto definition files
│   └── *.pb.go       # Generated Go code
├── generate.sh       # Code generation script
└── README.md         # This file
```

## Usage

1. Place your `.proto` files in system-specific subdirectories
2. Run the generation script:
   ```bash
   ./generate.sh                    # Generate all proto files
   ./generate.sh path/to/file.proto # Generate specific file
   ```

## Prerequisites

Make sure you have the following tools installed:

- `protoc` - Protocol Buffer compiler
- `protoc-gen-go` - Go plugin for protoc
- `protoc-gen-go-grpc` - gRPC Go plugin for protoc

Install them with:
```bash
# macOS
brew install protobuf

# Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

## Example

```bash
# Create a directory for your system
mkdir -p pkg/proto/external_system

# Add your proto files
cp your_api.proto pkg/proto/external_system/

# Generate Go code
./pkg/proto/generate.sh
```