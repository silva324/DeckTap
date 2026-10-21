# DeckTap Presenter & User Guide

Welcome to **DeckTap**. This guide covers everything you need to know to present confidently with DeckTap directly from your phone or mobile browser.

---

## 1. 🏷️ Smart Stage Cues in Speaker Notes

DeckTap automatically detects bracketed cues in your PowerPoint slide notes and illuminates them as distinct, color-coded visual badges. This allows you to spot upcoming stage directions in your peripheral vision without breaking eye contact with your audience.

### Supported Cues & Badges

| Cue Tag | Badge Style | Example / Usage | Recommended For |
| :--- | :--- | :--- | :--- |
| **`[DEMO]`** | 🟣 **Purple** | `[DEMO] Switch to VS Code and show agent execution` | Live coding, demos, screen switching |
| **`[PAUSE]`** / **`[WAIT]`** | 🟡 **Amber** | `[PAUSE] Let the audience read the numbers` | Strategic pauses, dramatic pacing, breathing |
| **`[CLICK]`** / **`[NEXT]`** | 🔵 **Cyan** | `[CLICK] Reveal next bullet point` | Animation triggers, staggered reveals |
| **`[QUESTION]`** / **`[Q&A]`** | 🟢 **Green** | `[QUESTION] Ask how many teams use CI today` | Audience polling, open questions |
| **`[WARN]`** / **`[ALERT]`** | 🔴 **Red** | `[WARN] Do not mention unannounced features` | Stage cautions, off-record remarks |
| **`[...]`** (Custom tags) | ⚪ **Neutral Pill** | `[SLIDE 5]`, `[TIP]`, `[CONTEXT]` | General notes bookmarks and section markers |

### How to Use
Simply type the bracketed tag directly into your slide notes in PowerPoint on your laptop (e.g., `[DEMO] Show live terminal`). DeckTap will automatically format it on your phone screen in real time.

---

## 2. 🤏 Pinch-to-Zoom Speaker Notes

Whether you are presenting on a small phone, a large tablet, or need larger text to read from a podium:

* **Two-Finger Pinch Gesture:** Place two fingers on the speaker notes area and pinch inward to shrink or pinch outward to enlarge (smoothly adjusts font size between **14px** and **48px**).
* **Zoom HUD Indicator:** A floating pill appears in the top corner displaying the exact font size (e.g., `24px`) and fades away automatically after 1 second.
* **Menu Quick Adjust:** You can also tap the menu (<kbd>☰</kbd>) and tap **`A-`** or **`A+`** to step font size in 2px increments.
* **Persistent Settings:** Your preferred font size is saved to your phone's local storage and restored automatically across presentation sessions.
* **Accidental Swipe Guard:** Slide navigation is locked while a two-finger pinch gesture is active, ensuring you never accidentally advance a slide while resizing text.

---

## 3. 🎯 Virtual Laser Pointer & Dimming Spotlight

DeckTap turns your phone screen into a trackpad controller for full-screen on-display overlays:

* **Laser Pointer:** Tap **Laser** in the bottom bar to project a bright, high-visibility red laser dot directly onto the presentation screen.
* **Spotlight:** Tap **Spot** to dim the entire projection screen while keeping a circular spotlight focused on the exact content you are discussing.
* **Slide Aspect Ratio Canvas:** The mobile controller displays a 16:9 widescreen reticle matching your slides, showing the current slide title, watermark number, and the first lines of your notes.
* **Recenter Button:** Tap **🎯 Center** (or double-tap the slide area) to instantly snap the pointer to the center of the display.
* **Optional Tilt/Gyro Aiming:** Tap **🧭 Tilt** if you prefer aiming by rotating and tilting your phone rather than dragging on the touchscreen.

---

## 4. 🧭 Slide Navigation & Stage Controls

| Action | Mobile Gesture / Control | Keyboard Shortcut |
| :--- | :--- | :--- |
| **Next Slide** | Tap **Next ›**, tap anywhere on right edge, or swipe left | <kbd>→</kbd>, <kbd>Space</kbd>, <kbd>PageDown</kbd>, <kbd>Enter</kbd> |
| **Previous Slide** | Tap **‹ Prev** or swipe right | <kbd>←</kbd>, <kbd>PageUp</kbd>, <kbd>Backspace</kbd> |
| **Jump to Slide** | Tap menu (<kbd>☰</kbd>) $\to$ select any slide from drawer list | Click slide in list |
| **Blackout Screen** | Tap menu (<kbd>☰</kbd>) $\to$ tap **Blackout** | <kbd>B</kbd> or <kbd>.</kbd> |
| **Elapsed Stopwatch** | Tap the timer (<kbd>00:00</kbd>) at top to reset to 0 | — |
| **End Presentation** | Tap menu (<kbd>☰</kbd>) $\to$ tap **End Show** | <kbd>Esc</kbd> |

---

## 5. 📱 PWA Installation & Pairing

DeckTap runs as a full Progressive Web App (PWA) with offline caching and camera support:

1. **Add to Home Screen:**
   - **iOS Safari:** Tap the **Share** button $\to$ select **Add to Home Screen**.
   - **Android Chrome:** Tap the install banner at the bottom or the browser menu $\to$ **Install App**.
2. **Instant In-App Scanner:**
   - Tap **Scan QR** on the ready screen to launch the built-in camera scanner (supports flashlight/torch toggle and front/rear lens switching).
   - Point your camera at the QR code on your laptop screen.
3. **Visual 4-Digit PIN:**
   - Enter the 4-digit PIN displayed on your laptop host window into the phone keypad.
   - All session traffic is secured with **AES-256-GCM** end-to-end encryption.
