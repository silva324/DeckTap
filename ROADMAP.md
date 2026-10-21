# DeckTap Product Roadmap

This document outlines the strategic product vision, architectural milestones, and upcoming capabilities for **DeckTap**.

---

## 🌟 Delivered in v0.0.1 (Foundation & Security)

- [x] **Zero-Trust End-to-End Encryption (`SPEC-SEC-001`):** AES-256-GCM encryption with ephemeral key transmission via URI `#fragment` (RFC 3986).
- [x] **Anti-Shoulder-Surfing PIN Verification:** 4-digit visual PIN shown exclusively on the presenter's screen with rate-limiting.
- [x] **Zero-Config Cloudflare Tunnel & LAN Fallback:** Autonomous edge tunnel deployment with automatic fallback to local LAN IP.
- [x] **Embedded Desktop In-App Window:** Clean, native GUI window showing the pairing QR code, PIN, connection status, and active peers (no external browser required).
- [x] **PWA "Add to Home Screen" & Mobile Optimizations:** Installable progressive web app, fixed non-scrolling navigation and controls bars, isolated speaker notes scrolling, and OLED dark mode.
- [x] **In-App Camera QR Code Scanner:** Instant pairing directly inside the PWA with camera viewfinder, torch toggle, lens switcher, and manual link entry.
- [x] **Resilient Session Resumption:** Deep backgrounding & memory eviction survival via `localStorage` + `sessionStorage` tokens with dead-connection heartbeat detection.
- [x] **Virtual Laser Pointer & Spotlight Overlay:** On-screen macOS Cocoa overlay with 16:9 widescreen HUD and real-time normalized touch tracking.
- [x] **Automated Updates via GitHub Releases:** Background release checking, semver comparison, desktop update notification banner, and system tray menu item.
- [x] **Native Presentation Drivers:** Native AppleScript for macOS PowerPoint, COM automation for Windows PowerPoint, and a synthetic zero-dependency Demo Engine (`-demo`).
- [x] **Automated CI/CD:** Continuous Integration matrix (macOS, Windows, Linux), automated semantic versioning via `release-please`, and cross-platform multi-architecture asset publishing (`.dmg`, `.zip`, `.exe`, `.tar.gz`).

---

## 🚀 Near-Term Milestone (v2.1) — Multi-Screen & Visual Presenter

### 1. Multi-Screen / External Projector Targeting
- **Goal:** Ensure virtual laser pointer and spotlight overlays render accurately on external projectors and audience displays.
- **Scope:**
  - Enumerate connected displays (`[NSScreen screens]`).
  - Pre-Presenter Display Selector: Choose between **All Displays (Simultaneous)**, **Projector / External Display**, or **Laptop Screen**.
  - Multi-window synchronized Cocoa overlay views.

### 2. Live Slide Visuals & Screenshot Mirroring
- **Goal:** Mirror actual slide visuals directly under the pointer trackpad on the mobile phone.
- **Scope:**
  - Pre-Presenter slide visuals loader: drop or select slide screenshots.
  - Client-side storage (IndexedDB) for zero-latency slide rendering.
  - 16:9 responsive aspect ratio container with dynamic reticle scaling.

### 3. Multi-Presentation Management & Switching
- **Goal:** Presenters with multiple slide decks open on their laptop can view, monitor, and switch between them remotely from their phone.
- **Scope:**
  - Enumerate all open presentations in PowerPoint via AppleScript (`presentations`) and Windows COM.
  - Mobile Deck Selector: Bottom sheet or dropdown displaying open decks with slide counts.
  - Remote Deck Switching: One-tap remote switch (`run slide show slide show settings of presentation X`) bringing the selected deck forward.

---

## 🔮 Mid-Term Milestone (v2.2) — Advanced Speaker Experience

### 4. Silent Haptic Pacing Timer
- **Goal:** Keep the presenter on schedule without needing to glance at a clock or phone screen.
- **Scope:**
  - Configurable target presentation duration (e.g., 15, 20, 45 minutes) configured on mobile.
  - Palm vibration alerts via `navigator.vibrate`:
    - **50% elapsed:** 1 gentle pulse.
    - **75% elapsed:** 2 gentle pulses.
    - **90% elapsed / 2 min left:** 3 pulses.
    - **Overtime:** Continuous subtle pulsing and timer display transitions to warning red.

### 5. Visual Slide Grid & Live Search Jump Sheet
- **Goal:** Instant non-linear navigation during audience Q&A or discussions without paging through all intermediate slides in front of the audience.
- **Scope:**
  - Slide drawer showing all slide numbers and titles.
  - Real-time search filter (e.g. typing "architecture" immediately narrows the list down to Slide 14).
  - One-tap jump (`GOTO`) directly to the target slide.

### 6. Gyroscope-based Laser Pointer (`DeviceOrientationEvent`)
- **Goal:** Point the phone at the audience screen to move the laser dot like a physical presenter remote.
- **Scope:**
  - Mobile sensor capture via `DeviceOrientationEvent` (`gamma` tilt left/right, `beta` tilt up/down).
  - Hold-to-point interaction with resting orientation calibrated upon touching the screen.
  - Exponential moving average smoothing to eliminate hand tremor jitter.

---

## 🌐 Long-Term & Community Horizons (v3.0+)

- **Additional Presentation Engines:**
  - Apple Keynote driver on macOS.
  - Google Slides integration via browser extension or Chrome DevTools Protocol.
  - Web slide decks (Reveal.js, Marp, Slidev).
- **Multi-Device Co-Presenting:**
  - Allow two speakers with phones to pair to the same host with token delegation for dual-presenter talks.
- **Offline Direct BLE / WebRTC:**
  - P2P pairing when neither Wi-Fi nor internet is available in secure government/enterprise auditoriums.

---

*Suggestions and contributions are welcome! Please open an issue or PR following [CONTRIBUTING.md](CONTRIBUTING.md).*
