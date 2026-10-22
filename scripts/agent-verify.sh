#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"

echo "=========================================="
echo "🤖 DeckTap Agent Verification & Lint Suite"
echo "=========================================="

# 1. Validate Vanilla JavaScript Syntax (Linter)
echo "==> 1. Running JavaScript Linter (node -c)..."
if ! command -v node >/dev/null 2>&1; then
    echo "❌ Error: Node.js is required to lint JavaScript files."
    exit 1
fi
node -c internal/webui/static/app.js internal/webui/static/crypto.js internal/webui/static/sw.js
echo "    ✓ JavaScript syntax is clean."

# 2. Check Go Formatting
echo "==> 2. Checking Go formatting (gofmt)..."
DIFF_FILES="$(gofmt -l . || true)"
if [ -n "${DIFF_FILES}" ]; then
    echo "❌ Error: Unformatted Go files detected:"
    echo "${DIFF_FILES}"
    echo "Run 'gofmt -w .' to fix."
    exit 1
fi
echo "    ✓ Go formatting is clean."

# 3. Run Go Vet (Static Analysis)
echo "==> 3. Running go vet..."
go vet ./...
echo "    ✓ Go vet passed."

# 4. Run Go Unit & Integration Tests
echo "==> 4. Running Go tests..."
go test ./...
echo "    ✓ All Go tests passed."

# 5. Verify Binary Compilation
echo "==> 5. Verifying Go binary build..."
go build -buildvcs=false -o /dev/null ./cmd/pptx-remote
echo "    ✓ Binary builds cleanly."

# 6. Git Hygiene Check
echo "==> 6. Checking git hygiene..."
if GIT_CONFIG_GLOBAL=/dev/null git ls-files | grep -i -E "\.ds_store|\.dmg$|\.zip$|\.exe$" >/dev/null 2>&1; then
    echo "❌ Error: Forbidden binaries or OS artifacts are tracked in Git!"
    exit 1
fi
echo "    ✓ Git hygiene is clean."

echo "=========================================="
echo "✅ All linter & verification gates passed!"
echo "=========================================="
