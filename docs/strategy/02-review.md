# DeckTap — Production Readiness Review

> Reviewed at `8a200a1` (v0.11.1) · 2026-10-31
> Method: full read of the security-critical paths (`e2ee`, `session`, `app`, `admin`, `updater`, `tunnel`, `webui/static/crypto.js`), a survey of everything else, `go test -cover ./...`, and inspection of CI and release.

## Verdict

**The security core is solid. Distribution, connectivity and supply chain are not production-ready.** The session protocol shows real care: constant-time compares, single-use tokens, rotating session IDs, GCM AAD bound to direction, per-direction sequence numbers, and a loopback admin page protected by a path secret plus Host and Origin guards. The gaps are around that core: how the binary reaches users, how it updates itself, how it connects, and one availability bug that can end a live presentation.

All tests pass (`go test ./...`).

---

## P0: must fix before calling it "production"

### P0-1 · Anyone can kick the presenter mid-talk (lockout DoS)
`internal/session/session.go` `fail()`: every rejected *unauthenticated* frame increments `h.failures`. When it reaches `MaxFailures` (default 5), the hub switches to `LockedOut`, **closes the active, legitimately paired phone** (`kicked = h.active`) and tears down the tunnel (`app.onLockout`).

- **LAN mode** binds `0.0.0.0`. Anyone on the same conference Wi-Fi can send 5 garbage WebSocket frames and end the presentation.
- **Tunnel mode** binds loopback, but `allowedOrigin` accepts `null` and any `*.trycloudflare.com` origin. A sandboxed iframe on any web page the presenter opens can scan localhost ports and do the same thing.

**Fix:**
- Count pre-auth failures separately from post-auth failures. Rate-limit pre-auth failures per remote IP.
- Never evict an authenticated session because of a pre-auth failure. Lock out only *new pairing attempts*, and only while `Unpaired`.
- Add a regression test: "5 bad handshakes while paired → paired client unaffected".

### P0-2 · Self-updater is fail-open and unsigned
`internal/updater/selfupdate.go`:
- If `checksumsURL` is empty the check is skipped, and an asset missing from `checksums.txt` installs anyway (`return nil // asset not explicitly listed`).
- The checksum comes from the same place as the binary, so it only protects against corruption, not against tampering. Anyone who compromises the GitHub account or the release workflow can ship code to every installed copy.
- On macOS, replacing the executable inside a `.app` breaks its code signature. That will matter once builds are signed (P0-3).

**Fix:** sign releases with Ed25519 (minisign or `go-minisign`) and embed the public key in the binary. Fail closed when there is no signature or it doesn't verify. On macOS, update the whole bundle (Sparkle-style) or hand off to Homebrew; don't patch the executable inside the bundle.

### P0-3 · Builds are unsigned and not notarized
`scripts/package-macos.sh` ad-hoc signs (`codesign --sign -`), and the README tells users to run `xattr -cr`. For a product that sells *trust*, this is the single biggest drag on conversion. It also teaches users to bypass Gatekeeper.

**Fix:**
- macOS: Apple Developer ID ($99/yr), hardened runtime, notarization and stapling in CI.
- Windows: SignPath Foundation (free for OSS) or Azure Trusted Signing (about $10/mo).
- Publish to winget.

### P0-4 · `cloudflared` is downloaded unverified from `latest`
`internal/tunnel/cloudflared.go` fetches `releases/latest/download/<asset>`, then `chmod 755`s and runs it, with no version pin and no hash. A compromised upstream release, or a bad release, reaches every user silently.

**Fix:** pin a version and embed its SHA-256 per OS/arch. Bump it deliberately, with Renovate opening the PR.

### P0-5 · Connectivity rests on Cloudflare quick tunnels
`trycloudflare.com` quick tunnels come with no SLA, have rate limits, are documented as being for testing, and get blocked by many corporate proxies. The ngrok fallback was added in #50 and removed again in #52, so corporate networks now have no fallback except LAN.

**Fix:** a DeckTap Relay, a blind ciphertext-only WebSocket rendezvous on your own domain over `wss://…:443`. It is the reliability foundation *and* the first paid surface. See [`04-plan.md`](04-plan.md), Phase 2.

### P0-6 · LAN mode delivers the key-handling JavaScript over plain HTTP
In LAN mode the QR points to `http://<lan-ip>:<port>/#token.key`. Anyone on the network path (ARP spoofing on shared Wi-Fi) can inject JavaScript that reads `location.hash` and steals the key. The embedded pure-JS AES-GCM fallback in `crypto.js` exists *because* WebCrypto is unavailable in this insecure context.

- **Short term:** document this as an explicit threat-model limitation and show a warning in the UI for LAN mode on untrusted networks.
- **Mid term:** serve HTTPS on the LAN with real certificates, Plex-style (`<ip-dashed>.<device-id>.lan.decktap.app` resolving to the private IP, with a DNS-01-issued certificate). Then remove the pure-JS AES engine.

---

## P1: fix before a public launch

| # | Finding | Location | Fix |
|---|---|---|---|
| P1-1 | **3,521 lines of dead code**: a whole parallel implementation (`internal/server`, `internal/crypto`, `internal/ppt`, `web/`) that nothing imports. Its tests pass, which inflates confidence, and it's the only consumer of `gorilla/websocket`. | `internal/{server,crypto,ppt}`, `web/` | Delete it and run `go mod tidy`. |
| P1-2 | **Hand-written RFC 6455 parser with 0% test coverage** that faces the internet | `internal/ws/ws.go` | Add `go test -fuzz` targets for frame parsing, fragmentation and close codes, or move to `github.com/coder/websocket`. |
| P1-3 | **Insecure RNG fallback:** `getRandomBytes` falls back to `Math.random()` for IVs and nonces | `webui/static/crypto.js:23` | Fail closed: throw if `crypto.getRandomValues` is missing. |
| P1-4 | **PWA trust anchor is a personal GitHub Pages site.** Whoever can push to `silva324/DeckTap` Pages (a personal account, not Volaro.ai) can ship key-stealing JS to every phone. The origin is also hard-coded in `allowedOrigin`. | `app/host.go`, `deploy-pages.yml` | Move to `decktap.app`, put deploys behind a protected environment with required review, add a strict CSP, and publish asset hashes per release. |
| P1-5 | **Security docs don't match the code.** `architecture.md` says the envelope field is `data` (the code uses `ct`) and claims PBKDF2 (not used). Auditors will read the docs first. | `docs/architecture.md` | Make the spec normative, and add a test that asserts the documented envelope. |
| P1-6 | **Module path is `pptx-remote`, and so is the cmd dir.** This conflicts with the branding invariant, and `go install` doesn't work. | `go.mod`, `cmd/pptx-remote` | Rename to `cmd/decktap` (done in #57). **Update:** change it to `github.com/volaroai/DeckTap` in the same PR as the transfer to the `volaroai` org. With no users yet, change every identifier at once. See [`05-volaro-transition.md`](05-volaro-transition.md) §0 and §3. |
| P1-7 | **Coverage gaps on the riskiest code:** presenter 7%, tunnel 29%, ws 0%, nativewin/tray 0% | n/a | Add contract tests with recorded AppleScript/COM outputs, plus the fuzzing from P1-2. |
| P1-8 | **macOS spawns `osascript` every 500 ms**, which costs CPU and battery during long talks. Windows already uses a persistent bridge. | `presenter/mac.go`, `app/sync.go` | Use a persistent `osascript -l JavaScript` REPL process, or adaptive polling. |
| P1-9 | **Feature parity:** the laser pointer only works on macOS with cgo. The Linux release ships but can't control any presentation app. | `pointer_other.go`, `presenter/other.go` | Add a Windows overlay, and either stop shipping Linux or add a LibreOffice Impress (UNO) driver. |
| P1-10 | **No observability.** There's no way to know pairing success rates, tunnel failure rates or crashes. | n/a | Add opt-in, content-free telemetry and crash reports, plus self-reported relay metrics. |

## P2: polish and hygiene

- The CI Go version (`1.23`) doesn't match `go.mod` (`go 1.24.0`). Pin one version, using `go-version-file: go.mod`.
- PRs mix unrelated changes. For example, #52 adds LAN mode *and* removes ngrok *and* fixes quit. That makes rollback and changelogs harder.
- The PWA hard-codes the `sw.js` cache name (`decktap-v2.10`) separately from the app version. Generate it from the release version.
- The PWA has no i18n and no accessibility pass (screen reader labels, contrast in stage-cue pills).
- `.DS_Store` files and built binaries sit in the working tree. They're correctly gitignored, but clean them up locally.

## What's good (keep it)

- `session` state machine: single-use token, rotating SID, monotonic sequence numbers across reconnects, constant-time compares, a 3 s auth deadline, and a cap on pending sockets.
- `e2ee`: AAD direction binding (prevents reflection), 96-bit random nonces, `crypto/rand` panics rather than falling back.
- `admin`: 192-bit path secret, Host check (blocks DNS rebinding), Origin check, strict CSP, `X-Frame-Options`.
- The PowerPoint AppleScript guard invariant is enforced, and Windows uses a persistent STA PowerShell bridge.
- Release automation (release-please, multi-OS packaging, Homebrew tap) is already in place.
