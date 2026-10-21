# Security Policy

## Security Model (`SPEC-SEC-001`)

PowerPoint Remote (now **DeckTap**) is built on a **Zero-Trust, End-to-End Encryption (E2EE)** model:

1. **Zero Cleartext Exposure:** Presentation titles, notes, and control commands are encrypted on the host before wire transmission and decrypted only inside the mobile browser's SubtleCrypto sandbox. Relays (e.g., Cloudflare Quick Tunnels or local LAN switches) see only opaque AES-256-GCM ciphertexts.
2. **Ephemeral Secret Transmission via URL `#fragment`:** In accordance with [RFC 3986](https://tools.ietf.org/html/rfc3986#section-3.5), URI `#fragments` are handled strictly client-side by browsers and are never transmitted to HTTP servers or proxy intermediaries in `GET /` requests.
3. **Anti-Shoulder-Surfing PIN Verification:** The URI fragment contains the master key ($K_{enc}$) and an ephemeral pairing token ($T_{pair}$). To prevent an audience member or remote viewer from pairing if they photograph the QR code from the presenter's laptop screen, the mobile app requires entering a 4-digit numeric visual PIN. The host enforces constant-time comparison and locks out after 5 consecutive failed attempts.
4. **Replay & Hijack Defense:** Monotonically increasing sequence counters (`seq`) and per-session tokens ($S_{id}$) are verified on every inbound frame. Captured frames cannot be replayed.
5. **No External JS Build Pipeline:** Static assets are served directly from embedded Go binaries without dynamic remote script CDNs (`unpkg`, `cdnjs`), ensuring no supply-chain tampering.

---

## Reporting a Vulnerability

We take the security of DeckTap seriously. If you believe you have discovered a vulnerability, please report it responsibly:

- **Do NOT open a public GitHub issue.**
- Report vulnerabilities via **GitHub Private Vulnerability Reporting** on this repository (under the `Security` tab) or email:
  `security@example.com`
- Please provide:
  - Detailed description of the vulnerability and attack vector.
  - Proof of concept (PoC) or reproducible steps.
  - Affected operating system, browser, and version.

### Response Timelines
- **Initial acknowledgment:** Within 48 hours.
- **Triage & severity assessment:** Within 5 business days.
- **Fix & advisory publication:** Released via GitHub Advisory and tagged release.
