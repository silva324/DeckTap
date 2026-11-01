# DeckTap — What Makes It a Success

> Status: proposal · 2026-11-01 · Two tiers only: **Free** (open source) and **Pro**. See [`03-monetization.md`](03-monetization.md).

## The constraint

At $39/yr, every $100k a year needs about **2,600 paying presenters**. At 2–3% conversion that means about **100k active users**. With no business tier and no sales team, there are only two levers:

1. **More active presenters** (reach and activation).
2. **A higher share of them paying** (conversion).

Everything below serves one of those two. Anything that serves neither waits.

## Priorities, in order

### 1. It never fails on stage
A remote that dies once in front of an audience gets uninstalled. Today, connecting from corporate and hotel networks depends on Cloudflare quick tunnels, which have no uptime guarantee and are often blocked. **The DeckTap Relay is the product.** It comes before any Pro feature ([`04-plan.md`](04-plan.md), Phase 2).

- **Metric:** first-try connection rate ≥ 95%; relay success on hostile networks ≥ 99%.

### 2. Zero install friction
Unsigned builds and "run `xattr -cr`" lose most people before their first slide. Apple notarization, Windows signing, winget and Homebrew are the cheapest growth work available (Phase 1).

- **Metric:** median time from download to first slide changed from the phone **under 60 seconds**.

### 3. A 14-day Pro trial on every install
No card, no account, verified offline like the licence key. Afterwards the Pro features switch off quietly and the free tier stays complete. People pay for what they've used. Next-slide preview and the timer are the features presenters miss most, because PowerPoint's Presenter View has taught them to expect both.

- **Metric:** trial-to-paid conversion; the share of trial users who use preview or timer in at least two talks.

### 4. The relay limit is the upgrade moment
When a free user hits the monthly relay limit, show a calm prompt at the start of the next session: upgrade, or switch to LAN mode. Never interrupt a talk in progress.

### 5. Pricing that matches how presenters buy
Presenters compare DeckTap with a clicker they buy once. Lead with **$39/yr**, offer **$89 lifetime** to early buyers only (capped), and keep $4/mo as a minor option. Companies buy several licences through the merchant of record, which handles invoices and tax. That replaces a business tier.

### 6. Growth through search and communities, not launch spikes
A Show HN post brings one spike. Lasting growth comes from people actively looking for this:

- **Search:** one landing page per high-intent query ("control PowerPoint from phone", "PowerPoint remote without app", "presentation clicker app", "speaker notes on phone"), each with a 30-second demo video.
- **YouTube:** short setup and "present on hotel Wi-Fi" tutorials.
- **Communities:** Toastmasters, trainers, sales engineers and consultants. Teachers stay on the free tier, but they spread it.
- **Listings:** winget, Homebrew, and the Mac App Store if the Apple Events entitlement is approved.

The phone UI's "DeckTap by Volaro" mark builds recognition, not reach: only the presenter sees their phone.

### 7. One main number, and the existing gates
- **Main metric:** presentations delivered per week (relay sessions plus opt-in telemetry).
- **Gate A:** build Pro only after 200 waitlist signups or 30 lifetime pre-sales.
- **Kill or pivot:** fewer than 500 MAU three months after launch means DeckTap stays a polished free OSS tool on sponsorships.

## What to cut or defer

| Item | Decision | Why |
|---|---|---|
| Linux release | Stop shipping it | It can't control any presentation app. It comes back with a LibreOffice Impress driver. |
| Keynote and Google Slides | After PowerPoint Pro has paying customers | Google Slides is the biggest reach play, but it's a large build. Prove Pro on PowerPoint first. |
| Business-tier work (admin console, SLA, custom contracts) | Not offered | Two tiers only. IT features are free and help companies approve DeckTap. |

## Risks

| Risk | Mitigation |
|---|---|
| Microsoft or Apple build a phone remote into PowerPoint or Keynote (Apple already has one for Keynote) | Stay cross-platform, need no phone install, keep end-to-end encryption you can audit, and work on blocked networks. |
| Support load with a single low-priced tier | Reliability first (priority 1), clear self-serve docs, community support only. |
| Trial abuse by reinstalling | Accept it. Enforcement stays soft by design; the people who pay want it signed, updated and supported. |

## 12-month scorecard

| Metric | Target |
|---|---|
| Monthly active hosts | 5,000 |
| First-try connection rate | ≥ 95% |
| Download to first slide | < 60 s median |
| Trial-to-paid conversion | ≥ 5% of trial users |
| Pro customers | 150–300 |
| Presentations delivered per week | Growing month over month |
