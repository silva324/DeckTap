#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo "0.0.1")}"
DIST_DIR="${ROOT_DIR}/dist"
ZIP_NAME="DeckTap-${VERSION}-windows-amd64.zip"

echo "==> Building DeckTap ${VERSION} for Windows (x64)..."
mkdir -p "${DIST_DIR}/bin"

# 1. Compile Windows x64 binary with GUI subsystem (no console window)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -buildvcs=false \
    -ldflags "-s -w -H=windowsgui -X main.version=${VERSION}" \
    -o "${DIST_DIR}/bin/pptx-remote.exe" \
    ./cmd/pptx-remote

# 2. Package ZIP
echo "==> Packaging ZIP: ${ZIP_NAME}..."
rm -f "${DIST_DIR}/${ZIP_NAME}"
(
    cd "${DIST_DIR}/bin"
    zip -9 "../${ZIP_NAME}" pptx-remote.exe
)

echo "==> Successfully created:"
echo "    EXE: ${DIST_DIR}/bin/pptx-remote.exe"
echo "    ZIP: ${DIST_DIR}/${ZIP_NAME} ($(du -sh "${DIST_DIR}/${ZIP_NAME}" | cut -f1))"
