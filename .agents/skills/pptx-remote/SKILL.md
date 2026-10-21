---
name: decktap
description: Guide for AI coding agents developing, debugging, and testing DeckTap.
---

# DeckTap Developer Skill

This skill guides AI agents working on **DeckTap**, a Zero-Trust E2EE presentation remote and speaker notes reader built in Go 1.23+ and vanilla JavaScript.

## 1. Quick Commands
```bash
# Build binary
go build -o bin/pptx-remote ./cmd/pptx-remote

# Run all unit and integration tests
go test -v ./...

# Run diagnostic doctor
./bin/pptx-remote doctor

# Run with demo deck (offline mode, no PowerPoint required)
./bin/pptx-remote -mode local -no-browser -demo internal/pptx/testdata/demo.pptx

# Check JavaScript syntax
node -c internal/webui/static/app.js internal/webui/static/crypto.js
```

## 2. Key Architecture Invariants
- `internal/e2ee/`: AES-256-GCM cipher with AAD direction binding (`ClientToHost` vs `HostToClient`).
- `internal/session/`: Session state machine with Anti-Shoulder-Surfing PIN verification (`AUTH_HELLO`), session rotation on resume (`AUTH_RESUME`), and brute-force lockout.
- `internal/presenter/`: Native presentation automation:
  - `mac.go`: macOS AppleScript automation with `if application "Microsoft PowerPoint" is not running then return "0"` guard.
  - `windows.go`: Windows PowerShell COM automation.
  - `demo.go`: Synthetic demo controller using `demo.pptx`.
- `internal/tunnel/`: Autonomous `cloudflared` auto-downloader and edge registration monitor.
- `internal/webui/static/`: Vanilla web application embedded via `//go:embed`. Zero external JS build tools.

## 3. Contribution Rules
- Always use Conventional Commits (`feat:`, `fix:`, `perf:`, etc.) for Google's `release-please` automation.
- Never leak plaintext notes or secrets in logs.
- Run `go test ./...` before committing.
