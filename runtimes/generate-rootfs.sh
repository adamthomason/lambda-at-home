#!/usr/bin/env bash

set -e

# Configuration
IMAGE_NAME="${1}"
DOCKERFILE_PATH="${2}"
OUTPUT_DIR="./dist/$IMAGE_NAME"
CONTAINER_NAME="rootfs-export-$(echo "$IMAGE_NAME" | tr ':' '-')"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

cleanup() {
    log "Cleaning up container: $CONTAINER_NAME"
    docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
}

# Set up cleanup trap
trap cleanup EXIT

docker build --platform linux/amd64 -t ${IMAGE_NAME} -f ${DOCKERFILE_PATH} . || error "Docker build failed for $IMAGE_NAME"

# Create output directory
log "Creating output directory: $OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"
rm -rf "$OUTPUT_DIR"/*

# Create a temporary container from the image
log "Creating temporary container from image: $IMAGE_NAME"
docker create --name "$CONTAINER_NAME" --platform="linux/amd64" --privileged "$IMAGE_NAME" >/dev/null

# Export the container filesystem
log "Exporting container filesystem..."
docker export "$CONTAINER_NAME" | tar -xf - -C "$OUTPUT_DIR"

# Verify the export
if [ ! -d "$OUTPUT_DIR/bin" ] || [ ! -d "$OUTPUT_DIR/usr" ]; then
    error "Export appears incomplete - missing essential directories"
fi

# Set proper permissions (important for rootfs)
# log "Setting proper permissions..."
# sudo chown -R root:root "$OUTPUT_DIR" 2>/dev/null || {
#     warn "Could not set root ownership (requires sudo). This may cause issues in the VM."
# }

# Display filesystem size

ROOTFS_SIZE=$(du -sh "$OUTPUT_DIR" | cut -f1)
ROOTFS_SIZE_BYTES=$(numfmt --from=iec "$ROOTFS_SIZE")
IMAGE_PATH="./artifacts/$IMAGE_NAME.ext4"

log "Exported rootfs size: $ROOTFS_SIZE"
log "Rootfs exported to: $OUTPUT_DIR"

log "Creating rootfs image"
docker run --platform=linux/amd64 --rm --privileged -v "$PWD/dist:/workspace/dist" -v "$PWD/artifacts:/workspace/artifacts" generate-ext4 "$IMAGE_NAME"