# DeckTap — Product Intent

> Status: proposal · 2026-10-31 · company: **Volaro.ai** · maintainer: Miguel Silva
>
> DeckTap is the product. Volaro.ai is the company that publishes it, sells the paid tiers and runs the relay. See [`05-volaro-transition.md`](05-volaro-transition.md).

## 1. What DeckTap is today

A single Go binary on the presenter's laptop drives PowerPoint (AppleScript on macOS, a COM bridge on Windows) and serves a vanilla-JS PWA to the presenter's phone. Pairing is a QR code plus a 4-digit PIN. Everything on the wire is AES-256-GCM with the key passed in the URL `#fragment`. Connectivity is either a Cloudflare quick tunnel or direct LAN.

The engineering is above average for a project at this stage. The session state machine, the AAD direction binding, the replay protection and the admin-page guards are all careful work. **The product isn't there yet:**

- Adoption is close to zero: 0 stars, 0 forks, about 35 DMG downloads on v0.11.1.
- Builds are unsigned. The README tells users to bypass Gatekeeper.
- Connectivity rests on `trycloudflare.com` quick tunnels. Cloudflare offers these for testing, with no uptime guarantee, and corporate proxies often block them.
- There is no revenue model beyond Buy Me a Coffee.

## 2. Intent statement

> **DeckTap turns any phone into a private, reliable presentation remote: notes, slide control and a laser pointer, with nothing to install on the phone. Your notes never leave your devices unencrypted.**

The intent has three parts:

1. **It works every time, on any network.** Hotel Wi-Fi, a corporate guest network or a conference hall shouldn't stop it. Reliability is the product. A remote that fails on stage once never gets used again.
2. **It is private by construction, and you can check that.** Neither DeckTap's servers nor anyone in between can read notes or commands. The open-source code is how users verify this, and it is also a selling point.
3. **It costs less than a clicker and does more.** It is free for individuals. Professionals and organisations pay for convenience, advanced features and control.

## 3. Who it is for (in priority order)

| Segment | Job to be done | Willingness to pay | Role |
|---|---|---|---|
| **Professional presenters**: consultants, sales engineers, trainers, keynote speakers | "Present without a $120 clicker, see my notes and timing, never fumble" | Medium: they already buy clickers | **Primary paying segment (Pro)** |
| **Privacy-sensitive organisations**: finance, legal, health, government, pre-announcement product launches | "Confidential speaker notes can't transit a third-party cloud in clear" | High, but slow procurement | **Enterprise** (self-hosted relay, MDM, policy) |
| **Event and conference AV teams** | "Get 40 speakers onto stage laptops reliably, with no per-speaker setup" | High per event | **Later**: operator console |
| **Educators and students** | "Free, works on school Wi-Fi, Google Slides" | Low | **Growth and community**, free forever |

## 4. Positioning vs alternatives

| Alternative | Weakness DeckTap exploits |
|---|---|
| Hardware clickers (Logitech Spotlight, about $110–150) | No notes, no timer, one more thing to carry and charge |
| Keynote Remote (Apple) | Apple-only and Keynote-only |
| App Store remote apps (one-time purchase) | Need an install on the phone, usually LAN-only, closed source, no privacy story |
| Teams / PowerPoint Live | Tied to a meeting; not a stage remote |

**Differentiators:** zero install on the phone (scan and go), end-to-end encryption you can audit, cross-platform host, and open source.

## 5. Principles (non-negotiable while monetizing)

1. **Core stays MIT and fully usable for free.** That covers pairing, control, notes, pointer, LAN and bring-your-own tunnel. Removing a free feature would break trust with the OSS community.
2. **The crypto stays auditable.** Paid features must never route plaintext through DeckTap servers. Anything hosted, such as the relay, only ever sees ciphertext.
3. **No accounts required for the free tier.** Paid licences use offline-verifiable keys, so nobody needs an account just to present.
4. **Telemetry is opt-in only,** documented, and contains no content.
5. **Brand discipline:** "DeckTap" is the product name everywhere. Volaro.ai appears only as the publisher ("DeckTap by Volaro.ai"). Volaro.ai owns the trademark, so forks must rebrand.

## 6. What success looks like (12 months)

| Metric | Target |
|---|---|
| Monthly active hosts (opt-in telemetry plus relay sessions) | 5,000 |
| Successful pairing rate (first try, under 30 s) | ≥ 95% |
| Relay session success on "hostile" networks | ≥ 99% |
| Paid conversions (Pro + Team seats) | 150 to 300 |
| GitHub stars | 1,000+ |
| First enterprise / self-hosted customer | 1 |

Honest framing: with Pro alone, this is a side-income product, roughly $5–15k a year at these numbers. Real money depends on the **enterprise or events** segments. The plan is built to find out cheaply whether those segments exist before investing in them. See [`03-monetization.md`](03-monetization.md) for details.
