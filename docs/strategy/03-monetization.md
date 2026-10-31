# DeckTap — Open Source + Monetization Model

> Status: proposal · 2026-10-31

## The core problem

DeckTap is a local, serverless tool, and "use your phone as a clicker" is a commodity. Nobody pays for the free part, so revenue has to come from what **costs money to run, saves real time, or reduces organisational risk**:

1. **Reliability on hostile networks.** That needs infrastructure, which means a hosted relay.
2. **Features professionals feel on stage.** Previews, timing, rehearsal, and Keynote / Google Slides support.
3. **Control for organisations.** Deployment, policy, self-hosting, and security paperwork.

## Model: open core plus a hosted service

| Layer | License | Why |
|---|---|---|
| Host app, PWA, protocol spec, drivers | **MIT** (unchanged) | The crypto must be auditable, since that's the brand. Keeping it MIT keeps contributors and distribution channels such as Homebrew and winget happy. |
| **DeckTap Relay** server | **AGPL-3.0** | Enterprises can self-host and audit it. AGPL stops someone else selling it as a hosted service without contributing back. |
| Pro / Team features (`ee/` directory) | **FSL-1.1-MIT** (source-available, becomes MIT after 2 years) | The code stays visible, which helps trust, but competitors can't resell it. GitLab, Sentry and Sidekiq follow this pattern. |
| Name and logo | **Trademark** (register "DeckTap") | Forks are welcome but must rebrand, which is the main protection for an OSS brand. |

Contributions: DCO for the MIT and AGPL code. A lightweight CLA only for `ee/`, so that it can be relicensed.

**Enforcement is deliberately soft.** Pro is unlocked by an **offline Ed25519-signed licence key**: no accounts and no phoning home. Someone *can* fork and strip the check. That's acceptable, because the people who pay are professionals and companies who want it signed, updated and supported.

## Tiers

| | **Free** | **Pro** | **Team** | **Enterprise** |
|---|---|---|---|---|
| Price | $0 | **$4/mo · $36/yr · $79 lifetime** (lifetime capped at launch) | **$7/user/mo**, billed yearly, min. 3 | **from $3k/yr** |
| Remote, notes, stage cues, pointer, multi-deck | ✅ | ✅ | ✅ | ✅ |
| LAN mode, bring-your-own tunnel | ✅ | ✅ | ✅ | ✅ |
| DeckTap Relay (works on corporate / hotel Wi-Fi) | Fair use (e.g. 10 sessions/mo, 90 min each) | Unlimited, priority regions | Unlimited | **Self-hosted relay** (AGPL) plus support |
| Slide thumbnails and next-slide preview on the phone | n/a | ✅ | ✅ | ✅ |
| Pacing timer with silent haptic cues, rehearsal mode and timing report | n/a | ✅ | ✅ | ✅ |
| Keynote and Google Slides drivers | n/a | ✅ | ✅ | ✅ |
| Drawing and annotations on slides | n/a | ✅ | ✅ | ✅ |
| Signed MSI / PKG plus MDM config profiles | n/a | n/a | ✅ | ✅ |
| Org policy (force LAN, pin relay, disable updates) | n/a | n/a | ✅ | ✅ |
| Central licence management, invoices | n/a | n/a | ✅ | ✅ |
| Security whitepaper, third-party audit report, DPA, SLA | n/a | n/a | n/a | ✅ |

**Price anchors.** A Logitech Spotlight costs about $110–150 one-time, and App Store remote apps are one-time purchases. Presenters dislike subscriptions for a "clicker", so **the yearly plan and a lifetime option should be the headline**, with monthly as the minor option. Pro for a year costs about a third of a hardware clicker.

**Which features go in Pro, and why:** they are things *professionals* notice, and the free tier remains complete without them. The free tier never loses an existing feature.

## Payments and operations

- **Merchant of record: Lemon Squeezy or Paddle.** They handle EU/UK VAT and US sales tax, which is essential for a solo developer selling worldwide. Their licence-key APIs integrate with the offline Ed25519 keys, issued by a webhook that signs them.
- **Relay costs.** Cloudflare Workers plus Durable Objects with WebSocket hibernation cost fractions of a cent per session, so the free fair-use tier costs little. Keep a hard global budget alarm.
- **Keep** GitHub Sponsors and Buy Me a Coffee for community goodwill.

## Revenue reality check

| Scenario | MAU | Paid conversion | ARPU/yr | ARR |
|---|---|---|---|---|
| Pro only, modest | 5,000 | 2% | $36 | **~$3.6k** |
| Pro only, good | 20,000 | 3% | $36 | **~$21.6k** |
| + Team (10 orgs × 10 seats) | | | $84/seat | **+ $8.4k** |
| + 3 Enterprise | | | $3–5k | **+ $9–15k** |

Pro alone pays for the infrastructure, the Apple and signing fees, and a few weekends. **Meaningful revenue depends on Team and Enterprise,** so validate them early and cheaply. Write the security whitepaper and add the policy file *before* building a full admin console, then talk to 10 security-conscious companies.

## Validation gates (don't build billing on hope)

1. **Before building Pro features:** publish a pricing page with a waitlist and a "Join Pro early" button. **Gate:** 200 or more signups, or 30 or more people who pre-pay a lifetime licence.
2. **Before building Team/Enterprise:** 10 discovery calls with privacy-sensitive organisations or AV companies. **Gate:** 3 or more say "we'd pay for X" and name X.
3. **Kill or pivot signal:** if after Phase 1 plus a public launch (Show HN, Product Hunt, r/powerpoint, speaker communities) DeckTap has fewer than 500 MAU in 3 months, keep it as a polished free OSS tool on sponsorships and stop building paid features.

## Growth levers (free tier)

- **Google Slides via a browser extension** is the biggest reach play: education, Chromebooks and startups. Ship the basic version free and keep thumbnails and timer in Pro.
- **The pairing page is the ad.** The phone UI shows a subtle "Powered by DeckTap" to everyone who scans, and stage presenters are very visible.
- Signed builds plus listings in winget, Homebrew and the Mac App Store (if the Apple Events entitlement is approved) remove install friction.
- Content: "Present from your phone on corporate Wi-Fi", "Private speaker notes", and comparisons with Logitech Spotlight.
