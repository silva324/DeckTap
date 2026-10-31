# DeckTap — Autonomous Agent Operating Guide (`AGENTS.md`)

Welcome, Agent. This document defines the autonomous operating contract, architectural invariants, and workflow procedures for contributing to **DeckTap**.

---

## 1. Operating Rules & Git Workflow

### 🚫 Rule 1: Never Commit to `main`
The `main` branch is protected. All contributions must follow the branch-and-PR lifecycle:
1. Ensure your local `main` is up to date: `git checkout main && git pull origin main`
2. Create a topic branch:
   - Features: `git checkout -b feat/<short-name>`
   - Bug fixes: `git checkout -b fix/<short-name>`
   - Documentation: `git checkout -b docs/<short-name>`
   - Chores / Maintenance: `git checkout -b chore/<short-name>`
3. Implement your changes.
4. **Mandatory Linter & Verification Gate:** Run `./scripts/agent-verify.sh` (or `make verify`). You MUST NOT commit or open a PR until all verification gates pass cleanly (0 exit code).
5. Push the branch: `git push -u origin <branch-name>`
6. Create a Pull Request via `gh` CLI:
   ```bash
   gh pr create --title "<type>(<scope>): <summary>" --body "<description>"
   ```
7. If the user commands you to merge it (as repo owner):
   ```bash
   gh pr merge <pr-number> --squash --delete-branch
   git checkout main && git pull origin main
   ```

### 🏷️ Rule 2: Strict Conventional Commits
All commit messages and PR titles must follow [Conventional Commits](https://www.conventionalcommits.org/):
- `feat(scope): ...` $\to$ Introduces a new capability (triggers minor release)
- `fix(scope): ...` $\to$ Patches a bug (triggers patch release)
- `docs(scope): ...` $\to$ Documentation changes
- `chore(scope): ...` $\to$ Build, CI, or dependency adjustments
- `perf(scope): ...` $\to$ Performance optimizations

### 🔀 Rule 3: Multi-Agent Concurrency & Git Isolation
Multiple autonomous agents may work concurrently on DeckTap. To ensure agents never stomp on each other's work or corrupt git state, every agent MUST abide by these isolation invariants:

1. **Workspace & Working Copy Isolation (Git Worktrees):**
   - Agents executing parallel tasks must **never** share a single mutable working directory if switching branches, modifying files, or committing concurrently.
   - Use dedicated **Git Worktrees** or isolated subagent workspaces (`Workspace: 'branch'` or `'share'`):
     ```bash
     # Fetch latest main and spawn an isolated worktree for the task
     git fetch origin main
     git worktree add ../DeckTap-<unique-topic> -b <type>/<unique-topic> origin/main
     cd ../DeckTap-<unique-topic>
     ```
   - Each worktree maintains its own independent `HEAD`, index, and file tree, preventing race conditions and branch-switching clashes.

2. **Branch Directly from Latest `origin/main`:**
   - Always base new topic branches on `origin/main`:
     ```bash
     git fetch origin main
     git checkout -b <type>/<unique-topic> origin/main
     ```
   - Never branch from another agent's unmerged local branch unless explicitly collaborating on that PR.
   - Always verify that proposed branch names are globally unique across all active local and remote branches (`git branch -a`).

3. **Strict Non-Interference (No Blanket Destructive Commands):**
   - **NEVER** run global destructive git commands that wipe uncommitted work or untracked files across the repository:
     - ❌ `git reset --hard` (destroys concurrent work or detached HEAD states)
     - ❌ `git clean -fd` (deletes untracked files created by parallel tasks)
     - ❌ `git checkout .` or `git restore .` (reverts unstaged files indiscriminately)
   - If an agent needs to revert its own changes, it must explicitly target only its owned files:
     ```bash
     git checkout -- path/to/my/file.go
     ```

4. **Explicit Atomic Staging (Never `git add .` or `git add -A`):**
   - Never run blanket `git add .` or `git add -A`. An agent could accidentally stage another concurrent agent's files or artifacts.
   - Always stage explicitly specified file paths:
     ```bash
     git add path/to/file1.go path/to/file2.js
     ```

5. **Rebase Before PR Submission:**
   - Before opening or finalizing a PR, rebase cleanly onto the latest `origin/main` to integrate any concurrent merges:
     ```bash
     git fetch origin main
     git rebase origin/main
     ```
   - If conflicts occur due to parallel work, resolve them in your own branch, re-run `./scripts/agent-verify.sh`, and push.

6. **Worktree Lifecycle & Cleanup:**
   - After a PR is merged, the agent must clean up its temporary worktree:
     ```bash
     cd <main-repo-dir>
     git worktree remove ../DeckTap-<unique-topic>
     git branch -d <type>/<unique-topic>
     ```

---

## 2. Core Architectural Invariants

### 🛡️ Security & Cryptography (`SPEC-SEC-001`)
- **Zero Plaintext Wire Exposure:** Slide notes, titles, and remote commands MUST be encrypted with **AES-256-GCM** before sending over WebSocket.
- **Out-of-Band Secrets:** Master keys ($K_{enc}$) and pairing tokens ($T_{pair}$) are transmitted **ONLY** via URI `#fragment` in the QR code. Never place secrets in query parameters, headers, or logs.
- **Anti-Shoulder-Surfing PIN:** The 4-digit visual PIN displayed on the host screen must be verified using constant-time comparison (`crypto/subtle.ConstantTimeCompare`).

### 🍎 Presenter Driver Safety (macOS AppleScript)
- **Always Guard PPT Process:** Every AppleScript controlling PowerPoint must check:
  `if application "Microsoft PowerPoint" is not running then return "0"`
  This prevents AppleScript from launching PowerPoint unprompted or hanging on IPC timeouts.
- **Avoid AppleScript Reserved Words:** Never use AppleScript reserved keywords (`out`, `st`, `nt`, `in`, `data`) as variable identifiers.

### 🌐 Pure Vanilla WebUI (No Node / npm)
- The mobile web client in `internal/webui/static/` is embedded into the Go binary (`//go:embed`).
- **DO NOT** introduce npm, Webpack, Vite, Tailwind CLI, or external script CDNs (`unpkg`, `cdnjs`). Keep it pure vanilla ES6+ and native browser Web APIs.

### 🏷️ Application Name & Branding Invariant
- The application name is exclusively **DeckTap** (binary executable name `decktap` on macOS/Linux, `DeckTap.exe` on Windows; local config directory `~/.decktap`).
- **NEVER** refer to, display, or output `"pptx-remote"` in user-facing logs, CLI banners, UI elements, or documentation.

---

## 3. Verification & Linting Commands for Agents

> [!IMPORTANT]
> Running the verification script is **MANDATORY** prior to every `git commit` and Pull Request. A pre-commit Git hook is available in `.githooks/pre-commit` and should be configured via `make setup-hooks`.

Run the automated verification suite:

```bash
./scripts/agent-verify.sh
# or via Makefile
make verify
```

The verification suite automatically runs all gates:
1. **JavaScript Linter:** `node -c internal/webui/static/app.js internal/webui/static/crypto.js internal/webui/static/sw.js` (detects syntax errors, re-declarations, etc.)
2. **Go Formatting:** `gofmt -l .` (ensures canonical formatting)
3. **Go Vet:** `go vet ./...` (catches structural and static analysis bugs)
4. **Go Tests:** `go test ./...` (runs unit and integration tests)
5. **Binary Compilation:** `go build -buildvcs=false -o /dev/null ./cmd/decktap`
6. **Git Hygiene:** Ensures no `.DS_Store`, `.exe`, or release archives slip into git.

To configure local git hooks automatically:
```bash
make setup-hooks
```

---

## 4. Repository Structure & Map

| Directory | Purpose |
| :--- | :--- |
| `cmd/decktap/` | Main application entry point and CLI flags |
| `internal/app/` | Host orchestrator, background synchronization, and state management |
| `internal/e2ee/` | AES-256-GCM encryption/decryption routines |
| `internal/session/` | Encrypted session state machine, PIN verification, and reconnection |
| `internal/pointer/` | On-screen Cocoa laser pointer and spotlight overlay |
| `internal/presenter/` | AppleScript (macOS), COM (Windows), and Demo controllers |
| `internal/nativewin/` | Desktop in-app window rendering pairing QR, PIN, and peer status |
| `internal/updater/` | GitHub Releases semver comparison and update checker |
| `internal/tunnel/` | Autonomous `cloudflared` tunnel downloader and runner |
| `internal/webui/static/` | Mobile PWA, camera QR scanner, speaker notes, and pointer HUD |
