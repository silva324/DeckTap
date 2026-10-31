# DeckTap — Open Source + Monetization Model

> Status: proposal · 2026-10-31

## The core problem

DeckTap is a local, serverless tool, and "use your phone as a clicker" is a commodity. Nobody pays for the free part, so revenue has to come from what **costs money to run or makes a professional presenter better on stage**:

1. **Reliability on hostile networks.** That needs infrastructure, which means a hosted relay.
2. **Features professionals feel on stage.** Previews, timing, rehearsal, and Keynote / Google Slides support.

Organisations are served by the free tier, not by a separate business plan. Signed installers, deployment profiles, policy settings and public security documentation help IT approve DeckTap, and the people who use it at work then buy Pro.

## Model: two tiers, open source plus Pro

| Layer | License | Why |
|---|---|---|
| Host app, PWA, protocol spec, drivers | **MIT** (unchanged) | The crypto must be auditable, since that's the brand. Keeping it MIT keeps contributors and distribution channels such as Homebrew and winget happy. |
| **DeckTap Relay** server | **AGPL-3.0** (licensor: Volaro.ai) | Anyone can self-host and audit it. AGPL stops someone else selling it as a hosted service without contributing back. |
| Pro features (`ee/` directory) | **FSL-1.1-MIT** (licensor: Volaro.ai; source-available, becomes MIT after 2 years) | The code stays visible, which helps trust, but competitors can't resell it. Sentry and Sidekiq follow this pattern. |
| Name and logo | **Trademark** ("DeckTap", registered by Volaro.ai) | Forks are welcome but must rebrand, which is the main protection for an OSS brand. |

Contributions: DCO for the MIT and AGPL code. A lightweight CLA with **Volaro.ai** only for `ee/`, so that it can be relicensed. Existing MIT code needs a copyright assignment to Volaro.ai first. See [`05-volaro-transition.md`](05-volaro-transition.md) §2.

**Enforcement is deliberately soft.** Pro is unlocked by an **offline Ed25519-signed licence key**: no accounts and no phoning home. Someone *can* fork and strip the check. That's acceptable, because the people who pay are professionals who want it signed, updated and supported.

## Tiers

| | **Free** (open source) | **Pro** |
|---|---|---|
| Price | $0 | **$4/mo · $36/yr · $79 lifetime** (lifetime capped at launch) |
| Remote, notes, stage cues, pointer, multi-deck | ✅ | ✅ |
| LAN mode, bring-your-own tunnel | ✅ | ✅ |
| DeckTap Relay (works on corporate / hotel Wi-Fi) | Fair use (e.g. 10 sessions/mo, 90 min each) | Unlimited, priority regions |
| Self-hosting the relay (AGPL code, community support) | ✅ | ✅ |
| Signed installers, deployment profiles, policy file (force LAN, pin a relay, disable updates) | ✅ | ✅ |
| Public security whitepaper and audit report | ✅ | ✅ |
| Slide thumbnails and next-slide preview on the phone | n/a | ✅ |
| Pacing timer with silent haptic cues, rehearsal mode and timing report | n/a | ✅ |
| Keynote and Google Slides drivers | n/a | ✅ |
| Drawing and annotations on slides | n/a | ✅ |

**Price anchors.** A Logitech Spotlight costs about $110–150 one-time, and App Store remote apps are one-time purchases. Presenters dislike subscriptions for a "clicker", so **the yearly plan and a lifetime option should be the headline**, with monthly as the minor option. Pro for a year costs about a third of a hardware clicker.

**Which features go in Pro, and why:** they are things *professionals* notice, and the free tier remains complete without them. The free tier never loses an existing feature.

**Companies** buy Pro licences in any quantity through the merchant of record, which issues invoices and handles tax. There is no separate business tier, no per-seat admin console, and no SLA or custom contract.

## Payments and operations

- **Merchant of record: Lemon Squeezy or Paddle.** They handle EU/UK VAT and US sales tax, which is essential for a small company selling worldwide. Volaro.ai is the vendor; the MoR resells on its behalf. Their licence-key APIs integrate with the offline Ed25519 keys, issued by a webhook that signs them. Quantity purchases issue one key per person.
- **Relay costs.** Cloudflare Workers plus Durable Objects with WebSocket hibernation cost fractions of a cent per session, so the free fair-use tier costs little. Keep a hard global budget alarm.
- **Move** GitHub Sponsors to the Volaro.ai org (or Open Collective), and retire the personal Buy Me a Coffee link.

## Revenue reality check

| Scenario | MAU | Paid conversion | ARPU/yr | ARR |
|---|---|---|---|---|
| Modest | 5,000 | 2% | $36 | **~$3.6k** |
| Good | 20,000 | 3% | $36 | **~$21.6k** |
| Strong | 50,000 | 3% | $36 | **~$54k** |

Lifetime licences add a one-off bump at launch but reduce recurring revenue, which is why they're capped.

With Pro as the only paid tier, **revenue scales with the number of active presenters**, not with deal size. That makes growth of the free tier the main lever, and it keeps the business simple to run: no sales calls, procurement, or support contracts. At modest adoption this pays for the infrastructure, the Apple and signing fees, and some development time. Becoming a meaningful business needs tens of thousands of active users.

## Validation gates (don't build billing on hope)

1. **Before building Pro features:** publish a pricing page with a waitlist and a "Join Pro early" button. **Gate:** 200 or more signups, or 30 or more people who pre-pay a lifetime licence.
2. **Kill or pivot signal:** if after Phase 1 plus a public launch (Show HN, Product Hunt, r/powerpoint, speaker communities) DeckTap has fewer than 500 MAU in 3 months, keep it as a polished free OSS tool on sponsorships and stop building paid features.

## Growth levers (free tier)

- **Google Slides via a browser extension** is the biggest reach play: education, Chromebooks and startups. Ship the basic version free and keep thumbnails and timer in Pro.
- **The pairing page is the ad.** The phone UI shows a subtle "DeckTap by Volaro" to everyone who scans, and stage presenters are very visible.
- Signed builds plus listings in winget, Homebrew and the Mac App Store (if the Apple Events entitlement is approved) remove install friction.
- **IT-friendly by default.** Free deployment profiles, a policy file and a public security page let companies approve DeckTap, which brings in presenters who then buy Pro.
- Content: "Present from your phone on corporate Wi-Fi", "Private speaker notes", and comparisons with Logitech Spotlight.
