# Contributing to DeckTap

Thank you for your interest in contributing to **DeckTap**! We welcome contributions from developers across platforms (macOS, Windows, Linux, and Web).

---

## 1. Ground Rules & Guiding Principles

1. **Zero-Trust Security Invariants:**
   - All presentation content, speaker notes, and commands MUST remain encrypted with AES-256-GCM.
   - Master keys and pairing tokens MUST ONLY be transmitted via URI `#fragment`.
   - Never log secret keys, session tokens, or plaintext payloads.
   - All security-sensitive comparisons (PINs, tokens, session IDs) MUST use `crypto/subtle.ConstantTimeCompare`.
2. **Zero External JS Dependencies:**
   - The web app runs on standard vanilla ES6+ and native browser APIs (`window.crypto.subtle`, `WebSocket`).
   - Do NOT introduce npm, Webpack, Vite, or external script CDNs (`unpkg`, `cdnjs`).
3. **Cross-Platform Compatibility:**
   - The host agent must run cleanly on both macOS (AppleScript) and Windows (PowerShell/COM).
   - Use build tags (`//go:build darwin`, `//go:build windows`) for platform-specific system calls.

---

## 2. Conventional Commits & Versioning (`release-please`)

This repository uses [Conventional Commits](https://www.conventionalcommits.org/) automated by Google's **`release-please`** action. Your commit messages dictate automated semantic versioning and changelog entries:

| Type | Description | SemVer Impact |
| :--- | :--- | :--- |
| `feat:` | A new user-facing feature or capability | **MINOR** (e.g. `1.1.0` $\to$ `1.2.0`) |
| `fix:` | A bug fix | **PATCH** (e.g. `1.1.0` $\to$ `1.1.1`) |
| `perf:` | A code change that improves performance | **PATCH** |
| `refactor:` | A code change that neither fixes a bug nor adds a feature | None |
| `test:` | Adding missing tests or correcting existing tests | None |
| `docs:` | Documentation changes only | None |
| `chore:` | Build tasks, package updates, CI/CD tweaks | None |
| `feat!:`, `fix!:` | **Breaking changes** (or includes `BREAKING CHANGE:` in footer) | **MAJOR** (e.g. `1.0.0` $\to$ `2.0.0`) |

### Example Commit Messages:
```bash
feat(presenter): add multi-deck enumeration and switching
fix(webui): resolve touch event delay on Android Chrome PIN pad
docs(readme): add installation guide for Windows winget
```

---

## 3. Local Development Setup

### Prerequisites
- **Go 1.23+**
- **Git**
- Microsoft PowerPoint (optional for live testing; the app includes a `-demo` controller)

### Building & Running
```bash
# Clone the repository
git clone https://github.com/silva324/DeckTap.git
cd DeckTap

# Run unit and integration tests
go test ./...

# Build the binary
go build -o bin/pptx-remote ./cmd/pptx-remote

# Run diagnostic doctor
./bin/pptx-remote doctor

# Run with demo deck (offline, no PowerPoint needed)
./bin/pptx-remote -mode local -demo internal/pptx/testdata/demo.pptx

# Run in live mode with tunnel
./bin/pptx-remote
```

---

## 4. Pull Request Process

1. **Fork the repo** and create your branch from `main`:
   ```bash
   git checkout -b feat/my-new-feature
   ```
2. **Write clean, documented Go code**:
   - Format with `gofmt -s -w .`
   - Run linter: `golangci-lint run` (or `go vet ./...`)
3. **Add or update tests**:
   - Every bug fix should include a regression test.
   - Run `go test -race ./...` to verify thread safety.
4. **Submit your Pull Request**:
   - Write a descriptive PR title using Conventional Commits.
   - Fill out the PR template checklist.

Thank you for helping make DeckTap the most secure, private presentation tool available!
