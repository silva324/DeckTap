# GitHub Copilot Instructions for DeckTap

See [.cursorrules](file:///.cursorrules) for full guidelines.

## Core Rules:
1. **Never Commit to `main`:** Always create a feature branch (`feat/...`, `fix/...`) and open a Pull Request. Direct pushes to `main` are strictly forbidden.
2. **Zero-Trust E2EE:** All WebSocket frames MUST be encrypted using AES-256-GCM. Never log or transmit plaintext slide notes or control payloads.
3. **Secrets via URI Fragment:** The 256-bit AES MasterKey and 128-bit PairingToken are transmitted out-of-band via `#fragment` in the QR code (RFC 3986).
4. **Anti-Shoulder-Surfing PIN:** Mandatory constant-time comparison of the 4-digit visual PIN code (`crypto/subtle.ConstantTimeCompare`).
5. **Pure Vanilla Web Client:** No npm, Webpack, Vite, or external JS CDNs. Use browser SubtleCrypto and vanilla JS.
6. **AppleScript Safety:** Always guard with `if application "Microsoft PowerPoint" is not running then return "0"`. Avoid AppleScript reserved keywords (`out`, `st`, `nt`).
7. **Conventional Commits:** Use `feat:`, `fix:`, `docs:`, `chore:`, `perf:` for automated `release-please` versioning.
8. **Mandatory Pre-Commit Linter:** Always execute `./scripts/agent-verify.sh` (or `make verify`) before committing or opening a PR to catch syntax errors, unformatted Go, or failing tests.
