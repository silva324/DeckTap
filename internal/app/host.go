package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"pptx-remote/internal/admin"
	"pptx-remote/internal/pointer"
	"pptx-remote/internal/presenter"
	"pptx-remote/internal/session"
	"pptx-remote/internal/tunnel"
	"pptx-remote/internal/updater"
	"pptx-remote/internal/webui"
)

type Mode string

const (
	ModeTunnel Mode = "tunnel" // public https URL through tunnel (default)
	ModeLAN    Mode = "lan"    // local area network (Wi-Fi direct, zero internet needed)
	ModeLocal  Mode = "local"  // loopback only, for testing in a desktop browser
)

// DefaultPWAURL is the canonical PWA origin hosted on GitHub Pages.
const DefaultPWAURL = "https://silva324.github.io/DeckTap/"

type Config struct {
	Mode         Mode
	Port         int // 0 = OS-assigned
	Cloudflared  string
	PWAURL       string // Canonical PWA entry point (default: DefaultPWAURL). "direct" = direct tunnel URL.
	MaxFailures  int
	PollInterval time.Duration
	Version      string
}

// TunnelState values shown in the UI.
const (
	TunnelOff      = "off"
	TunnelStarting = "starting"
	TunnelOnline   = "online"
	TunnelFailed   = "failed"
	TunnelLocked   = "locked"
)

// Snapshot is everything the tray/admin page needs.
type Snapshot struct {
	Mode           Mode
	TunnelState    string
	TunnelError    string
	TunnelProvider string
	PublicURL      string
	Session        session.Status
	Show           presenter.Status
	PairingURL     string
	DirectURL      string
	Pin            string
	Version        string
	Update         updater.Status
}

// Host owns the servers, the tunnel and the session hub.
type Host struct {
	cfg     Config
	log     *slog.Logger
	ctl     presenter.Controller
	Hub     *session.Hub
	Sync    *Sync
	pointer pointer.Controller
	updater *updater.Service

	ln    net.Listener
	srv   *http.Server
	admin *admin.Server

	ctx    context.Context
	cancel context.CancelFunc

	mu          sync.Mutex
	tun         *tunnel.Tunnel
	tunnelState string
	tunnelErr   string
	publicURL   string
	supervising bool
	listeners   []func()
	stopping    bool
}

func NewHost(cfg Config, ctl presenter.Controller, log *slog.Logger) *Host {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	ver := cfg.Version
	if ver == "" {
		ver = "0.0.1"
	}
	if cfg.PWAURL == "" {
		cfg.PWAURL = DefaultPWAURL
	}
	h := &Host{
		cfg:         cfg,
		log:         log,
		ctl:         ctl,
		tunnelState: TunnelOff,
		pointer:     pointer.New(),
		updater:     updater.NewService(ver, nil),
	}
	h.Sync = NewSync(ctl, log, cfg.PollInterval)
	h.Sync.version = ver
	h.Hub = session.New(session.Options{
		MaxFailures:     cfg.MaxFailures,
		Logger:          log,
		AllowedOrigin:   h.allowedOrigin,
		CurrentState:    h.Sync.CurrentState,
		OnAuthenticated: h.Sync.PushDeck,
		OnCommand: func(c session.Command) {
			if c.Action == "POINTER" {
				h.pointer.Update(c.PointerActive, c.PointerX, c.PointerY, c.PointerMode)
				return
			}
			h.Sync.Handle(h.ctx, c)
		},
		OnLockout: h.onLockout,
		OnChange:  h.notify,
	})
	h.Sync.send = h.Hub.Send
	h.Sync.onChange = h.notify
	return h
}

// OnChange registers a callback fired whenever the snapshot may have changed.
func (h *Host) OnChange(f func()) {
	h.mu.Lock()
	h.listeners = append(h.listeners, f)
	h.mu.Unlock()
}

func (h *Host) notify() {
	h.mu.Lock()
	ls := append([]func(){}, h.listeners...)
	h.mu.Unlock()
	for _, f := range ls {
		f()
	}
}

func (h *Host) allowedOrigin(origin string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	cleanOrigin := strings.TrimRight(strings.ToLower(origin), "/")

	// Allow null origin (sent by iOS Safari in private browsing, standalone Web Clips, or strict privacy)
	// and apple-web-clip schemes. All WebSocket traffic is end-to-end encrypted with AES-256-GCM.
	if cleanOrigin == "null" || cleanOrigin == "" || strings.HasPrefix(cleanOrigin, "apple-web-clip:") {
		return true
	}

	if h.publicURL != "" && strings.EqualFold(cleanOrigin, strings.TrimRight(strings.ToLower(h.publicURL), "/")) {
		return true
	}
	// Allow canonical GitHub Pages PWA origin
	if cleanOrigin == "https://silva324.github.io" {
		return true
	}
	// Allow custom PWAURL origin if configured
	if h.cfg.PWAURL != "" && h.cfg.PWAURL != "direct" {
		if u, err := url.Parse(h.cfg.PWAURL); err == nil && u.Scheme != "" && u.Host != "" {
			pwaOrigin := strings.TrimRight(strings.ToLower(fmt.Sprintf("%s://%s", u.Scheme, u.Host)), "/")
			if cleanOrigin == pwaOrigin {
				return true
			}
		}
	}
	// Allow trycloudflare tunnel origins (e.g. from previously installed PWA or direct tunnel access)
	if strings.HasSuffix(cleanOrigin, ".trycloudflare.com") {
		return true
	}
	if h.ln != nil {
		port := strconv.Itoa(h.ln.Addr().(*net.TCPAddr).Port)
		if cleanOrigin == "http://127.0.0.1:"+port || cleanOrigin == "http://localhost:"+port {
			return true
		}
		// In LAN mode, allow connections originating from local or private IP addresses on this port
		if u, err := url.Parse(cleanOrigin); err == nil && u.Scheme == "http" && u.Port() == port {
			host := u.Hostname()
			if ip := net.ParseIP(host); ip != nil {
				if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
					return true
				}
			}
			lanIP := tunnel.GetPreferredOutboundIP()
			if host == lanIP {
				return true
			}
		}
	}
	return false
}

// Start binds loopback (or 0.0.0.0 in LAN mode), starts polling, the admin page and the tunnel.
func (h *Host) Start(parent context.Context) error {
	h.ctx, h.cancel = context.WithCancel(parent)
	bindHost := "127.0.0.1"
	if h.cfg.Mode == ModeLAN {
		bindHost = "0.0.0.0"
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(bindHost, strconv.Itoa(h.cfg.Port)))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	h.ln = ln
	mux := http.NewServeMux()
	mux.Handle("/ws", h.Hub)
	mux.Handle("/", webui.Handler())
	h.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go h.srv.Serve(ln)

	h.updater.SetRestartHandler(func() error {
		h.log.Info("DeckTap self-update applied; stopping host for restart...")
		go func() {
			time.Sleep(300 * time.Millisecond)
			h.cancel()
			time.Sleep(200 * time.Millisecond)
			os.Exit(0)
		}()
		return nil
	})

	a, err := admin.Start(h.adminView, h.Reset, h.updater.ApplyUpdate, h.updater.OpenReleasePage)
	if err != nil {
		return err
	}
	a.SetQuitHandler(h.Quit)
	a.SetCheckUpdateHandler(h.CheckUpdate)
	a.SetReloadDeckHandler(h.ReloadDeck)
	a.SetModeHandler(func(m string) error {
		return h.SetMode(Mode(m))
	})
	h.admin = a
	h.updater.StartBackgroundCheck(h.ctx, 1*time.Hour)
	go h.Sync.Run(h.ctx)

	if h.cfg.Mode == ModeTunnel {
		h.startTunnelSupervisor()
	}
	h.log.Info("DeckTap started", "mode", h.cfg.Mode, "listen", ln.Addr().String(), "controller", h.ctl.Name())
	return nil
}

func (h *Host) Port() int { return h.ln.Addr().(*net.TCPAddr).Port }

// AdminURL is the laptop-only pairing page.
func (h *Host) AdminURL() string { return h.admin.URL() }

func (h *Host) setTunnel(state, errMsg, url string, t *tunnel.Tunnel) {
	h.mu.Lock()
	h.tunnelState, h.tunnelErr, h.publicURL, h.tun = state, errMsg, url, t
	h.mu.Unlock()
	h.notify()
}

func (h *Host) startTunnelSupervisor() {
	h.mu.Lock()
	if h.supervising || h.stopping {
		h.mu.Unlock()
		return
	}
	h.supervising = true
	h.mu.Unlock()
	go h.superviseTunnel()
}

// supervisorExit atomically decides whether the supervisor must stop; deciding and
// clearing `supervising` under one lock means Reset can never miss a restart.
func (h *Host) supervisorExit(force bool) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if force || h.stopping || h.ctx.Err() != nil || h.tunnelState == TunnelLocked {
		h.supervising = false
		return true
	}
	return false
}

// superviseTunnel keeps one quick tunnel alive. A new tunnel means a new URL, so the
// pairing is reset (the paired phone could never reach the new URL anyway).
func (h *Host) superviseTunnel() {
	backoff := 2 * time.Second
	for first := true; ; first = false {
		if h.supervisorExit(false) {
			return
		}
		h.setTunnel(TunnelStarting, "", "", nil)
		ctx, cancel := context.WithTimeout(h.ctx, 45*time.Second)
		t, err := h.startTunnel(ctx)
		cancel()
		if err != nil {
			h.log.Warn("tunnel failed to start", "err", err)
			h.setTunnel(TunnelFailed, err.Error(), "", nil)
			select {
			case <-h.ctx.Done():
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		backoff = 2 * time.Second
		if !first && h.Hub.Status().State == session.Paired {
			h.Hub.Reset()
		}
		h.log.Info("tunnel online", "provider", t.Provider, "url", t.URL)
		h.setTunnel(TunnelOnline, "", t.URL, t)

		select {
		case <-h.ctx.Done():
			t.Stop()
		case <-t.Done():
			h.log.Warn("tunnel exited", "err", t.Err())
		}
		h.mu.Lock()
		if h.tunnelState == TunnelOnline && h.tun == t { // dropped by itself, not stopped by us
			h.tunnelState, h.tunnelErr, h.publicURL, h.tun = TunnelFailed, "tunnel dropped, reconnecting", "", nil
		}
		h.mu.Unlock()
		h.notify()
	}
}

func (h *Host) startTunnel(ctx context.Context) (*tunnel.Tunnel, error) {
	bin, err := tunnel.FindBinary(h.cfg.Cloudflared)
	if err != nil {
		return nil, err
	}
	return tunnel.Start(ctx, bin, h.Port(), h.log)
}

func (h *Host) stopTunnel(state string) {
	h.mu.Lock()
	t := h.tun
	h.tun = nil
	h.tunnelState, h.publicURL = state, ""
	h.mu.Unlock()
	if t != nil {
		t.Stop()
	}
	h.notify()
}

// onLockout destroys the tunnel after too many failed handshakes (SPEC-SEC-001 §4).
func (h *Host) onLockout() {
	h.log.Error("too many failed attempts: tunnel destroyed; use Reset to start again")
	if h.cfg.Mode == ModeTunnel {
		h.stopTunnel(TunnelLocked)
	}
}

// Reset is the hard reset: new token, key and session; kicks the phone. After a lockout
// it also brings up a brand-new tunnel (new URL).
func (h *Host) Reset() {
	h.Hub.Reset()
	if h.cfg.Mode == ModeTunnel {
		h.mu.Lock()
		if h.tunnelState == TunnelLocked {
			h.tunnelState = TunnelStarting // a still-running supervisor will now loop instead of exiting
		}
		online := h.tunnelState == TunnelOnline
		h.mu.Unlock()
		if !online {
			h.startTunnelSupervisor()
		}
	}
	h.notify()
}

// DirectURL returns the direct tunnel, LAN, or loopback URL without canonical PWA wrapping.
func (h *Host) DirectURL() string {
	frag := h.Hub.Fragment()
	if frag == "" {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	switch h.cfg.Mode {
	case ModeTunnel:
		if h.tunnelState != TunnelOnline || h.publicURL == "" {
			return ""
		}
		return h.publicURL + "/#" + frag
	case ModeLAN:
		if h.ln == nil {
			return ""
		}
		ip := tunnel.GetPreferredOutboundIP()
		port := h.ln.Addr().(*net.TCPAddr).Port
		return fmt.Sprintf("http://%s:%d/#%s", ip, port, frag)
	default:
		if h.ln == nil {
			return ""
		}
		return fmt.Sprintf("http://127.0.0.1:%d/#%s", h.ln.Addr().(*net.TCPAddr).Port, frag)
	}
}

// PairingURL is the QR content, or "" when not available (token burned, tunnel down…).
func (h *Host) PairingURL() string {
	frag := h.Hub.Fragment()
	if frag == "" {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	switch h.cfg.Mode {
	case ModeTunnel:
		if h.tunnelState != TunnelOnline || h.publicURL == "" {
			return ""
		}
		if h.cfg.PWAURL != "" && h.cfg.PWAURL != "direct" {
			basePWA := h.cfg.PWAURL
			if !strings.HasSuffix(basePWA, "/") {
				basePWA += "/"
			}
			tunnelHost := strings.TrimPrefix(h.publicURL, "https://")
			tunnelHost = strings.TrimPrefix(tunnelHost, "http://")
			tunnelHost = strings.TrimRight(tunnelHost, "/")
			return fmt.Sprintf("%s#%s@%s", basePWA, frag, tunnelHost)
		}
		return h.publicURL + "/#" + frag
	case ModeLAN:
		if h.ln == nil {
			return ""
		}
		ip := tunnel.GetPreferredOutboundIP()
		port := h.ln.Addr().(*net.TCPAddr).Port
		return fmt.Sprintf("http://%s:%d/#%s", ip, port, frag)
	default:
		if h.ln == nil {
			return ""
		}
		return fmt.Sprintf("http://127.0.0.1:%d/#%s", h.ln.Addr().(*net.TCPAddr).Port, frag)
	}
}

func (h *Host) Snapshot() Snapshot {
	s := Snapshot{
		Session:    h.Hub.Status(),
		Show:       h.Sync.Status(),
		PairingURL: h.PairingURL(),
		DirectURL:  h.DirectURL(),
		Pin:        h.Hub.Pin(),
		Version:    h.cfg.Version,
		Update:     h.updater.Status(),
	}
	h.mu.Lock()
	s.Mode, s.TunnelState, s.TunnelError, s.PublicURL = h.cfg.Mode, h.tunnelState, h.tunnelErr, h.publicURL
	if h.tun != nil && h.tun.Provider != "" {
		s.TunnelProvider = h.tun.Provider
	} else {
		s.TunnelProvider = "cloudflared"
	}
	h.mu.Unlock()
	return s
}

// Describe turns a snapshot into short human-readable lines (tray, admin page, terminal).
func (s Snapshot) Describe() (headline, phone, show string, problem bool) {
	switch {
	case s.Session.State == session.LockedOut || s.TunnelState == TunnelLocked:
		headline, problem = "Connection paused: multiple incorrect PIN attempts. Click 'New Pairing Code' to resume.", true
	case s.Mode == ModeLAN:
		headline = "Ready · Local Wi-Fi (offline)"
	case s.Mode == ModeTunnel && s.TunnelState == TunnelStarting:
		headline = "Preparing secure connection…"
	case s.Mode == ModeTunnel && s.TunnelState == TunnelFailed:
		headline, problem = "Connection issue: "+s.TunnelError, true
	case s.Mode == ModeTunnel:
		headline = "Ready to connect"
	default:
		headline = "Ready · local test mode"
	}
	switch {
	case s.Session.State == session.Unpaired:
		if s.Mode == ModeLAN {
			phone = "Connect phone to same Wi-Fi & scan QR"
		} else {
			phone = "Scan QR code with your phone"
		}
	case s.Session.Connected:
		phone = "Phone connected 🔒"
	case s.Session.State == session.Paired:
		phone = "Phone paired, reconnecting…"
	default:
		phone = "Ready"
	}
	switch {
	case !s.Show.AppRunning:
		show = "PowerPoint not detected"
	case s.Show.DeckName == "":
		show = "No presentation open"
	case s.Show.Presenting:
		show = fmt.Sprintf("Slide %d of %d · %s", s.Show.Slide, s.Show.Total, s.Show.DeckName)
	default:
		show = fmt.Sprintf("%s (%d slides ready)", s.Show.DeckName, s.Show.Total)
	}
	return
}

func (h *Host) adminView() admin.View {
	s := h.Snapshot()
	headline, phone, show, problem := s.Describe()
	v := admin.View{
		Mode:           string(s.Mode),
		Headline:       headline,
		Detail:         phone + " · " + show,
		PairingURL:     s.PairingURL,
		DirectURL:      s.DirectURL,
		PinCode:        s.Pin,
		Problem:        problem,
		Paired:         s.Session.State == session.Paired,
		CurrentVersion: s.Version,
		Update:         s.Update,
	}
	if s.Mode == ModeLAN {
		v.LANIP = tunnel.GetPreferredOutboundIP()
	}
	if s.PublicURL != "" {
		v.PublicHost = strings.TrimPrefix(s.PublicURL, "https://")
	}
	if s.Session.State == session.Paired {
		v.Detail = phone + " · " + show + " · Remote control is active"
	}
	return v
}

// SetMode dynamically switches the host's operating mode between ModeTunnel, ModeLAN, and ModeLocal.
func (h *Host) SetMode(m Mode) error {
	h.mu.Lock()
	if h.cfg.Mode == m {
		h.mu.Unlock()
		return nil
	}
	prevMode := h.cfg.Mode
	h.cfg.Mode = m
	h.mu.Unlock()

	h.log.Info("switching operating mode", "from", prevMode, "to", m)

	// Stop current tunnel if transitioning away from ModeTunnel
	if prevMode == ModeTunnel {
		h.supervisorExit(true)
		h.stopTunnel(TunnelOff)
	}

	targetBind := "127.0.0.1"
	if m == ModeLAN {
		targetBind = "0.0.0.0"
	}

	h.mu.Lock()
	currentPort := 0
	if h.ln != nil {
		currentPort = h.ln.Addr().(*net.TCPAddr).Port
	}
	h.mu.Unlock()

	// Rebind listener if moving between LAN (0.0.0.0) and Tunnel/Local (127.0.0.1)
	if (prevMode == ModeLAN && m != ModeLAN) || (prevMode != ModeLAN && m == ModeLAN) {
		if err := h.rebindServer(targetBind, currentPort); err != nil {
			h.log.Error("failed to rebind server for mode change", "mode", m, "err", err)
		}
	}

	// Start tunnel supervisor if entering ModeTunnel
	if m == ModeTunnel {
		h.startTunnelSupervisor()
	}

	// Reset pairing credentials to burn old token/keys and generate fresh ones
	h.Hub.Reset()
	h.notify()
	return nil
}

func (h *Host) rebindServer(bindHost string, port int) error {
	h.mu.Lock()
	oldSrv := h.srv
	oldLn := h.ln
	h.mu.Unlock()

	if oldSrv != nil {
		_ = oldSrv.Close()
	}
	if oldLn != nil {
		_ = oldLn.Close()
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(bindHost, strconv.Itoa(port)))
	if err != nil {
		ln, err = net.Listen("tcp", net.JoinHostPort(bindHost, "0"))
		if err != nil {
			return fmt.Errorf("rebind listen on %s: %w", bindHost, err)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", h.Hub)
	mux.Handle("/", webui.Handler())
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	h.mu.Lock()
	h.ln = ln
	h.srv = srv
	h.mu.Unlock()

	go srv.Serve(ln)
	return nil
}

// ApplyUpdate opens the latest download or release URL.
func (h *Host) ApplyUpdate() error { return h.updater.ApplyUpdate() }

// CheckUpdate queries GitHub for the latest release on demand.
func (h *Host) CheckUpdate(ctx context.Context) (updater.Status, error) {
	st, err := h.updater.Check(ctx)
	h.notify()
	return st, err
}

// Updater returns the updater service instance.
func (h *Host) Updater() *updater.Service { return h.updater }

// ReloadDeck forces an immediate refresh of slide titles and notes from PowerPoint.
func (h *Host) ReloadDeck() {
	if h.Sync != nil {
		h.Sync.ReloadDeck()
	}
}

// Quit gracefully terminates DeckTap and exits the process.
func (h *Host) Quit() {
	h.log.Info("Quit requested from desktop interface")
	go func() {
		h.Shutdown()
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
}

// Shutdown tears everything down (idempotent): phone, tunnel, servers, controller.
func (h *Host) Shutdown() {
	h.mu.Lock()
	if h.stopping {
		h.mu.Unlock()
		return
	}
	h.stopping = true
	h.mu.Unlock()
	h.Hub.Shutdown()
	time.Sleep(150 * time.Millisecond) // let the close frame reach the phone
	h.stopTunnel(TunnelOff)
	if h.cancel != nil {
		h.cancel()
	}
	if h.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		h.srv.Shutdown(ctx)
		cancel()
	}
	if h.admin != nil {
		h.admin.Close()
	}
	if h.pointer != nil {
		h.pointer.Close()
	}
	h.ctl.Close()
	h.log.Info("DeckTap stopped")
}
