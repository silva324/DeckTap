#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo "0.0.1")}"
DIST_DIR="${ROOT_DIR}/dist"
mkdir -p "${DIST_DIR}/bin"

echo "==> Building DeckTap ${VERSION} for Linux..."

# 1. linux/amd64
echo "==> Compiling linux/amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o "${DIST_DIR}/bin/decktap-linux-amd64" \
    ./cmd/decktap

# 2. linux/arm64
echo "==> Compiling linux/arm64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -buildvcs=false \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o "${DIST_DIR}/bin/decktap-linux-arm64" \
    ./cmd/decktap

# 3. Create Tarballs
echo "==> Packaging Linux Tarballs..."
(
    cd "${DIST_DIR}/bin"
    tar -czf "../DeckTap-${VERSION}-linux-amd64.tar.gz" decktap-linux-amd64
    tar -czf "../DeckTap-${VERSION}-linux-arm64.tar.gz" decktap-linux-arm64
)

echo "==> Successfully created Linux packages:"
echo "    AMD64: ${DIST_DIR}/DeckTap-${VERSION}-linux-amd64.tar.gz"
echo "    ARM64: ${DIST_DIR}/DeckTap-${VERSION}-linux-arm64.tar.gz"
