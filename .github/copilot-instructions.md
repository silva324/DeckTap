# GitHub Copilot Instructions for DeckTap

See [.cursorrules](file:///.cursorrules) for full guidelines.

## Core Rules:
1. **Zero-Trust E2EE:** All WebSocket frames MUST be encrypted using AES-256-GCM. Never log or transmit plaintext slide notes or control payloads.
2. **Secrets via URI Fragment:** The 256-bit AES MasterKey and 128-bit PairingToken are transmitted out-of-band via `#fragment` in the QR code (RFC 3986).
3. **Anti-Shoulder-Surfing PIN:** Mandatory constant-time comparison of the 4-digit visual PIN code (`crypto/subtle.ConstantTimeCompare`).
4. **Pure Vanilla Web Client:** No npm, Webpack, Vite, or external JS CDNs. Use browser SubtleCrypto and vanilla JS.
5. **AppleScript Safety:** Always guard with `if application "Microsoft PowerPoint" is not running then return "0"`. Avoid AppleScript reserved keywords (`out`, `st`, `nt`).
6. **Conventional Commits:** Use `feat:`, `fix:`, `docs:`, `chore:`, `perf:` for automated `release-please` versioning.
