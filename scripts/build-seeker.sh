#!/bin/bash
# Build key-seeker for Linux AMD64 (Debian Trixie)
# Run from Windows with WSL or on a Linux machine

set -e

echo "Building key-seeker for Linux AMD64..."

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
  -ldflags="-s -w" \
  -o bin/key-seeker-linux-amd64 \
  ./cmd/key-seeker

echo "Built: bin/key-seeker-linux-amd64"

# Optional: Build for ARM64 (Raspberry Pi, etc.)
echo "Building key-seeker for Linux ARM64..."

GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build \
  -ldflags="-s -w" \
  -o bin/key-seeker-linux-arm64 \
  ./cmd/key-seeker

echo "Built: bin/key-seeker-linux-arm64"

echo ""
echo "Copy to your Debian server:"
echo "  scp bin/key-seeker-linux-amd64 user@server:/usr/local/bin/key-seeker"
