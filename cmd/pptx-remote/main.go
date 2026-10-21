// DeckTap: control presentations from your phone and read speaker notes,
// over an ephemeral, end-to-end encrypted tunnel (AES-256-GCM + Anti-Shoulder-Surfing PIN).
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"pptx-remote/internal/app"
	"pptx-remote/internal/nativewin"
	"pptx-remote/internal/presenter"
	"pptx-remote/internal/qr"
	"pptx-remote/internal/tray"
	"pptx-remote/internal/tunnel"
)

var version = "0.0.1"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		os.Exit(doctor(os.Args[2:]))
	}

	mode := flag.String("mode", "tunnel", "tunnel (public HTTPS via cloudflared) | local (127.0.0.1 only, for desktop testing)")
	port := flag.Int("port", 0, "local port (0 = OS-assigned)")
	cloudflared := flag.String("cloudflared", "", "path to the cloudflared binary (default: auto-detect & auto-download)")
	demo := flag.String("demo", "", "use the demo controller with this .pptx instead of PowerPoint")
	pwaURL := flag.String("pwa-url", app.DefaultPWAURL, "canonical PWA web app URL for pairing QR ('direct' for direct tunnel/LAN URL)")
	noTray := flag.Bool("no-tray", false, "run in the terminal without a menu-bar/tray icon")
	noBrowser := flag.Bool("no-browser", false, "do not open the pairing page automatically")
	maxFailures := flag.Int("max-failures", 5, "failed handshakes before the tunnel is destroyed")
	verbose := flag.Bool("v", false, "verbose logging")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println("DeckTap", version)
		return
	}

	interactive := isTerminal(os.Stdin) && isTerminal(os.Stdout)
	log, closeLog := setupLogging(interactive, *verbose)
	defer closeLog()

	ctl, err := newController(*demo)
	if err != nil {
		fatal(log, err)
	}
	m := app.Mode(*mode)
	if m != app.ModeTunnel && m != app.ModeLocal {
		fatal(log, fmt.Errorf("unknown mode %q", *mode))
	}
	host := app.NewHost(app.Config{
		Mode:        m,
		Port:        *port,
		Cloudflared: *cloudflared,
		PWAURL:      *pwaURL,
		MaxFailures: *maxFailures,
		Version:     version,
	}, ctl, log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := host.Start(ctx); err != nil {
		fatal(log, err)
	}
	var shutdownOnce sync.Once
	shutdown := func() { shutdownOnce.Do(host.Shutdown) }
	defer shutdown() // also on panics in main: never leave the tunnel running

	// Open the pairing page once the QR is ready, and keep the terminal QR up to date.
	var mu sync.Mutex
	lastQR, opened := "", false
	host.OnChange(func() {
		snap := host.Snapshot()
		url := snap.PairingURL
		mu.Lock()
		defer mu.Unlock()
		if url == "" || url == lastQR {
			return
		}
		lastQR = url
		if interactive {
			printQR(url, snap.Pin)
		}
		if !opened && !*noBrowser {
			opened = true
			showAdminWindow(host.AdminURL())
		}
	})
	if interactive {
		fmt.Printf("\n  DeckTap %s · controller: %s · mode: %s\n", version, ctl.Name(), m)
		fmt.Println("  Waiting for secure tunnel…   [R]+Enter = reset pairing · [Q]+Enter = quit")
		go readKeys(host, stop)
	}

	if tray.Supported() && !*noTray {
		go func() { <-ctx.Done(); tray.Quit() }()
		tray.Run(host, showAdminWindow, shutdown) // blocks on the main thread (required by macOS)
		return
	}
	<-ctx.Done()
}

func newController(demo string) (presenter.Controller, error) {
	if demo != "" {
		return presenter.NewDemo(demo)
	}
	return presenter.NewPlatform()
}

func readKeys(host *app.Host, quit func()) {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		switch strings.ToLower(strings.TrimSpace(sc.Text())) {
		case "r":
			fmt.Println("  Resetting pairing: new token, key and PIN…")
			host.Reset()
		case "q":
			quit()
			return
		case "s":
			h, p, s, _ := host.Snapshot().Describe()
			fmt.Printf("  %s\n  %s\n  %s\n", h, p, s)
		}
	}
}

func printQR(url, pin string) {
	code, err := qr.Encode([]byte(url))
	if err != nil {
		return
	}
	fmt.Println("\n  Scan with your phone camera (one phone only):")
	fmt.Print(code.Terminal())
	fmt.Printf(`
  ┌────────────────────────────────────────────────────────────┐
  │  ANTI-SHOULDER-SURFING PIN CODE: [ %s ]                   │
  │  Enter this PIN on your phone after scanning the QR code.  │
  └────────────────────────────────────────────────────────────┘
`, pin)
	fmt.Println("  Or open the pairing page from the menu-bar icon.  [R]+Enter = new QR & PIN · [Q]+Enter = quit")
}

func showAdminWindow(u string) {
	nativewin.Open(nativewin.WindowConfig{
		URL:    u,
		Title:  "DeckTap — Pairing & Status",
		Width:  460,
		Height: 680,
	})
}

func openURL(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	_ = cmd.Start()
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func setupLogging(interactive, verbose bool) (*slog.Logger, func()) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	var w io.Writer = os.Stderr
	closeFn := func() {}
	if !interactive {
		dir := logDir()
		if err := os.MkdirAll(dir, 0o700); err == nil {
			if f, err := os.OpenFile(filepath.Join(dir, "decktap.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); err == nil {
				w, closeFn = f, func() { f.Close() }
			}
		}
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})), closeFn
}

func logDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Logs", "DeckTap")
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "DeckTap")
		}
	}
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "decktap")
	}
	return os.TempDir()
}

func fatal(log *slog.Logger, err error) {
	log.Error("fatal", "err", err)
	fmt.Fprintln(os.Stderr, "DeckTap:", err)
	os.Exit(1)
}

func doctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	demo := fs.String("demo", "", "check the demo controller with this .pptx")
	cfPath := fs.String("cloudflared", "", "path to cloudflared")
	_ = fs.Parse(args)
	fmt.Printf("DeckTap doctor · %s · %s/%s\n\n", version, runtime.GOOS, runtime.GOARCH)
	failed := false
	if bin, err := tunnel.FindBinary(*cfPath); err != nil {
		fmt.Println("FAIL cloudflared:", err)
		failed = true
	} else {
		out, _ := exec.Command(bin, "--version").CombinedOutput()
		fmt.Printf("OK   cloudflared: %s (%s)\n", bin, strings.TrimSpace(string(out)))
	}
	fmt.Printf("INFO tray icon in this build: %v\n", tray.Supported())
	ctl, err := newController(*demo)
	if err != nil {
		fmt.Println("FAIL controller:", err)
		return 1
	}
	defer ctl.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	for _, line := range ctl.Diagnose(ctx) {
		fmt.Println(line)
		if strings.HasPrefix(line, "FAIL") {
			failed = true
		}
	}
	if failed {
		return 1
	}
	return 0
}
