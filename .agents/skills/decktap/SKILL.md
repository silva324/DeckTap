---
name: decktap
description: Developer skill and architectural guide for AI coding agents developing, debugging, and testing DeckTap.
---

# DeckTap Developer Skill

This skill guides AI agents working on **DeckTap**, a Zero-Trust E2EE presentation remote, notes reader, and virtual laser pointer built with Go 1.23+ and vanilla modern JavaScript.

## 1. Quick Commands
```bash
# Build binary
go build -o bin/pptx-remote ./cmd/pptx-remote

# Run all unit and integration tests
go test -v ./...

# Run diagnostic doctor (verifies cloudflared, PowerPoint permissions, notes extraction)
./bin/pptx-remote doctor

# Run with demo deck (offline mode, no PowerPoint required)
./bin/pptx-remote -mode local -no-browser -demo internal/pptx/testdata/demo.pptx

# Check JavaScript syntax
node -c internal/webui/static/app.js internal/webui/static/crypto.js
```

## 2. Key Architecture Invariants
- `internal/e2ee/`: AES-256-GCM cipher with AAD direction binding (`ClientToHost` vs `HostToClient`). Ephemeral secrets passed exclusively via URI `#fragment` (RFC 3986).
- `internal/session/`: Session state machine with Anti-Shoulder-Surfing PIN verification (`AUTH_HELLO`), session resumption (`AUTH_RESUME`), and brute-force rate-limiting.
- `internal/pointer/`: Transparent on-screen laser pointer and spotlight Cocoa overlay (`pointer_darwin.go`) with 16:9 normalized reticle coordinates.
- `internal/nativewin/`: Native host pairing window displaying embedded QR code, PIN, connection status, and active peers.
- `internal/updater/`: Autonomous GitHub Releases update checker with semver comparison and desktop update prompt banners.
- `internal/presenter/`: Native presentation automation:
  - `mac.go`: macOS AppleScript automation with `if application "Microsoft PowerPoint" is not running then return "0"` guard.
  - `windows.go`: Windows PowerShell COM automation.
  - `demo.go`: Synthetic demo controller using `demo.pptx`.
- `internal/tunnel/`: Autonomous `cloudflared` auto-downloader and edge registration monitor.
- `internal/webui/static/`: Vanilla PWA embedded via `//go:embed`. Features In-App Camera QR scanner (`BarcodeDetector`), fixed non-scrolling bars, isolated notes scrolling, and OLED dark mode.

## 3. Contribution Rules & Branching Workflow (Mandatory for AI Agents)
- **NEVER commit directly to `main`:**
  1. Always branch off `main`: `git checkout -b feat/<feature-name>` (or `fix/<bug-name>`).
  2. Implement changes with atomic, well-tested commits.
  3. Verify locally: `go test ./...` and `node -c internal/webui/static/app.js`.
  4. Push your branch: `git push -u origin feat/<feature-name>`.
  5. Open a Pull Request targeting `main` for review.
- **Always use Conventional Commits:** Format all commit messages and PR titles strictly (`feat:`, `fix:`, `perf:`, `docs:`, `chore:`, `refactor:`) so Google's `release-please` can compute version bumps and generate changelogs automatically.
- **Never leak secrets or notes:** Ephemeral keys, pairing PINs, and presentation content must never be logged to console or exposed unencrypted.
