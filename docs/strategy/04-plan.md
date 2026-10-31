# DeckTap — Production Plan

> Status: proposal · 2026-10-31 · Durations assume one part-time maintainer plus agents.
> Each line is meant to become one PR (`type(scope): …`), per `AGENTS.md`.

```
Phase 0  Harden the core         ~2 wk   ── security & hygiene, no new features
Phase 1  Trustworthy distribution ~3 wk  ── signed, notarized, own domain, telemetry, launch
Phase 2  DeckTap Relay            ~4 wk  ── reliable connectivity + first paid surface
   ▸ Gate A: waitlist ≥ 200 or ≥ 30 lifetime pre-sales
Phase 3  Pro + licensing          ~5 wk  ── features presenters pay for
   ▸ Gate B: ≥ 3 orgs name what they'd pay for
Phase 4  Team / Enterprise        ongoing
```

---

## Phase 0: Harden the core (no new features)

| PR | Fixes | Done when |
|---|---|---|
| `fix(session): isolate pre-auth failures from paired sessions` | P0-1 | Garbage handshakes are rate-limited per IP. A paired client survives any number of bad pre-auth frames. Regression test included. |
| `fix(updater): require Ed25519 signatures and fail closed` | P0-2 | Releases ship `.minisig` files and the public key is embedded. A missing or invalid signature aborts the update. Tests cover tamper, missing-sig and wrong-key cases. |
| `fix(tunnel): pin cloudflared version and verify SHA-256` | P0-4 | A hash table per OS/arch is embedded. A mismatch refuses to execute. |
| `fix(webui): fail closed without crypto.getRandomValues` | P1-3 | No `Math.random` anywhere in crypto paths. |
| `chore: remove dead server/crypto/ppt/web packages` | P1-1 | About 3.5k lines deleted. `go mod tidy` drops gorilla. |
| ~~`chore: rename module to github.com/silva324/DeckTap, cmd/decktap`~~ (done, #57). Next: `chore(repo): move module to decktap.app vanity path` | P1-6 | `go install decktap.app/cmd/decktap@latest` works and survives the repo moving to the Volaro.ai org. |
| `test(ws): add fuzz targets for frame parsing` | P1-2 | `FuzzReadFrame` runs in CI for 60 s per PR, with no panics. |
| `docs(architecture): make SEC-001 normative and match the code` | P1-5 | Envelope field names and KDF claims match. A test asserts the documented envelope. |
| `docs(security): threat model incl. LAN-mode HTTP limitation` | P0-6 (short term) | A `docs/threat-model.md` exists, and the UI warns when LAN mode is used on untrusted networks. |
| `chore(ci): use go-version-file, add govulncheck + staticcheck gates` | P2 | CI matches `go.mod`. |

## Phase 1: Trustworthy distribution and launch

| PR / task | Done when |
|---|---|
| Volaro.ai company setup: copyright assignment, D-U-N-S, Apple Developer **Organization**, GitHub org transfer ([`05`](05-volaro-transition.md) §2, §5) | Assignment signed. Repo under the Volaro.ai org. Apple and Windows signing identities issued in Volaro.ai's name. |
| Register `decktap.app` and file the trademark in Volaro.ai's name | Domain live. Trademark filing submitted (EUIPO/UKIPO, then USPTO). |
| `feat(webui): serve PWA from decktap.app with protected deploys` (P1-4) | Pages deploys need a protected-environment approval. Strict CSP. The old origin redirects. `allowedOrigin` is updated. |
| `chore(release): Developer ID signing + notarization` (P0-3) | `spctl -a -vv DeckTap.app` reports accepted. The Gatekeeper workaround is gone from the README. |
| `chore(release): Windows Authenticode signing` | Azure Trusted Signing or an OV certificate in Volaro.ai's name. (SignPath's free OSS program is unlikely to fit a commercially sold product.) SmartScreen stops warning after reputation builds. |
| `feat(updater): bundle-level macOS updates` | The update replaces the whole signed `.app` (or defers to `brew upgrade`). The signature stays valid after updating. |
| `chore(release): publish to winget` | `winget install DeckTap.DeckTap` works. |
| `feat(telemetry): opt-in anonymous metrics + crash reports` (P1-10) | First-run consent screen. Events cover pairing, mode and failure reasons, with no content. A public schema is in `docs/telemetry.md`. |
| `perf(presenter): persistent osascript bridge on macOS` (P1-8) | CPU use while presenting drops by at least 50%. |
| Landing page plus docs site | Covers the value proposition, 30-second demo video, download, security page and pricing waitlist (feeds Gate A). |
| **Launch:** Show HN, Product Hunt, r/powerpoint, speaker communities | Tracks MAU and waitlist count. |

## Phase 2: DeckTap Relay (P0-5)

A **blind rendezvous relay** that forwards opaque frames between the host and the phone. It never sees keys or plaintext, so the zero-trust model is unchanged.

```
 phone ──wss://relay.decktap.app/r/<roomId>──┐        ┌── host (outbound only, 443)
                                             ▼        ▼
                              Durable Object per room: pipe ciphertext, enforce quotas
```

- **Room ID:** 128-bit random, created by the host, sent in the QR fragment next to the key. The relay never learns the key.
- **Authorisation:** the host registers the room with an HMAC proof derived from the pairing token (or a separate relay secret), so strangers can't squat on rooms.
- **Infrastructure:** Cloudflare Workers plus Durable Objects with WebSocket hibernation. Code goes in `relay/` under AGPL. A Docker image is published for self-hosting.
- **Host order of preference:** Relay first, then LAN, then BYO cloudflared. The current quick-tunnel code stays as an option rather than the default.
- **Quotas:** fair-use limits for the free tier, enforced per host install ID. Unlimited with a valid licence key. A global cost alarm.

| PR | Done when |
|---|---|
| `feat(relay): Worker + Durable Object blind relay` | Load test with 1,000 concurrent rooms. p99 added latency under 80 ms in-region. |
| `feat(tunnel): relay transport with automatic fallback` | Works behind a proxy that blocks `trycloudflare.com`. Integration test included. |
| `feat(relay): self-hosting image + docs` | `docker run decktap/relay` plus a `-relay-url` flag. |
| `feat(lan): HTTPS LAN mode via per-device certs` (P0-6 mid term) | Plex-style `*.lan.decktap.app` with DNS-01 certificates. The pure-JS AES engine is deleted. |

## Phase 3: Pro and licensing (after Gate A)

| PR | Done when |
|---|---|
| `feat(license): offline Ed25519 licence keys` | Keys are verified locally, never phoned home. Expiry is respected, with a grace period. `ee/` is under FSL-1.1-MIT. |
| Lemon Squeezy or Paddle store plus a webhook signer | Purchase leads to a signed key by email in under 1 minute. |
| `feat(ee): encrypted slide thumbnails + next-slide preview` | Thumbnails are exported via the AppleScript/COM `export` command, downscaled, and E2EE-chunked to the phone. |
| `feat(ee): pacing timer, haptic cues, rehearsal report` | |
| `feat(ee): Keynote driver (macOS)` | Keeps parity with the PowerPoint driver's `Controller` interface. |
| `feat(ee): Google Slides browser extension driver` | The basic version ships free (growth). Thumbnails and timer are Pro. |
| `feat(pointer): Windows overlay` (P1-9) | Free: brings the laser pointer to parity with macOS. |

## Phase 4: Team and Enterprise (after Gate B)

- Signed `.pkg` / `.msi`, MDM configuration profiles, and a `policy.json` (force LAN or a specific relay, disable updates, disable telemetry).
- Self-hosted relay support package and SLA.
- A security whitepaper plus an **independent crypto and protocol audit**, for example via OSTIF or a boutique firm (budget $5–15k, possibly OSS-funded).
- Later, if the events segment validates: a multi-stage **operator console** for AV teams.

---

## Immediate next steps (this week)

1. ~~Fix **P0-1** (lockout DoS).~~ Done in #56.
2. ~~Delete the dead packages and rename the module.~~ Done in #57; the vanity-path follow-up is in Phase 0.
3. Start the Volaro.ai items with lead times: copyright assignment, D-U-N-S, then Apple Developer *Organization* and Windows signing in the company's name, plus the `decktap.app` domain.
4. Put up a one-page landing site with a Pro waitlist so Gate A starts collecting data while Phases 0 and 1 are under way.
