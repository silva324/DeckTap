package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
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
	ModeTunnel Mode = "tunnel" // public https URL through cloudflared (default)
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
	Mode        Mode
	TunnelState string
	TunnelError string
	PublicURL   string
	Session     session.Status
	Show        presenter.Status
	PairingURL  string
	DirectURL   string
	Pin         string
	Version     string
	Update      updater.Status
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
	if h.publicURL != "" && strings.EqualFold(origin, h.publicURL) {
		return true
	}
	// Allow canonical GitHub Pages PWA origin
	if strings.EqualFold(origin, "https://silva324.github.io") {
		return true
	}
	// Allow custom PWAURL origin if configured
	if h.cfg.PWAURL != "" && h.cfg.PWAURL != "direct" {
		if u, err := url.Parse(h.cfg.PWAURL); err == nil && u.Scheme != "" && u.Host != "" {
			pwaOrigin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
			if strings.EqualFold(origin, pwaOrigin) {
				return true
			}
		}
	}
	// Allow trycloudflare tunnel origins (e.g. from previously installed PWA or direct tunnel access)
	if strings.HasSuffix(strings.ToLower(origin), ".trycloudflare.com") {
		return true
	}
	if h.ln != nil {
		port := strconv.Itoa(h.ln.Addr().(*net.TCPAddr).Port)
		if origin == "http://127.0.0.1:"+port || origin == "http://localhost:"+port {
			return true
		}
	}
	return false
}

// Start binds 127.0.0.1 only (never 0.0.0.0), starts polling, the admin page and the tunnel.
func (h *Host) Start(parent context.Context) error {
	h.ctx, h.cancel = context.WithCancel(parent)
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(h.cfg.Port)))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	h.ln = ln
	mux := http.NewServeMux()
	mux.Handle("/ws", h.Hub)
	mux.Handle("/", webui.Handler())
	h.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go h.srv.Serve(ln)

	a, err := admin.Start(h.adminView, h.Reset, h.updater.ApplyUpdate)
	if err != nil {
		return err
	}
	h.admin = a
	h.updater.StartBackgroundCheck(h.ctx, 4*time.Hour)
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
		bin, err := tunnel.FindBinary(h.cfg.Cloudflared)
		if err != nil {
			h.setTunnel(TunnelFailed, err.Error(), "", nil)
			h.supervisorExit(true) // nothing to retry until it is installed; Reset tries again
			return
		}
		h.setTunnel(TunnelStarting, "", "", nil)
		ctx, cancel := context.WithTimeout(h.ctx, 45*time.Second)
		t, err := tunnel.Start(ctx, bin, h.Port(), h.log)
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
		h.log.Info("tunnel online", "url", t.URL)
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

// DirectURL returns the direct tunnel or loopback URL without canonical PWA wrapping.
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
	h.mu.Unlock()
	return s
}

// Describe turns a snapshot into short human-readable lines (tray, admin page, terminal).
func (s Snapshot) Describe() (headline, phone, show string, problem bool) {
	switch {
	case s.Session.State == session.LockedOut || s.TunnelState == TunnelLocked:
		headline, problem = "Stopped: too many failed connection attempts", true
	case s.Mode == ModeTunnel && s.TunnelState == TunnelStarting:
		headline = "Starting secure tunnel…"
	case s.Mode == ModeTunnel && s.TunnelState == TunnelFailed:
		headline, problem = "Tunnel problem: "+s.TunnelError, true
	case s.Mode == ModeTunnel:
		headline = "Running · tunnel online"
	default:
		headline = "Running · local test mode"
	}
	switch {
	case s.Session.State == session.Unpaired:
		phone = "Phone: waiting for pairing"
	case s.Session.Connected:
		phone = "Phone: connected 🔒"
	case s.Session.State == session.Paired:
		phone = "Phone: paired, reconnecting…"
	default:
		phone = "Phone: —"
	}
	switch {
	case !s.Show.AppRunning:
		show = "PowerPoint: not running"
	case s.Show.DeckName == "":
		show = "PowerPoint: no presentation open"
	case s.Show.Presenting:
		show = fmt.Sprintf("Presenting %d/%d · %s", s.Show.Slide, s.Show.Total, s.Show.DeckName)
	default:
		show = fmt.Sprintf("Ready · %s (%d slides)", s.Show.DeckName, s.Show.Total)
	}
	return
}

func (h *Host) adminView() admin.View {
	s := h.Snapshot()
	headline, phone, show, problem := s.Describe()
	v := admin.View{
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
	if s.PublicURL != "" {
		v.PublicHost = strings.TrimPrefix(s.PublicURL, "https://")
	}
	if s.Session.State == session.Paired {
		v.Detail = phone + " · " + show + " · the QR code was used and is no longer valid"
	}
	return v
}

// ApplyUpdate opens the latest download or release URL.
func (h *Host) ApplyUpdate() error { return h.updater.ApplyUpdate() }

// Updater returns the updater service instance.
func (h *Host) Updater() *updater.Service { return h.updater }

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
