#!/bin/bash

# Quka AI 后端多架构二进制构建脚本
# 用法: ./build.sh [架构] [选项]
# 
# 示例:
#   ./build.sh                          # 构建默认架构 (linux/amd64,linux/arm64)
#   ./build.sh linux/amd64              # 仅构建 AMD64
#   ./build.sh linux/arm64              # 仅构建 ARM64
#   ./build.sh linux/amd64,linux/arm64  # 构建两种架构
#   ./build.sh --help                   # 显示帮助信息

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [architectures] [options]"
    echo ""
    echo "Parameters:"
    echo "  architectures   Comma-separated list of target architectures (optional)"
    echo "                  Format: OS/ARCH (e.g., linux/amd64,linux/arm64)"
    echo "                  Default: linux/amd64,linux/arm64"
    echo "                  Supported: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64"
    echo ""
    echo "Options:"
    echo "  --tags TAG      Build tags to include (e.g., commercial,enterprise)"
    echo "  --output DIR    Output directory (default: _build)"
    echo "  --single        Build single binary without architecture suffix"
    echo "  --help, -h      Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  BUILD_TAGS      Build tags (can be overridden by --tags)"
    echo "  BUILD_DIR       Build output directory (can be overridden by --output)"
    echo ""
    echo "Examples:"
    echo "  $0                              # Build for default architectures"
    echo "  $0 linux/amd64                 # Build for AMD64 only"
    echo "  $0 linux/amd64,linux/arm64     # Build for both architectures"
    echo "  $0 --tags commercial           # Build with commercial features"
    echo "  $0 --output ./dist --single    # Build single binary to ./dist"
}

# Change to project root directory
cd "$(dirname "$0")"
cd ../

# Default values
SERVICE="quka"
SUB_SERVICE="service"
BUILD_DIR=${BUILD_DIR:-${PWD}/_build}
BUILD_TAGS=${BUILD_TAGS:-""}
ARCHITECTURES=""
SINGLE_BINARY=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --tags)
            BUILD_TAGS="$2"
            shift 2
            ;;
        --output)
            BUILD_DIR="$2"
            shift 2
            ;;
        --single)
            SINGLE_BINARY=true
            shift
            ;;
        --help|-h)
            show_usage
            exit 0
            ;;
        -*)
            log_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
        *)
            if [ -z "$ARCHITECTURES" ]; then
                ARCHITECTURES="$1"
            else
                log_error "Multiple architecture arguments provided"
                show_usage
                exit 1
            fi
            shift
            ;;
    esac
done

# Set default architectures if not provided
if [ -z "$ARCHITECTURES" ]; then
    ARCHITECTURES="linux/amd64,linux/arm64"
fi

# Parse architectures
IFS=',' read -ra ARCH_ARRAY <<< "$ARCHITECTURES"

log_info "Starting Quka AI binary build"
log_info "Architectures: $ARCHITECTURES"
log_info "Build tags: ${BUILD_TAGS:-none}"
log_info "Output directory: $BUILD_DIR"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    log_error "Go is not installed or not in PATH"
    exit 1
fi

# Create build directory
mkdir -p "$BUILD_DIR"

# Validate architectures and build
for arch in "${ARCH_ARRAY[@]}"; do
    # Remove leading/trailing whitespace
    arch=$(echo "$arch" | xargs)
    
    IFS='/' read -ra arch_parts <<< "$arch"
    if [ ${#arch_parts[@]} -ne 2 ]; then
        log_error "Invalid architecture format: $arch (expected OS/ARCH)"
        exit 1
    fi
    
    goos="${arch_parts[0]}"
    goarch="${arch_parts[1]}"
    
    # Validate supported architectures
    case "$goos/$goarch" in
        linux/amd64|linux/arm64|darwin/amd64|darwin/arm64|windows/amd64)
            ;;
        *)
            log_warning "Architecture $goos/$goarch may not be fully supported"
            ;;
    esac
    
    log_info "Building for $goos/$goarch..."
    
    # Set output file name
    if [ "$SINGLE_BINARY" = "true" ] || [ "${#ARCH_ARRAY[@]}" -eq 1 ]; then
        if [ "$goos" = "windows" ]; then
            output_name="${SERVICE}.exe"
        else
            output_name="${SERVICE}"
        fi
    else
        if [ "$goos" = "windows" ]; then
            output_name="${SERVICE}-${goos}-${goarch}.exe"
        else
            output_name="${SERVICE}-${goos}-${goarch}"
        fi
    fi
    
    # Prepare build command
    build_cmd="CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build"
    
    # Add build tags if specified
    if [ -n "$BUILD_TAGS" ]; then
        build_cmd="$build_cmd -tags \"$BUILD_TAGS\""
    fi
    
    # Add build flags
    build_cmd="$build_cmd -a -ldflags '-extldflags \"-static\"'"
    build_cmd="$build_cmd -o \"${BUILD_DIR}/${output_name}\" ./cmd/"
    
    # Execute build command
    if ! eval "$build_cmd"; then
        log_error "Failed to build for $goos/$goarch"
        exit 1
    fi
    
    log_success "Built ${output_name}"
done

# Create default symbolic link for single architecture builds
if [ "${#ARCH_ARRAY[@]}" -eq 1 ] && [ "$SINGLE_BINARY" = "false" ]; then
    arch="${ARCH_ARRAY[0]}"
    IFS='/' read -ra arch_parts <<< "$arch"
    goos="${arch_parts[0]}"
    goarch="${arch_parts[1]}"
    
    if [ "$goos" = "windows" ]; then
        arch_output="${SERVICE}-${goos}-${goarch}.exe"
        default_output="${SERVICE}.exe"
    else
        arch_output="${SERVICE}-${goos}-${goarch}"
        default_output="${SERVICE}"
    fi
    
    if [ -f "${BUILD_DIR}/${arch_output}" ] && [ "${arch_output}" != "${default_output}" ]; then
        cd "$BUILD_DIR"
        ln -sf "${arch_output}" "${default_output}"
        cd - > /dev/null
        log_info "Created symbolic link: ${default_output} -> ${arch_output}"
    fi
fi

# Copy configuration files
log_info "Copying configuration files..."
mkdir -p "${BUILD_DIR}/etc"
if [ -d "./cmd/${SUB_SERVICE}/etc/" ]; then
    cp -r "./cmd/${SUB_SERVICE}/etc/" "${BUILD_DIR}/etc/"
    log_info "Configuration files copied to ${BUILD_DIR}/etc/"
else
    log_warning "Configuration directory ./cmd/${SUB_SERVICE}/etc/ not found"
fi

# Show build results
log_info "Build artifacts:"
if command -v ls &> /dev/null; then
    ls -la "${BUILD_DIR}/" | grep -E "(${SERVICE}|^total)" || true
fi

log_success "Build completed successfully!"
log_info "Binaries available in: ${BUILD_DIR}/"

# Show next steps
echo ""
log_info "To run the service:"
if [ "${#ARCH_ARRAY[@]}" -eq 1 ]; then
    arch="${ARCH_ARRAY[0]}"
    IFS='/' read -ra arch_parts <<< "$arch"
    goos="${arch_parts[0]}"
    if [ "$goos" = "$(uname -s | tr '[:upper:]' '[:lower:]')" ]; then
        echo "  ${BUILD_DIR}/${SERVICE} service -c ${BUILD_DIR}/etc/service-default.toml"
    else
        echo "  # Copy ${BUILD_DIR}/${SERVICE} to target ${goos} system and run:"
        echo "  ${SERVICE} service -c etc/service-default.toml"
    fi
else
    echo "  # Choose the appropriate binary for your platform and run:"
    echo "  ${BUILD_DIR}/${SERVICE}-<os>-<arch> service -c ${BUILD_DIR}/etc/service-default.toml"
fi