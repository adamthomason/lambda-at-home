#!/usr/bin/env bash

set -e

# Configuration
IMAGE_NAME="${1}"
SIZE="${2}"
SOURCE_DIR="/workspace/dist/$IMAGE_NAME"
ARTIFACT_PATH="/workspace/artifacts/$IMAGE_NAME.ext4"
MOUNT_PATH="/tmp/$IMAGE_NAME"

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

if [ -f "$ARTIFACT_PATH" ] ; then
    rm "$ARTIFACT_PATH"
fi

# Auto-calculate size from source directory if not explicitly provided
if [ -z "$SIZE" ]; then
    MEASURED_MB=$(du -sm "$SOURCE_DIR" | cut -f1)
    SIZE=$(( MEASURED_MB * 12 / 10 + 16 ))
    log "Auto-calculated image size: ${SIZE}MB (source: ${MEASURED_MB}MB + 20% overhead + 16MB ext4 metadata)"
fi

log "Creating and mounting base ext4 image"
dd if=/dev/zero of=$ARTIFACT_PATH bs=1M count=$SIZE
mkfs.ext4 $ARTIFACT_PATH
mkdir $MOUNT_PATH
sudo mount $ARTIFACT_PATH $MOUNT_PATH

log "Copying contents from $SOURCE_DIR to $MOUNT_PATH"
sudo cp -a "$SOURCE_DIR/." "$MOUNT_PATH/"

log "Image creation complete: $ARTIFACT_PATH"
