# DeckTap Product Roadmap & Ideas

> [!NOTE]
> **No Commitments / Exploratory Only:** This document is an informal, living collection of creative explorations and community ideas. Nothing here represents a commitment, guarantee, roadmapped deliverable, or promised timeline. Priorities are fluid, community-driven, and subject to change or abandonment at any time.

---

## 🌟 Current Baseline (Initial Release)

- [x] **Zero-Trust End-to-End Encryption (`SPEC-SEC-001`):** AES-256-GCM encryption with ephemeral key transmission via URI `#fragment` (RFC 3986).
- [x] **Anti-Shoulder-Surfing PIN Verification:** 4-digit visual PIN shown exclusively on the presenter's screen with rate-limiting.
- [x] **Zero-Config Cloudflare Tunnel & LAN Fallback:** Autonomous edge tunnel deployment with automatic fallback to local LAN IP.
- [x] **Embedded Desktop In-App Window:** Native GUI window showing the pairing QR code, PIN, connection status, and active peers (no external browser required).
- [x] **PWA "Add to Home Screen" & Mobile Optimizations:** Installable progressive web app, fixed non-scrolling bars, isolated speaker notes scrolling, and OLED dark mode.
- [x] **In-App Camera QR Code Scanner:** Instant pairing directly inside the PWA with camera viewfinder, torch toggle, lens switcher, and manual link entry.
- [x] **Resilient Session Resumption:** Deep backgrounding & memory eviction survival via `localStorage` + `sessionStorage` tokens with dead-connection heartbeat detection.
- [x] **Virtual Laser Pointer & Spotlight Overlay:** On-screen macOS Cocoa overlay with 16:9 widescreen HUD and real-time touch tracking.
- [x] **Automated Updates via GitHub Releases:** Background release checking, semver comparison, desktop update notification banner, and system tray menu item.
- [x] **Native Presentation Drivers:** Native AppleScript for macOS PowerPoint, COM automation for Windows PowerPoint, and a synthetic zero-dependency Demo Engine (`-demo`).
- [x] **Automated CI/CD:** Matrix continuous integration, automated semantic versioning via `release-please`, and cross-platform packaging (`.dmg`, `.zip`, `.exe`, `.tar.gz`).
- [x] **Homebrew Cask Distribution:** One-command installation and seamless updates via `brew install --cask silva324/tap/decktap`.

---

## 🚀 Horizon 1 — Multi-Screen & Visual Presentation (Ideas)

- **External Projector & Multi-Display Targeting:** Exploring options to render laser pointer and spotlight overlays simultaneously or selectively across external projectors, secondary monitors, and laptop screens.
- **Slide Visuals / Screenshot Mirroring:** Ideas around displaying lightweight slide screenshots or previews directly on the phone's touch trackpad for more intuitive pointing.
- **Multi-Deck Management:** Exploring remote switching when presenters have more than one `.pptx` presentation open simultaneously.

---

## 🔮 Horizon 2 — Speaker Experience & Pacing (Ideas)

- **Silent Haptic Pacing Timer:** Potential subtle palm vibration alerts (`navigator.vibrate`) at milestone intervals (e.g. half-time, 2-minute warning) to help speakers manage time without watching a clock.
- **Visual Slide Grid & Search:** Quick-jump sheet to filter slides by title during audience Q&A sessions.
- **Motion / Gyroscope Pointer:** Experiments using device motion sensors (`DeviceOrientationEvent`) as an alternative to touch dragging.

---

## 🌐 Horizon 3 — Ecosystem & Explorations

- **Alternative Presentation Drivers:** Exploring community-contributed support for Apple Keynote, Google Slides, or web-based slide formats (Reveal.js, Marp).
- **Multi-Presenter Hand-off:** Experimental token delegation for dual-speaker presentations.
- **Direct Offline P2P:** Researching direct BLE or WebSockets P2P connections for highly restricted, offline auditorium networks.

---

*Ideas and community feedback are always welcome! Feel free to start a discussion or propose an idea on GitHub.*
