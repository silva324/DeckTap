//go:build !notray

package tray

import (
	"fmt"
	"runtime"
	"time"

	"fyne.io/systray"

	"pptx-remote/internal/app"
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
	mUpdate := systray.AddMenuItem("✨ Update Available…", "Download the latest version of DeckTap")
	mUpdate.Hide()
	mQR := systray.AddMenuItem("Show pairing QR…", "Open the pairing page on this computer")
	mReset := systray.AddMenuItem("Reset pairing (new QR)", "Disconnect the phone and rotate all keys")
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
			case <-mQR.ClickedCh:
				openURL(h.AdminURL())
			case <-mUpdate.ClickedCh:
				_ = h.ApplyUpdate()
			case <-mReset.ClickedCh:
				h.Reset()
				openURL(h.AdminURL())
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}
