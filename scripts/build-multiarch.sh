#!/bin/bash

# Quka AI 后端多架构 Docker 构建脚本
# 用法: ./build-image.sh <镜像项目> <版本> [架构] [--push]
# 
# 示例:
#   ./build-image.sh myregistry/myproject v1.0.0 amd64
#   ./build-image.sh myregistry/myproject v1.0.0 arm64 --push
#   ./build-image.sh myregistry/myproject v1.0.0 both --push
#   ./build-image.sh myregistry/myproject v1.0.0 both --local

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
    echo "Usage: $0 <image_project> <version> [architecture] [options]"
    echo ""
    echo "Parameters:"
    echo "  image_project    Docker registry project (e.g., myregistry/myproject)"
    echo "  version         Image version tag (e.g., v1.0.0, latest)"
    echo "  architecture    Target architecture (optional, default: both)"
    echo "                  - amd64: Build for x86_64 architecture"
    echo "                  - arm64: Build for ARM64 architecture"
    echo "                  - both: Build for both architectures"
    echo ""
    echo "Options:"
    echo "  --push          Push images to registry after building"
    echo "  --local         Build and load images locally (default behavior)"
    echo "  --no-cache      Build without using cache"
    echo "  --dockerfile    Dockerfile path (default: ./Dockerfile)"
    echo "  --help, -h      Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 myregistry/myproject v1.0.0 amd64"
    echo "  $0 myregistry/myproject v1.0.0 arm64 --push"
    echo "  $0 myregistry/myproject v1.0.0 both --push"
    echo "  $0 myregistry/myproject v1.0.0 both --dockerfile ./Dockerfile.prod --push"
}

# Parse command line arguments
IMAGE_PROJECT=$1
VERSION=$2
ARCHITECTURE=${3:-both}
IMAGE_NAME=quka
PUSH_IMAGES=false
USE_CACHE=true
DOCKERFILE_PATH="./Dockerfile"

# Check for required parameters
if [ -z "$IMAGE_PROJECT" ] || [ -z "$VERSION" ]; then
    log_error "Missing required parameters"
    show_usage
    exit 1
fi

# Parse options
shift 2
if [ $# -gt 0 ] && [[ "$1" != --* ]]; then
    shift 1  # Skip architecture parameter if provided
fi

while [[ $# -gt 0 ]]; do
    case $1 in
        --push)
            PUSH_IMAGES=true
            shift
            ;;
        --local)
            PUSH_IMAGES=false
            shift
            ;;
        --no-cache)
            USE_CACHE=false
            shift
            ;;
        --dockerfile)
            DOCKERFILE_PATH="$2"
            shift 2
            ;;
        --help|-h)
            show_usage
            exit 0
            ;;
        push)
            # 兼容旧版本参数
            PUSH_IMAGES=true
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Constants
FULL_IMAGE_NAME=${IMAGE_PROJECT}/${IMAGE_NAME}:${VERSION}

# Map architecture names to Docker platform strings
case $ARCHITECTURE in
    amd64|x86_64)
        PLATFORMS="linux/amd64"
        ;;
    arm64|aarch64)
        PLATFORMS="linux/arm64"
        ;;
    both|all)
        PLATFORMS="linux/amd64,linux/arm64"
        ;;
    *)
        log_error "Unsupported architecture: $ARCHITECTURE"
        log_error "Supported architectures: amd64, arm64, both"
        exit 1
        ;;
esac

log_info "Starting multi-architecture Docker build"
log_info "Image: $FULL_IMAGE_NAME"
log_info "Platforms: $PLATFORMS"
log_info "Dockerfile: $DOCKERFILE_PATH"
log_info "Push to registry: $PUSH_IMAGES"

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    exit 1
fi

# Check if docker buildx is available
if ! docker buildx version &> /dev/null; then
    log_error "Docker buildx is not available. Please update Docker to a version that supports buildx."
    exit 1
fi

# Check if Dockerfile exists
if [ ! -f "$DOCKERFILE_PATH" ]; then
    log_error "Dockerfile not found: $DOCKERFILE_PATH"
    exit 1
fi

# Create and use a new buildx builder if it doesn't exist
BUILDER_NAME="quka-multiarch-builder"

if ! docker buildx ls | grep -q "$BUILDER_NAME"; then
    log_info "Creating new buildx builder: $BUILDER_NAME"
    docker buildx create --name "$BUILDER_NAME" --driver docker-container --use
else
    log_info "Using existing buildx builder: $BUILDER_NAME"
    docker buildx use "$BUILDER_NAME"
fi

# Ensure the builder is running
log_info "Bootstrapping buildx builder..."
docker buildx inspect --bootstrap

# Prepare build arguments
BUILD_ARGS=""
if [ "$USE_CACHE" = "false" ]; then
    BUILD_ARGS="$BUILD_ARGS --no-cache"
fi

# Add proxy settings if they exist
if [ -n "$HTTP_PROXY" ]; then
    BUILD_ARGS="$BUILD_ARGS --build-arg HTTP_PROXY=$HTTP_PROXY"
fi

if [ -n "$HTTPS_PROXY" ]; then
    BUILD_ARGS="$BUILD_ARGS --build-arg HTTPS_PROXY=$HTTPS_PROXY"
fi

# Build command
if [ "$PUSH_IMAGES" = "true" ]; then
    log_info "Building and pushing multi-architecture images..."
    docker buildx build \
        --platform "$PLATFORMS" \
        --tag "$FULL_IMAGE_NAME" \
        --file "$DOCKERFILE_PATH" \
        --push \
        $BUILD_ARGS \
        .
    
    log_success "Images built and pushed successfully!"
    log_info "Available platforms for $FULL_IMAGE_NAME:"
    docker buildx imagetools inspect "$FULL_IMAGE_NAME" | grep -E "(MediaType|Platform)" || log_warning "Could not inspect image platforms"
else
    log_info "Building images locally..."
    # For local builds with multiple platforms, try to build and fallback to single platform
    if [ "$ARCHITECTURE" = "both" ]; then
        log_info "Attempting to build for both platforms locally..."
        docker buildx build \
            --platform "$PLATFORMS" \
            --tag "$FULL_IMAGE_NAME" \
            --file "$DOCKERFILE_PATH" \
            --load \
            $BUILD_ARGS \
            . 2>/dev/null || {
            log_warning "Multi-platform local build failed, falling back to AMD64 only"
            docker buildx build \
                --platform "linux/amd64" \
                --tag "$FULL_IMAGE_NAME" \
                --file "$DOCKERFILE_PATH" \
                --load \
                $BUILD_ARGS \
                .
        }
    else
        docker buildx build \
            --platform "$PLATFORMS" \
            --tag "$FULL_IMAGE_NAME" \
            --file "$DOCKERFILE_PATH" \
            --load \
            $BUILD_ARGS \
            .
    fi
    
    log_success "Images built locally!"
fi

# Show built images
log_info "Available local images:"
docker images | grep "$FULL_IMAGE_NAME" | grep "$VERSION" || log_warning "No local images found (expected for pushed images)"

log_success "Multi-architecture build completed successfully!"

# Cleanup instructions
log_info "To clean up the builder later, run:"
log_info "docker buildx rm $BUILDER_NAME"
