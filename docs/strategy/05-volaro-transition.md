# DeckTap by Volaro.ai — Company Transition

> Status: proposal · 2026-10-31
> Assumption: **Volaro.ai** is the company behind DeckTap. It sells Pro, Team and Enterprise, runs the relay, owns the trademark, and employs or contracts the maintainers.

DeckTap stays the **product** name, as the branding invariant in `AGENTS.md` requires. Volaro.ai is the **publisher**: "DeckTap by Volaro.ai" in the About box, the installer publisher field and the store listings. The UI should never mix the two names into one ("Volaro DeckTap").

## 1. Roles

| Role | Today | Under Volaro.ai |
|---|---|---|
| Copyright holder (MIT core) | "Miguel Silva and contributors" | Unchanged for past contributions, unless assigned (see §2) |
| Licensor of `ee/` (FSL) and relay (AGPL) | n/a | **Volaro.ai** |
| CLA counterparty for `ee/` | n/a | **Volaro.ai** |
| Trademark owner ("DeckTap", logo) | none | **Volaro.ai** (EUIPO/UKIPO first, then USPTO) |
| Seller / merchant of record | n/a | **Paddle or Lemon Squeezy, reselling on behalf of Volaro.ai** |
| Relay operator (data processor) | n/a | **Volaro.ai**: privacy policy, DPA, sub-processor list (Cloudflare) |
| Code-signing identity | ad-hoc / none | **Volaro.ai** (Apple Developer *Organization*, Windows OV/Trusted Signing in the company's name) |
| Security contact | personal | `security@volaro.ai` (update `SECURITY.md`) |
| GitHub home | `github.com/silva324/DeckTap` | **A `volaro-ai` (or similar) organization**, repo transferred |

## 2. Legal and ownership (do first; it blocks relicensing)

1. **Copyright assignment from Miguel Silva to Volaro.ai** for the existing code base. This is a short written assignment. Without it, Volaro.ai can still *use* the MIT code (MIT allows anyone to), but it can't relicense it or put it in `ee/`, and it doesn't have clear standing in trademark or enforcement matters. Third-party contributions stay MIT under their authors. The git history shows only the maintainer's accounts so far, so the assignment is simple *now* and gets harder with every outside contributor.
2. **Update `LICENSE`** after the assignment: `Copyright (c) 2026 Volaro.ai and contributors`. Keep MIT. **Don't change it before the assignment is signed.**
3. **Trademark** "DeckTap" plus logo in Volaro.ai's name, and add a short `TRADEMARK.md` (forks welcome, rename required).
4. **Contribution policy:** DCO (`Signed-off-by`) for the MIT core and the AGPL relay, and a CLA with Volaro.ai for `ee/` only. Automate it with CLA Assistant on `ee/**` paths.
5. A **privacy policy and DPA** template for the relay. It only sees ciphertext and metadata: room ID, IP, timing, byte counts. Say exactly that and give retention periods.

*This is not legal advice. Have a lawyer in Volaro.ai's jurisdiction review the assignment and the trademark filings.*

## 3. Identifiers: what to change, what to freeze

Some identifiers are baked into binaries that are already installed. Changing them carelessly breaks upgrades or makes users re-grant permissions.

| Identifier | Current value | Decision | Why |
|---|---|---|---|
| **Go module path** | `github.com/silva324/DeckTap` (#57) | **Change once more to a vanity path, `decktap.app`** (`go install decktap.app/cmd/decktap@latest`), served by a `go-import` meta tag | Separates the import path from GitHub hosting, so transferring the repo to an org never breaks it again. Do it **before** anyone depends on the current path; there's been no tagged release with it yet. |
| **GitHub repo** | `silva324/DeckTap` | Transfer to the Volaro.ai org | GitHub redirects git and web URLs after a transfer. The release download and update URLs in old binaries keep working through the redirect, but don't rely on it forever. |
| **macOS bundle ID** | `com.silva324.decktap` | **Change to `ai.volaro.decktap` in the first signed release, deliberately** | A new bundle ID means macOS treats it as a new app, so users must re-approve *Automation → PowerPoint*. That's acceptable once, because the first Developer-ID-signed release changes the code identity anyway. Do both in the same release and say so in the release notes. |
| **Windows installer `AppId`** | `{D37F2C0B-…}` | **Freeze, never change** | Inno Setup uses it to upgrade in place. Changing it leaves two DeckTaps installed. |
| Installer publisher | "Miguel Silva and contributors" | `Volaro.ai` | Must match the code-signing certificate subject, or SmartScreen and users see a mismatch. |
| Homebrew tap | `silva324/tap` | New tap `volaro-ai/tap`. Keep the old cask as a pointer for 2 or more releases, then upstream to `homebrew/cask`. | |
| winget ID | none yet | `Volaro.DeckTap` | Publisher.Product convention. |
| Config dir | `~/.decktap` | **Freeze** | Product-scoped, not company-scoped. Good as is. |
| **PWA origin** | `https://silva324.github.io/DeckTap/` (hard-coded in `app.DefaultPWAURL` and `allowedOrigin`) | Move to `https://decktap.app/` | It's the **trust anchor** for key-handling JavaScript. Ship the new origin as the default and **keep the old origin in `allowedOrigin` for two minor releases**, so phones with the old PWA still pair. Then remove it. Keep the old Pages site as a redirect. |
| Funding links | Buy Me a Coffee and Sponsors under `silva324` | GitHub Sponsors on the Volaro.ai org (or Open Collective). Drop the personal Buy Me a Coffee. | Companies are wary of tipping individuals for a company's product. |

## 4. Domains

- **`decktap.app` is the product** (landing, docs, PWA, Go vanity path, pricing). `relay.decktap.app` hosts the relay and `lan.decktap.app` the future LAN certificates.
- **`volaro.ai` is the company:** legal pages, about, security contact, and enterprise sales (`sales@volaro.ai`).
- Why a separate registrable domain rather than `decktap.volaro.ai`? The PWA origin is a security boundary, and keeping it apart from the company's other web properties limits how far a compromise can spread (shared DNS, cookies, deploy tooling). It also leaves the product easy to spin out or sell.

## 5. Accounts with lead times (start now)

| Account | Lead time | Note |
|---|---|---|
| **D-U-N-S number** for Volaro.ai | Days to weeks | Apple needs it for **Organization** enrollment |
| **Apple Developer Program, Organization** | Days, after D-U-N-S | Developer ID certificate *in Volaro.ai's name* plus notarization |
| **Windows code signing** (Azure Trusted Signing or OV certificate) | Days to weeks | Needs the organization validated. Check eligibility for the company's age and jurisdiction. |
| Paddle / Lemon Squeezy seller verification | Days | Needs the company's legal and bank details |
| Trademark filing | Months to registration | File early. Use ™ until it's registered. |

## 6. Repo changes, as PRs (after §2 is signed)

1. `chore(repo): move module to decktap.app vanity path`, plus the `go-import` meta page on decktap.app.
2. *(GitHub UI)* Transfer the repo to the Volaro.ai org, then `chore(ci): update repo URLs, tap, CODEOWNERS to org teams`.
3. `docs(legal): assign copyright to Volaro.ai, add TRADEMARK.md, update SECURITY.md contact`.
4. `feat(release): sign as Volaro.ai, bundle ID ai.volaro.decktap`, released together with notarization (Phase 1).
5. `feat(webui): decktap.app PWA origin with legacy-origin grace period` (Phase 1, P1-4).
6. `chore(installer): publisher Volaro.ai, keep AppId`.
7. In the UI, show "DeckTap by Volaro.ai" in the About and footer, and link to `decktap.app` instead of GitHub.
