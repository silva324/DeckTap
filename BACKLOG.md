# Product Backlog & Roadmap

> 🗺️ **Looking for our high-level strategic roadmap?** See [ROADMAP.md](ROADMAP.md).

This document outlines granular work items, architectural specs, and upcoming technical tasks for **DeckTap**.

---

## 🚀 High Priority (Next Release)

### 1. Multi-Presentation Management & Switching
- **Goal:** Allow presenters with multiple slide decks open on their laptop to view, monitor, and switch between them remotely from their phone.
- **Features:**
  - Enumerate all open presentations in PowerPoint via AppleScript (`presentations`) and Windows COM.
  - Expose active/focused presentation status to the phone client.
  - Mobile Deck Selector: Bottom sheet or dropdown displaying open decks with slide counts.
  - Remote Deck Switching: One-tap remote switch (`run slide show slide show settings of presentation X`) bringing the selected deck forward and loading its slide outline and speaker notes.
- **Status:** Architecture validated; native enumeration tested.

### 2. Silent Haptic Pacing Timer
- **Goal:** Keep the presenter on schedule without needing to glance at a clock or phone screen.
- **Features:**
  - Configurable target presentation duration (e.g. 15, 20, 45 minutes) set directly on mobile.
  - Palm vibration alerts via `navigator.vibrate`:
    - **50% elapsed:** 1 gentle pulse.
    - **75% elapsed:** 2 gentle pulses.
    - **90% elapsed / 2 min left:** 3 pulses.
    - **Overtime:** Periodic pulsing and timer display transitions to warning red.
- **Status:** Specification ready.

### 3. Visual Slide Grid & Live Search Jump Sheet
- **Goal:** Instant non-linear navigation during audience Q&A or discussions without paging through all intermediate slides in front of the audience.
- **Features:**
  - Drawer showing all slide numbers and titles.
  - Real-time search filter (e.g. typing "architecture" immediately narrows the list down to Slide 14).
  - One-tap jump (`GOTO`) directly to the target slide.
- **Status:** Wireframes specified.

---

## 🔮 Medium Priority

### 4. Slide Previews (Current & Next Slide Thumbnails)
- **Goal:** Give the speaker confidence on upcoming transitions by showing a preview of the next slide.
- **Features:**
  - Background thumbnail cache generation on deck load (`save curPres in tmpDir as save as PNG`).
  - Web-optimized thumbnail compression (WebP/JPEG ~360px wide).
  - Encrypted streaming over AES-256-GCM WebSocket channel (`PREVIEW_UPDATE`).
  - Minimal bandwidth overhead: cached locally on the phone once fetched.
- **Status:** Background export tested; requires thumbnail compression pipeline.

### 5. Virtual Laser Pointer / Spotlight via Phone Gyroscope
- **Goal:** Turn the phone into a precision laser pointer or spotlight using built-in motion sensors.
- **Features:**
  - Mobile sensor capture via `DeviceOrientationEvent` (`gamma` tilt left/right, `beta` tilt up/down).
  - Hold-to-point interaction: resting orientation calibrated upon touching the "Pointer" button.
  - Host-side renderer: Lightweight transparent click-through overlay window on the presentation display rendering a glowing red laser dot or dimming spotlight.
  - Exponential moving average smoothing to eliminate hand tremor jitter.
- **Status:** Sensor protocol defined.

### 6. Stage Mode Controls (Blackout / Whiteout)
- **Goal:** Pull audience focus back to the speaker during key storytelling moments or Q&A.
- **Features:**
  - One-touch Blackout (`B`) and Whiteout (`W`) buttons.
  - Visual status indication on mobile when the screen is currently blacked out.
- **Status:** AppleScript / COM hooks ready.

---

## 📦 Distribution & Packaging

### 7. Native Installers & Packages
- **macOS:**
  - Universal Binary (`darwin-arm64` + `darwin-amd64` via `lipo`).
  - Signed & notarized `.app` inside `.dmg` installer.
  - Homebrew formula (`brew install pptx-remote`).
- **Windows:**
  - InnoSetup / WiX installer (`.msi` or `.exe`).
  - Portable single `.exe` (pre-compiled with `-H=windowsgui` to run silently in the system tray).
  - Winget / Chocolatey package.

---

## 💡 Explorations & Community Ideas

- **Audience Q&A Scratchpad:** Speaker can jot down or bookmark slides during rehearsal for quick reference during Q&A.
- **Multi-Device Co-Presenting:** Allow two speakers with phones to pair to the same host with token delegation for dual-presenter talks.
- **Offline Direct Bluetooth / WebRTC:** P2P pairing when neither Wi-Fi nor internet is available in secure government/enterprise auditoriums.

---

*Suggestions and contributions are welcome! Please open an issue or PR following [CONTRIBUTING.md](CONTRIBUTING.md).*
