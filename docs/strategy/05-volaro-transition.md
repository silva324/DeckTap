# DeckTap by Volaro.ai — Company Transition

> Status: proposal · 2026-10-31
> Assumptions: **Volaro.ai** is the company behind DeckTap, and its GitHub home is [`github.com/volaroai`](https://github.com/volaroai). Volaro.ai sells DeckTap Pro, runs the relay, owns the trademark, and employs or contracts the maintainers. **DeckTap has no users yet.**

DeckTap stays the **product** name, as the branding invariant in `AGENTS.md` requires. Volaro.ai is the **publisher**: "DeckTap by Volaro.ai" in the About box, the installer publisher field and the store listings. The UI should never mix the two names into one ("Volaro DeckTap").

## 0. The key point: make every breaking change now

With no installed base, renaming identifiers costs nothing today. Once there are users, each rename means a migration: lost permissions, broken pairing links, or two copies installed side by side. So **change every identity-bearing value once, now, to its final Volaro.ai form, before the first public release**, and then freeze them.

## 1. Roles

| Role | Today | Under Volaro.ai |
|---|---|---|
| Copyright holder (MIT core) | "Miguel Silva and contributors" | **Volaro.ai and contributors**, after assignment (see §2) |
| Licensor of `ee/` (FSL) and relay (AGPL) | n/a | **Volaro.ai** |
| CLA counterparty for `ee/` | n/a | **Volaro.ai** |
| Trademark owner ("DeckTap", logo) | none | **Volaro.ai** (EUIPO/UKIPO first, then USPTO) |
| Seller / merchant of record | n/a | **Paddle or Lemon Squeezy, reselling on behalf of Volaro.ai** |
| Relay operator (data processor) | n/a | **Volaro.ai**: privacy policy, DPA, sub-processor list (Cloudflare) |
| Code-signing identity | ad-hoc / none | **Volaro.ai** (Apple Developer *Organization*, Windows OV/Trusted Signing in the company's name) |
| Security contact | personal | `security@volaro.ai` (update `SECURITY.md`) |
| GitHub home | `github.com/silva324/DeckTap` | **`github.com/volaroai/DeckTap`** |

## 2. Legal and ownership

1. **Copyright assignment from Miguel Silva to Volaro.ai** for the existing code base. This is a short written assignment. Without it, Volaro.ai can still *use* the MIT code (MIT allows anyone to), but it can't relicense it or move it into `ee/`. The git history has only the maintainer's accounts, so this is a one-signature job now. Every outside contributor would make it harder.
2. **Update `LICENSE`** once it's signed: `Copyright (c) 2026 Volaro.ai and contributors`. Keep MIT.
3. **Trademark** "DeckTap" plus logo in Volaro.ai's name, and add a short `TRADEMARK.md` (forks welcome, rename required).
4. **Contribution policy:** DCO (`Signed-off-by`) for the MIT core and the AGPL relay, and a CLA with Volaro.ai for `ee/` only. Automate it with CLA Assistant on `ee/**` paths.
5. A **privacy policy and DPA** template for the relay. It only sees ciphertext and metadata: room ID, IP, timing, byte counts. Say exactly that and give retention periods.

*This is not legal advice. Have a lawyer in Volaro.ai's jurisdiction review the assignment and the trademark filings.*

## 3. Identifiers: final values (set now, then freeze)

| Identifier | Current value | Final value | Note |
|---|---|---|---|
| GitHub repo | `silva324/DeckTap` | `volaroai/DeckTap` | Transfer it. GitHub redirects repo, git and API URLs, but **not** GitHub Pages. With no users, the PWA URL just changes with everything else. |
| Go module path | `github.com/silva324/DeckTap` | `github.com/volaroai/DeckTap` | No vanity path. The org is the long-term home. |
| PWA origin (`app.DefaultPWAURL`, `allowedOrigin`) | `https://silva324.github.io/DeckTap/` | **`https://decktap.volaro.ai/`**, set directly with no interim origin | Drop the old origin entirely; nobody has the old PWA installed. Set up DNS (§4) *before* the move PR, so the first release ships the final origin. |
| macOS bundle ID | `com.silva324.decktap` | `ai.volaro.decktap` | Changing it later would make every user re-approve *Automation → PowerPoint*. Now it's free. |
| Windows installer `AppId` | `{D37F2C0B-…}` | Keep it | It's a random GUID with no owner name in it. **Freeze it from the first public release.** Changing it after that leaves two copies installed. |
| Installer publisher | "Miguel Silva and contributors" | `Volaro.ai` | Must match the code-signing certificate subject. |
| Homebrew tap | `silva324/tap` | `volaroai/homebrew-tap` (`brew install --cask volaroai/tap/decktap`) | Delete or archive the old tap; nobody is on it. |
| winget ID | none | `Volaro.DeckTap` | |
| Config dir | `~/.decktap` | Keep | Product-scoped, not company-scoped. |
| `CODEOWNERS` | `@silva324` | an org team, e.g. `@volaroai/decktap-maintainers` | |
| Funding links | Buy Me a Coffee and Sponsors under `silva324` | GitHub Sponsors on `volaroai` (or Open Collective) | Companies are wary of tipping individuals for a company's product. |

## 4. Domains

DeckTap lives under the company domain, as **`decktap.volaro.ai`**, in its **own delegated DNS zone**.

| Hostname | Purpose |
|---|---|
| `decktap.volaro.ai` | PWA (the security anchor for key-handling JavaScript), landing page, docs, pricing |
| `relay.decktap.volaro.ai` | DeckTap Relay (Phase 2) |
| `<ip-dashed>.<device-id>.lan.decktap.volaro.ai` | Per-device LAN certificates (Phase 2, P0-6) |
| `volaro.ai` | Company site, legal pages, `security@` and `sales@volaro.ai` |

**Why a subdomain works.** Browsers isolate by exact hostname, so other `volaro.ai` sites can't read the PWA's scripts or its `localStorage` session tokens. The usual risk of sharing a parent domain is cookie injection from sibling subdomains, and that doesn't apply because the PWA uses no cookies (authentication is the E2EE handshake). Business buyers also see the vendor's domain, and there's no extra domain to renew.

**Required setup:**

1. **Delegate the zone.** Add `NS` records for `decktap.volaro.ai` in the `volaro.ai` zone, pointing to a separate DNS zone with its own account and access. DeckTap's automation (DNS-01 certificates for LAN mode, relay hostnames) can then write DNS without any access to the company's main zone, and the people who manage `volaro.ai` for the website or email can't accidentally change the PWA's records.
2. **Harden the parent.**
   - registrar lock and 2FA on the registrar account
   - DNSSEC on `volaro.ai`, with the DS record for the delegated zone
   - a CAA record limiting which certificate authorities can issue for the domain
   - regular cleanup of dangling CNAMEs
3. **Verify `volaro.ai` in the `volaroai` GitHub org** (Settings, Pages, Verified domains) *before* pointing `decktap.volaro.ai` at Pages, so nobody else can claim the hostname on GitHub.
4. **Check the relay's DNS needs.** Custom hostnames for Cloudflare Workers normally need the zone that holds them to be on Cloudflare, so choose where the delegated `decktap.volaro.ai` zone lives with that in mind, and check what your Cloudflare plan allows.
5. **Optional:** register `decktap.app` cheaply, purely as a redirect to the landing page, so nobody else takes it. It is never a PWA origin.

**Trade-off accepted:** if DeckTap were ever sold or spun out, its PWA origin would sit under the company domain. You'd keep delegating the zone to the new owner, or migrate the origin with a grace period.

## 5. Accounts with lead times (start now)

| Account | Lead time | Note |
|---|---|---|
| **D-U-N-S number** for Volaro.ai | Days to weeks | Apple needs it for **Organization** enrollment |
| **Apple Developer Program, Organization** | Days, after D-U-N-S | Developer ID certificate *in Volaro.ai's name* plus notarization |
| **Windows code signing** (Azure Trusted Signing or OV certificate) | Days to weeks | Needs the organization validated. Check eligibility for the company's age and jurisdiction. |
| Paddle / Lemon Squeezy seller verification | Days | Needs the company's legal and bank details |
| Trademark filing | Months to registration | File early. Use ™ until it's registered. |

## 6. Moving to `volaroai`: one step, no migration

1. **Set up DNS first (§4):** delegate `decktap.volaro.ai` and verify `volaro.ai` in the GitHub org.
2. **Transfer** `silva324/DeckTap` to `volaroai/DeckTap` (Settings, Danger Zone, Transfer; your account is an org admin). Enable Pages on the org repo with the custom domain `decktap.volaro.ai` and enforce HTTPS.
3. **Open one PR,** `chore(repo): move DeckTap to volaroai`, that applies every value in §3:
   - module path
   - `DefaultPWAURL` and `allowedOrigin` set to `https://decktap.volaro.ai` only, with tests updated
   - bundle ID
   - installer publisher
   - README, CONTRIBUTING and issue-template URLs
   - `CODEOWNERS`
   - `FUNDING.yml`
   - Homebrew tap scripts
   - any hard-coded `silva324` paths in the release workflows
4. **Cut the next release** from the org repo, and archive `silva324/homebrew-tap`.
5. After the copyright assignment is signed: `docs(legal): assign copyright to Volaro.ai, add TRADEMARK.md, update SECURITY.md contact`.
6. With the first signed release (Phase 1): sign as Volaro.ai and show "DeckTap by Volaro.ai" in the About and footer, linking to `decktap.volaro.ai` instead of GitHub.

From the first public release on, treat the §3 values as **frozen**. Any later change needs a migration plan.
