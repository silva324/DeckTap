# DeckTap — Zero-Trust E2EE Presentation Remote & Notes Reader

<p align="center">
  <img src="internal/webui/static/icon.svg" width="128" height="128" alt="DeckTap Logo" />
</p>

<p align="center">
  <a href="https://github.com/silva324/DeckTap/actions/workflows/ci.yml"><img src="https://github.com/silva324/DeckTap/actions/workflows/ci.yml/badge.svg" alt="CI Status" /></a>
  <a href="https://github.com/silva324/DeckTap/releases"><img src="https://img.shields.io/github/v/release/silva324/DeckTap?color=emerald" alt="Latest Release" /></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go" alt="Go Version" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT" /></a>
  <a href="https://buymeacoffee.com/silva324"><img src="https://img.shields.io/badge/Buy%20Me%20A%20Coffee-Donate-orange?style=flat&logo=buy-me-a-coffee" alt="Buy Me A Coffee" /></a>
</p>

> **DeckTap** is a modern, cross-platform presentation remote, speaker notes reader, and virtual laser pointer with **Zero-Trust End-to-End Encryption (AES-256-GCM)**, **Anti-Shoulder-Surfing PIN verification**, and **instant zero-install pairing**. Control your slides with confidence directly from your phone.

---

## ⚡ Quick Start — How to Use

Get presenting in less than 30 seconds:

```
+--------------------+         +--------------------+         +--------------------+
| 1. Launch Host     |         | 2. Scan & Pair     |         | 3. Enter PIN       |
| Desktop app opens  | ------> | Phone camera or    | ------> | Type 4-digit PIN   | ------> Present!
| pairing window     |         | Web App scans QR   |         | shown on computer  |
+--------------------+         +--------------------+         +--------------------+
```

1. **Launch DeckTap on your computer:** Open `DeckTap.app` on macOS or `DeckTap.exe` on Windows. The in-app desktop window displays a pairing QR code and a 4-digit visual PIN.
2. **Pair with your phone:** Scan the QR code using your phone camera, or open the web app ([silva324.github.io/DeckTap](https://silva324.github.io/DeckTap)) and tap **Scan QR** with the built-in scanner.
3. **Enter the 4-digit PIN:** Type the PIN shown on your computer screen to authorize your encrypted session.
4. **Present with confidence:** Advance slides with taps or swipes, read speaker notes with color-coded stage direction badges, zoom text with pinch gestures, and point with the laser or spotlight!

> 💡 **Tip:** Add the web app to your phone's home screen (**Share › Add to Home Screen** on iOS Safari, or **Install App** on Android Chrome) for an instant, full-screen offline-capable PWA experience.

---

## ✨ Features & Capabilities

DeckTap is designed for speakers who need absolute privacy, reliability, and modern presentation controls:

- 🛡️ **Zero-Trust End-to-End Encryption:** Slide notes, titles, and commands are encrypted with AES-256-GCM. Master keys are passed out-of-band via URI `#fragment` ([RFC 3986](https://tools.ietf.org/html/rfc3986#section-3.5)) and are never visible to servers or tunnels.
- 🔒 **Anti-Shoulder-Surfing PIN:** A 4-digit visual PIN shown strictly on the presenter's laptop screen prevents audience members from capturing or hijacking pairing QR codes.
- 🌐 **Instant Pairing & Edge Tunnel:** Automatically downloads and runs a secure Cloudflare tunnel for immediate pairing anywhere without port forwarding, with seamless local LAN fallback.
- 📱 **Progressive Web App & Camera Scanner:** Pure vanilla web app with an integrated camera scanner (torch/lens switch), OLED dark mode, haptic feedback, and deep background resume.
- 🎯 **Virtual Laser Pointer & Spotlight:** On-screen presentation overlay with a 16:9 trackpad reticle, touch tracking, instant centering, and optional gyroscope motion tilt aiming.
- 📝 **Smart Notes, Stage Cues & Pinch Zoom:** Color-coded stage badges (`[DEMO]`, `[PAUSE]`, `[CLICK]`, `[Q&A]`, `[WARN]`) and two-finger pinch-to-zoom (14px–48px) with persistent font preferences.
- 🖥️ **Multi-Display & Multi-Deck Presenter:** Automatically targets slide show monitors while keeping presenter view on your laptop, with live multi-deck switching.
- 📦 **Homebrew Cask & 1-Click Updates:** Install effortlessly with `brew install --cask silva324/tap/decktap` and receive automatic release notifications.

> 📖 **Detailed Guides & Specifications:**  
> - For gestures, stage cues, pinch zoom, and pointer controls $\to$ read the [**Presenter & User Guide**](docs/user-guide.md).  
> - For cryptographic specifications, wire envelopes, and IPC architecture $\to$ read the [**Architecture & Security Model**](docs/architecture.md).  
> - For future explorations and community ideas $\to$ explore the [**Product Roadmap**](ROADMAP.md).

---

## 🚀 Installation & Downloads

### macOS

#### Option A: Homebrew Cask (Recommended)
Install or update with a single terminal command:
```bash
brew install --cask silva324/tap/decktap
```
To update in the future:
```bash
brew upgrade --cask decktap
```

#### Option B: Direct DMG Download
Download the latest **Universal `.dmg`** from [Releases](https://github.com/silva324/DeckTap/releases):
1. Download `DeckTap-*-macOS-Universal.dmg`.
2. Open the disk image and drag **DeckTap** to your `/Applications` folder.
3. When prompted on first launch, allow DeckTap automation permissions in **System Settings › Privacy & Security › Automation**.

> [!TIP]
> **First-Launch on macOS (Gatekeeper Note):**  
> Because DeckTap is free, community open-source software and not signed with an Apple Developer Program ID certificate (\$99/year), macOS Gatekeeper may show *"Apple could not verify DeckTap.app"* on first launch.
> - **Method 1 (System Settings):** Open **System Settings › Privacy & Security**, scroll down to **Security**, and click **Open Anyway**.
> - **Method 2 (Right-Click):** In Finder, right-click (or <kbd>Control</kbd>-click) **DeckTap.app** in `/Applications` $\to$ click **Open** $\to$ click **Open**.
> - **Method 3 (Terminal):** Run `xattr -cr /Applications/DeckTap.app`
> 
> *(macOS only asks once on first launch; subsequent launches open with a normal double-click)*.

### Windows

Download the latest release package from [Releases](https://github.com/silva324/DeckTap/releases):
- **Setup Installer (Recommended):** Download `DeckTap-*-Setup-Windows-x64.exe` and follow the setup wizard.
- **Portable ZIP:** Download `DeckTap-*-windows-amd64.zip`, extract, and run `DeckTap.exe`. It runs silently in your System Tray with no distracting command window.

### Linux

Download `DeckTap-*-linux-amd64.tar.gz` or `DeckTap-*-linux-arm64.tar.gz` from [Releases](https://github.com/silva324/DeckTap/releases):
```bash
tar -xzf DeckTap-*-linux-amd64.tar.gz
./DeckTap
```

### Build from Source
```bash
# Clone the repository
git clone https://github.com/silva324/DeckTap.git
cd DeckTap

# Run tests
go test ./...

# Build binary
go build -o bin/decktap ./cmd/pptx-remote
```

---

## 🛠️ Usage & CLI Flags

```bash
# Standard mode (starts tunnel, opens host window & menu bar item)
./bin/decktap

# Diagnostic doctor (tests cloudflared, PowerPoint permissions, and notes extraction)
./bin/decktap doctor

# Local offline mode (skips tunnel; 127.0.0.1 or LAN IP)
./bin/decktap -mode local

# Demo mode (runs synthetic presentation controller without PowerPoint)
./bin/decktap -demo internal/pptx/testdata/demo.pptx

# Terminal only (no GUI window / tray icon, prints QR and PIN directly to console)
./bin/decktap -no-tray -no-browser
```

---

## ☕ Support & Sponsorship

If DeckTap has saved your presentations and kept your speaker notes private, consider supporting continuous development and maintenance:

<p align="center">
  <a href="https://buymeacoffee.com/silva324" target="_blank">
    <img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" style="height: 60px !important;width: 217px !important;" />
  </a>
</p>

You can also sponsor via [GitHub Sponsors](https://github.com/sponsors/silva324).

---

## 📚 Documentation

Detailed guides and architecture specifications are available in the [`docs/`](docs/) directory:

- **[Presenter & User Guide](docs/user-guide.md)**: Full walkthrough of smart stage cues (`[DEMO]`, `[PAUSE]`), pinch-to-zoom notes resizing, virtual laser & spotlight controls, slide drawer navigation, and PWA setup.
- **[Architecture & Security Specification](docs/architecture.md)**: Deep dive into the AES-256-GCM zero-trust security model, out-of-band key exchange, and macOS/Windows PowerPoint automation drivers.

---

## 🗺️ Roadmap & Community

- **[Product Roadmap & Ideas](ROADMAP.md)**: Explore future horizons, exploratory ideas, and community wishlists.
- **[Contributing Guide](CONTRIBUTING.md)**: Read how to contribute code, follow Conventional Commits for `release-please`, and submit PRs.
- **[Security Policy](SECURITY.md)**: Responsible disclosure guidelines and cryptographic boundaries.
- **[Code of Conduct](CODE_OF_CONDUCT.md)**: Contributor Covenant v2.1.

---

## 📄 License

This project is licensed under the [MIT License](LICENSE) &copy; 2026 Miguel Silva and contributors.
