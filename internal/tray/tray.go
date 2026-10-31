//go:build !notray

package tray

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"fyne.io/systray"

	"github.com/silva324/DeckTap/internal/app"
)

// Supported reports whether this build has a tray.
func Supported() bool { return true }

// Quit stops the tray loop (safe to call from any goroutine).
func Quit() { systray.Quit() }

// Run blocks on the main thread until Quit. openURL opens a page in the browser.
func Run(h *app.Host, openURL func(string), onExit func()) {
	systray.Run(func() { setup(h, openURL) }, onExit)
}

func setup(h *app.Host, openURL func(string)) {
	switch runtime.GOOS {
	case "darwin":
		systray.SetTemplateIcon(iconTemplate, iconTemplate)
	case "windows":
		systray.SetIcon(iconICO)
	default:
		systray.SetIcon(iconPNG)
	}
	systray.SetTooltip("DeckTap")

	mStatus := systray.AddMenuItem("Starting…", "")
	mStatus.Disable()
	mPin := systray.AddMenuItem("PIN: —", "Anti-Shoulder-Surfing PIN")
	mPin.Disable()
	mPhone := systray.AddMenuItem("Phone: —", "")
	mPhone.Disable()
	mShow := systray.AddMenuItem("PowerPoint: —", "")
	mShow.Disable()
	systray.AddSeparator()
	mModeTunnel := systray.AddMenuItemCheckbox("🌐 Cloud Tunnel (Internet)", "Connect over Internet tunnel (Cloudflare)", h.Snapshot().Mode != app.ModeLAN)
	mModeLAN := systray.AddMenuItemCheckbox("📶 Local Wi-Fi (Offline / Direct)", "Direct connection over local Wi-Fi or Hotspot", h.Snapshot().Mode == app.ModeLAN)
	systray.AddSeparator()
	mUpdate := systray.AddMenuItem("✨ Update Available…", "Download the latest version of DeckTap")
	mUpdate.Hide()
	mReloadDeck := systray.AddMenuItem("Reload notes & slides", "Re-read presentation notes and slides from PowerPoint")
	mCheckUpdate := systray.AddMenuItem("Check for updates…", "Check GitHub for a newer version of DeckTap")
	mQR := systray.AddMenuItem("Show pairing QR…", "Open the pairing page on this computer")
	mReset := systray.AddMenuItem("Reset pairing (new QR)", "Disconnect the phone and rotate all keys")
	systray.AddSeparator()
	mAbout := systray.AddMenuItem("About DeckTap…", "View DeckTap on GitHub")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit DeckTap", "Stop the tunnel and quit")

	update := func() {
		s := h.Snapshot()
		headline, phone, show, problem := s.Describe()
		prefix := "● "
		if problem {
			prefix = "⚠︎ "
		}
		mStatus.SetTitle(prefix + headline)
		if s.Pin != "" {
			mPin.SetTitle("PIN: " + s.Pin)
		}
		mPhone.SetTitle(phone)
		mShow.SetTitle(show)
		if s.PairingURL != "" {
			mQR.SetTitle("Show pairing QR…")
			mQR.Enable()
		} else {
			mQR.SetTitle("Show status page…")
		}
		if s.Update.Available {
			mUpdate.SetTitle(fmt.Sprintf("✨ Update to %s…", s.Update.LatestVer))
			mUpdate.Show()
		} else {
			mUpdate.Hide()
		}
		if s.Mode == app.ModeLAN {
			mModeLAN.Check()
			mModeTunnel.Uncheck()
		} else {
			mModeTunnel.Check()
			mModeLAN.Uncheck()
		}
		verLabel := s.Version
		if verLabel != "" {
			if !strings.HasPrefix(verLabel, "v") {
				verLabel = "v" + verLabel
			}
			mAbout.SetTitle("About DeckTap (" + verLabel + ")…")
		} else {
			mAbout.SetTitle("About DeckTap…")
		}
		title := ""
		if s.Show.Presenting && s.Show.Total > 0 {
			title = fmt.Sprintf("%d/%d", s.Show.Slide, s.Show.Total)
		}
		if runtime.GOOS == "darwin" {
			systray.SetTitle(title)
		}
		systray.SetTooltip("DeckTap · " + headline + " · " + phone)
	}

	changes := make(chan struct{}, 1)
	h.OnChange(func() {
		select {
		case changes <- struct{}{}:
		default:
		}
	})
	update()
	go func() {
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-changes:
				update()
			case <-tick.C:
				update()
			case <-mModeTunnel.ClickedCh:
				_ = h.SetMode(app.ModeTunnel)
			case <-mModeLAN.ClickedCh:
				_ = h.SetMode(app.ModeLAN)
			case <-mQR.ClickedCh:
				openURL(h.AdminURL())
			case <-mUpdate.ClickedCh:
				_ = h.ApplyUpdate()
			case <-mReloadDeck.ClickedCh:
				mReloadDeck.SetTitle("Reloading notes…")
				h.ReloadDeck()
				go func() {
					time.Sleep(1 * time.Second)
					mReloadDeck.SetTitle("Reload notes & slides")
				}()
			case <-mCheckUpdate.ClickedCh:
				mCheckUpdate.SetTitle("Checking for updates…")
				go func() {
					st, err := h.CheckUpdate(context.Background())
					if err == nil && st.Available {
						mCheckUpdate.SetTitle("✨ Update to " + st.LatestVer + "…")
					} else if err == nil {
						mCheckUpdate.SetTitle("✓ DeckTap is up to date")
						time.Sleep(3 * time.Second)
						mCheckUpdate.SetTitle("Check for updates…")
					} else {
						mCheckUpdate.SetTitle("Check failed")
						time.Sleep(3 * time.Second)
						mCheckUpdate.SetTitle("Check for updates…")
					}
				}()
			case <-mReset.ClickedCh:
				h.Reset()
				openURL(h.AdminURL())
			case <-mAbout.ClickedCh:
				_ = OpenURL("https://github.com/silva324/DeckTap")
			case <-mQuit.ClickedCh:
				go h.Quit()
				systray.Quit()
				return
			}
		}
	}()
}
