// Package admin serves the laptop-only pairing page (QR code + status).
//
// It listens on its own loopback port, separate from the public server, because in
// tunnel mode every public request also arrives from 127.0.0.1 (cloudflared). The
// cloudflared tunnel only points at the public port, and the admin URL contains a
// random 192-bit path secret, so neither the internet nor other local users can read
// the QR (which carries the key).
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"pptx-remote/internal/e2ee"
	"pptx-remote/internal/qr"
	"pptx-remote/internal/updater"
)

// View is what the page shows.
type View struct {
	Headline       string
	Detail         string
	PairingURL     string // "" = no QR
	DirectURL      string // direct tunnel or loopback URL
	PublicHost     string
	PinCode        string // 4-digit visual PIN
	Paired         bool
	Problem        bool
	CurrentVersion string
	Update         updater.Status
}

type Server struct {
	ln           net.Listener
	srv          *http.Server
	secret       string
	view         func() View
	reset        func()
	applyUpdate  func() error
	manualUpdate func() error
	mu           sync.Mutex
	quit         func()
	checkUpdate  func(context.Context) (updater.Status, error)
	reloadDeck   func()
}

// SetQuitHandler registers the callback invoked when the user quits from the desktop interface.
func (s *Server) SetQuitHandler(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quit = fn
}

// SetReloadDeckHandler registers the callback invoked to reload the presentation notes and slides.
func (s *Server) SetReloadDeckHandler(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadDeck = fn
}

// SetCheckUpdateHandler registers the callback invoked to check for updates on demand.
func (s *Server) SetCheckUpdateHandler(fn func(context.Context) (updater.Status, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkUpdate = fn
}

func Start(view func() View, reset func(), applyUpdate ...func() error) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	var updateFn func() error
	if len(applyUpdate) > 0 {
		updateFn = applyUpdate[0]
	}
	var manualFn func() error
	if len(applyUpdate) > 1 {
		manualFn = applyUpdate[1]
	}
	s := &Server{ln: ln, secret: e2ee.B64.EncodeToString(e2ee.RandomBytes(24)), view: view, reset: reset, applyUpdate: updateFn, manualUpdate: manualFn}
	mux := http.NewServeMux()
	mux.HandleFunc("/"+s.secret+"/", s.page)
	mux.HandleFunc("/"+s.secret+"/reset", s.doReset)
	mux.HandleFunc("/"+s.secret+"/status", s.apiStatus)
	mux.HandleFunc("/"+s.secret+"/update/apply", s.doUpdateApply)
	mux.HandleFunc("/"+s.secret+"/update/manual", s.doUpdateManual)
	mux.HandleFunc("/"+s.secret+"/update/check", s.doUpdateCheck)
	mux.HandleFunc("/"+s.secret+"/deck/reload", s.doReloadDeck)
	mux.HandleFunc("/"+s.secret+"/quit", s.doQuit)
	s.srv = &http.Server{Handler: s.guard(mux), ReadHeaderTimeout: 5 * time.Second}
	go s.srv.Serve(ln)
	return s, nil
}

// URL is the page address (contains the path secret: open it only locally).
func (s *Server) URL() string { return fmt.Sprintf("http://%s/%s/", s.ln.Addr(), s.secret) }

func (s *Server) Close() { s.srv.Close() }

// guard blocks DNS-rebinding (Host must be the loopback address) and sets headers.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != s.ln.Addr().String() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src data:; form-action 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) doReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != "http://"+s.ln.Addr().String() && o != "null" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	s.reset()
	http.Redirect(w, r, "/"+s.secret+"/", http.StatusSeeOther)
}

func (s *Server) doUpdateApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != "http://"+s.ln.Addr().String() && o != "null" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var err error
	if s.applyUpdate != nil {
		err = s.applyUpdate()
	}
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) doUpdateManual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != "http://"+s.ln.Addr().String() && o != "null" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var err error
	if s.manualUpdate != nil {
		err = s.manualUpdate()
	}
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) doUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != "http://"+s.ln.Addr().String() && o != "null" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if s.checkUpdate == nil {
		http.Error(w, "not configured", http.StatusNotFound)
		return
	}
	st, err := s.checkUpdate(r.Context())
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":             true,
		"available":      st.Available,
		"latestVersion":  st.LatestVer,
		"currentVersion": st.CurrentVer,
		"releaseURL":     st.ReleaseURL,
		"releaseNotes":   st.ReleaseNotes,
	})
}

func (s *Server) doReloadDeck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != "http://"+s.ln.Addr().String() && o != "null" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	s.mu.Lock()
	fn := s.reloadDeck
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) doQuit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != "http://"+s.ln.Addr().String() && o != "null" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	s.mu.Lock()
	q := s.quit
	s.mu.Unlock()
	if q != nil {
		go func() {
			time.Sleep(150 * time.Millisecond)
			q()
		}()
	}
}

func (s *Server) apiStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	v := s.view()
	var qrSVG string
	if v.PairingURL != "" {
		if code, err := qr.Encode([]byte(v.PairingURL)); err == nil {
			qrSVG = code.SVG(8)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"headline":       v.Headline,
		"detail":         v.Detail,
		"pairingURL":     v.PairingURL,
		"directURL":      v.DirectURL,
		"publicHost":     v.PublicHost,
		"pinCode":        v.PinCode,
		"paired":         v.Paired,
		"problem":        v.Problem,
		"qrSVG":          qrSVG,
		"currentVersion": v.CurrentVersion,
		"update":         v.Update,
	})
}

func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/"+s.secret+"/" {
		http.NotFound(w, r)
		return
	}
	v := s.view()
	var qrHTML string
	if v.PairingURL != "" {
		if code, err := qr.Encode([]byte(v.PairingURL)); err == nil {
			qrHTML = `<div class="qr-box" id="qr-container">` + code.SVG(8) + `</div>`
		}
	}
	statusColor := "#10b981"
	if v.Problem {
		statusColor = "#f59e0b"
	}

	var b strings.Builder
	b.WriteString(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>DeckTap — Presentation Remote</title>
<style>
:root {
  --bg: #090d16;
  --panel: #0f172a;
  --card: #1e293b;
  --line: #334155;
  --text: #f8fafc;
  --muted: #94a3b8;
  --accent: #10b981;
  --cyan: #38bdf8;
  --warn: #f59e0b;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background: var(--bg);
  color: var(--text);
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  user-select: none;
}
.app-card {
  width: 100%;
  max-width: 440px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 24px;
  padding: 24px 22px;
  text-align: center;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.45);
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--line);
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 19px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}
.brand-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
}
.brand-version {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  background: rgba(255, 255, 255, 0.08);
  padding: 2px 7px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  letter-spacing: 0;
  vertical-align: middle;
}
.badge-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
  background: rgba(16, 185, 129, 0.15);
  color: var(--accent);
  border: 1px solid rgba(16, 185, 129, 0.3);
}
.badge-status.warn {
  background: rgba(245, 158, 11, 0.15);
  color: var(--warn);
  border-color: rgba(245, 158, 11, 0.3);
}
.badge-status.connected {
  background: rgba(56, 189, 248, 0.15);
  color: var(--cyan);
  border-color: rgba(56, 189, 248, 0.3);
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 6px currentColor;
}
.status-title {
  font-size: 16px;
  font-weight: 700;
  margin-bottom: 4px;
  color: #fff;
}
.status-sub {
  font-size: 13px;
  color: var(--muted);
  margin-bottom: 14px;
}
.qr-box {
  background: #ffffff;
  border-radius: 18px;
  padding: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3);
  margin: 4px 0 14px;
  border: 3px solid rgba(56, 189, 248, 0.2);
}
.qr-box svg {
  width: 210px;
  height: 210px;
  display: block;
}
.pin-card {
  background: var(--card);
  border: 1px solid var(--line);
  border-radius: 16px;
  padding: 12px 14px;
  margin-bottom: 14px;
  position: relative;
}
.pin-label {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--muted);
  margin-bottom: 4px;
}
.pin-digits {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 32px;
  font-weight: 800;
  letter-spacing: 8px;
  color: var(--cyan);
  text-shadow: 0 0 12px rgba(56, 189, 248, 0.4);
}
.connected-card {
  display: none;
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.3);
  border-radius: 16px;
  padding: 18px;
  margin-bottom: 14px;
  color: #fff;
}
.connected-card.active { display: block; }
.connected-icon { font-size: 36px; margin-bottom: 6px; }
.warning-pill {
  font-size: 11px;
  color: #fbbf24;
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.2);
  border-radius: 8px;
  padding: 6px 10px;
  margin-bottom: 14px;
}
.actions {
  display: flex;
  gap: 8px;
  justify-content: center;
}
.btn {
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  padding: 9px 14px;
  border-radius: 10px;
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--text);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  transition: all .15s ease;
  user-select: none;
}
.btn:hover { background: #334155; border-color: #475569; }
.btn:active { transform: scale(0.97); }
.btn-primary { background: var(--accent); color: #042f2e; border: none; font-weight: 700; }
.btn-primary:hover { background: #34d399; }
.btn-danger {
  background: rgba(239, 68, 68, 0.15);
  border-color: rgba(239, 68, 68, 0.35);
  color: #fca5a5;
}
.btn-danger:hover {
  background: rgba(239, 68, 68, 0.3);
  border-color: rgba(239, 68, 68, 0.6);
  color: #fff;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.btn-header-quit {
  font: inherit;
  font-size: 11px;
  font-weight: 700;
  padding: 4px 9px;
  border-radius: 999px;
  border: 1px solid rgba(239, 68, 68, 0.3);
  background: rgba(239, 68, 68, 0.12);
  color: #fca5a5;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  transition: all .15s ease;
  user-select: none;
}
.btn-header-quit:hover {
  background: #ef4444;
  color: #fff;
  border-color: #ef4444;
}
.tunnel-info {
  font-size: 11px;
  color: var(--muted);
  margin-top: 12px;
  word-break: break-all;
}
.footer-links {
  margin-top: 10px;
  font-size: 11px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex-wrap: wrap;
}
.footer-link {
  color: var(--cyan);
  text-decoration: none;
  opacity: 0.8;
  transition: opacity 0.15s ease;
}
.footer-link:hover {
  opacity: 1;
  text-decoration: underline;
}
.footer-sep {
  color: var(--muted);
  opacity: 0.4;
}
.footer-btn {
  background: none;
  border: none;
  color: var(--cyan);
  font-size: 11px;
  cursor: pointer;
  padding: 0;
  opacity: 0.8;
  font-family: inherit;
  transition: all 0.15s ease;
}
.footer-btn:hover {
  opacity: 1;
  text-decoration: underline;
}
.footer-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  text-decoration: none;
}
.update-toast {
  margin-top: 8px;
  font-size: 11px;
  font-weight: 500;
  padding: 4px 10px;
  border-radius: 6px;
  display: inline-block;
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
  border: 1px solid rgba(16, 185, 129, 0.3);
}
.update-toast.error {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
  border-color: rgba(239, 68, 68, 0.3);
}

/* Update Card */
.hidden { display: none !important; }
.update-card {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.14) 0%, rgba(56, 189, 248, 0.14) 100%);
  border: 1px solid rgba(52, 211, 153, 0.4);
  border-radius: 18px;
  padding: 14px 16px;
  margin-bottom: 16px;
  text-align: left;
  box-shadow: 0 6px 20px rgba(16, 185, 129, 0.15);
  animation: fadeInDown 0.25s ease;
}
@keyframes fadeInDown {
  from { opacity: 0; transform: translateY(-8px); }
  to { opacity: 1; transform: translateY(0); }
}
.update-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}
.update-pill {
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.08em;
  color: #34d399;
  background: rgba(16, 185, 129, 0.2);
  border: 1px solid rgba(52, 211, 153, 0.3);
  padding: 3px 8px;
  border-radius: 999px;
}
.btn-update-close {
  background: transparent;
  border: none;
  color: var(--muted);
  font-size: 15px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 6px;
  transition: color 0.15s;
}
.btn-update-close:hover { color: #fff; }
.update-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
  margin-bottom: 3px;
}
.update-sub {
  font-size: 12px;
  color: var(--muted);
  line-height: 1.4;
  margin-bottom: 12px;
}
.update-notes-box {
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 10px 12px;
  margin-bottom: 12px;
  max-height: 130px;
  overflow-y: auto;
  font-size: 12px;
  color: #cbd5e1;
  line-height: 1.45;
  white-space: pre-wrap;
  user-select: text;
}
.update-progress-box {
  margin: 10px 0 12px;
}
.update-progress-track {
  width: 100%;
  height: 6px;
  background: rgba(255, 255, 255, 0.12);
  border-radius: 999px;
  overflow: hidden;
  margin-bottom: 6px;
}
.update-progress-fill {
  height: 100%;
  width: 0%;
  background: linear-gradient(90deg, #10b981, #38bdf8);
  border-radius: 999px;
  transition: width 0.3s ease;
}
.update-progress-text {
  font-size: 11px;
  font-weight: 600;
  color: var(--cyan);
}
.update-btn-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.btn-ghost {
  background: transparent;
  border-color: transparent;
  color: var(--muted);
}
.btn-ghost:hover {
  background: rgba(255, 255, 255, 0.06);
  color: #fff;
}
</style>
</head>
<body>
<div class="app-card">
  <div class="header">
    <div class="brand">
      <svg class="brand-icon" viewBox="0 0 512 512">
        <rect width="512" height="512" rx="104" fill="#0f172a" stroke="#1e293b" stroke-width="8"/>
        <rect x="136" y="150" width="240" height="160" rx="18" fill="#1e293b" stroke="#38bdf8" stroke-width="6"/>
        <rect x="164" y="182" width="110" height="14" rx="7" fill="#f8fafc"/>
        <rect x="164" y="214" width="184" height="10" rx="5" fill="#64748b"/>
        <circle cx="376" cy="136" r="28" fill="#090d16" stroke="#10b981" stroke-width="5"/>
        <path d="M 368 136 L 373 141 L 385 129" fill="none" stroke="#10b981" stroke-width="6" stroke-linecap="round"/>
        <path d="M 110 360 C 180 420, 332 420, 402 360" fill="none" stroke="#10b981" stroke-width="14" stroke-linecap="round"/>
      </svg>
      DeckTap <span class="brand-version" id="brand-version">` + html.EscapeString(v.CurrentVersion) + `</span>
    </div>
    <div class="header-right">
      <div class="badge-status `)
	if v.Problem {
		b.WriteString("warn")
	} else if v.Paired {
		b.WriteString("connected")
	}
	b.WriteString(`" id="badge-status">
        <span class="dot"></span>
        <span id="badge-text">`)
	if v.Paired {
		b.WriteString("Phone Connected")
	} else if v.Problem {
		b.WriteString("Connecting…")
	} else {
		b.WriteString("Ready to Pair")
	}
	b.WriteString(`</span>
      </div>
      <button type="button" class="btn-header-quit" id="btn-header-quit" title="Quit DeckTap completely">⏻ Quit</button>
    </div>
  </div>

  <!-- Update Available Banner -->
  <div id="update-card" class="update-card hidden">
    <div class="update-top">
      <div class="update-pill">✨ NEW UPDATE AVAILABLE</div>
      <button type="button" class="btn-update-close" id="btn-update-dismiss" title="Dismiss">✕</button>
    </div>
    <div class="update-title" id="update-title">DeckTap update ready</div>
    <div class="update-sub" id="update-sub">A newer version is ready to install. Would you like to update?</div>
    
    <div id="update-progress-box" class="update-progress-box hidden">
      <div class="update-progress-track">
        <div class="update-progress-fill" id="update-progress-fill"></div>
      </div>
      <div class="update-progress-text" id="update-progress-text">Downloading update…</div>
    </div>

    <div id="update-notes-box" class="update-notes-box hidden">
      <div class="update-notes-content" id="update-notes-content"></div>
    </div>

    <div class="update-btn-row" id="update-btn-row">
      <button type="button" class="btn btn-primary" id="btn-update-now">🚀 Update Now</button>
      <button type="button" class="btn" id="btn-update-notes">Release Notes</button>
      <button type="button" class="btn hidden" id="btn-update-manual">Manual Download</button>
      <button type="button" class="btn btn-ghost" id="btn-update-later">Later</button>
    </div>
  </div>

  <div id="unpaired-section" `)
	if v.Paired {
		b.WriteString(`style="display:none;"`)
	}
	b.WriteString(`>
    <div class="status-title" id="st-headline">` + html.EscapeString(v.Headline) + `</div>
    <div class="status-sub" id="st-detail">` + html.EscapeString(v.Detail) + `</div>

    ` + qrHTML + `

    <div class="pin-card" id="pin-card" `)
	if v.PinCode == "" {
		b.WriteString(`style="display:none;"`)
	}
	b.WriteString(`>
      <div class="pin-label">4-Digit Security PIN</div>
      <div class="pin-digits" id="pin-digits">` + html.EscapeString(v.PinCode) + `</div>
    </div>

    <div class="warning-pill">
      🔒 Presenter screen only. Do not display this window on the audience projector.
    </div>
  </div>

  <div class="connected-card `)
	if v.Paired {
		b.WriteString("active")
	}
	b.WriteString(`" id="connected-card">
    <div class="connected-icon">📱</div>
    <div class="status-title" id="conn-title">Presenter Connected</div>
    <div class="status-sub" id="conn-sub">Your phone is paired and controlling the presentation.</div>
  </div>

  <div class="actions">
    <form method="post" action="/` + s.secret + `/reset" style="display:inline;">
      <button type="submit" class="btn" title="Create a fresh pairing code and disconnect previous devices">🔄 New Code</button>
    </form>
    <button type="button" class="btn" id="btn-reload-deck" onclick="reloadDeckNow()" title="Reload slide titles and speaker notes from PowerPoint">↻ Reload Notes</button>
    <button type="button" class="btn" id="btn-copy-link">📋 Copy Link</button>
    <button type="button" class="btn btn-danger" id="btn-quit" title="Quit DeckTap completely">⏻ Quit</button>
  </div>

  <div class="tunnel-info" id="tunnel-info">`)
	if v.PublicHost != "" {
		b.WriteString("Web App: silva324.github.io/DeckTap · Host: " + html.EscapeString(v.PublicHost))
	} else {
		b.WriteString("Web App: silva324.github.io/DeckTap")
	}
	b.WriteString(`</div>
  <div class="footer-links">
    <a href="https://github.com/silva324/DeckTap" target="_blank" rel="noopener" class="footer-link" id="footer-version-link">DeckTap ` + html.EscapeString(v.CurrentVersion) + ` · GitHub Repository</a>
    <span class="footer-sep">·</span>
    <button type="button" class="footer-btn" id="btn-footer-reload-deck" onclick="reloadDeckNow()">Reload notes</button>
    <span class="footer-sep">·</span>
    <button type="button" class="footer-btn" id="btn-check-update" onclick="checkUpdateNow()">Check for updates</button>
  </div>
  <div id="update-toast" class="update-toast hidden"></div>
</div>

<script>
let lastPairingURL = ` + fmt.Sprintf("%q", v.PairingURL) + `;

async function refreshStatus() {
  try {
    const res = await fetch("status");
    if (!res.ok) return;
    const data = await res.json();
    
    // Update badge
    const badge = document.getElementById("badge-status");
    const badgeTxt = document.getElementById("badge-text");
    const unpairedSec = document.getElementById("unpaired-section");
    const connCard = document.getElementById("connected-card");

    if (data.paired) {
      badge.className = "badge-status connected";
      badgeTxt.textContent = "Phone Connected";
      unpairedSec.style.display = "none";
      connCard.classList.add("active");
      document.getElementById("conn-sub").textContent = data.detail || "Controlling presentation.";
    } else {
      connCard.classList.remove("active");
      unpairedSec.style.display = "block";
      if (data.problem) {
        badge.className = "badge-status warn";
        badgeTxt.textContent = "Connecting…";
      } else {
        badge.className = "badge-status";
        badgeTxt.textContent = "Ready to Pair";
      }
    }

    document.getElementById("st-headline").textContent = data.headline || "";
    document.getElementById("st-detail").textContent = data.detail || "";
    
    // Update PIN
    const pinEl = document.getElementById("pin-digits");
    const pinCard = document.getElementById("pin-card");
    if (data.pinCode) {
      pinEl.textContent = data.pinCode;
      pinCard.style.display = "block";
    } else {
      pinCard.style.display = "none";
    }

    // Update QR code if changed
    if (data.pairingURL && data.pairingURL !== lastPairingURL && data.qrSVG) {
      lastPairingURL = data.pairingURL;
      const qrCont = document.getElementById("qr-container");
      if (qrCont) qrCont.innerHTML = data.qrSVG;
    }

    if (data.publicHost) {
      document.getElementById("tunnel-info").textContent = "Tunnel: " + data.publicHost;
    }

    if (data.currentVersion) {
      const bv = document.getElementById("brand-version");
      if (bv) bv.textContent = data.currentVersion;
      const fv = document.getElementById("footer-version-link");
      if (fv) fv.textContent = "DeckTap " + data.currentVersion + " · GitHub Repository";
    }

    // Handle update banner
    if (data.update) {
      renderUpdate(data.update);
    }
  } catch {}
}

let activeUpdate = null;
let updateDismissed = false;

function renderUpdate(u) {
  const card = document.getElementById("update-card");
  if (!card) return;
  if (!u || !u.available) {
    card.classList.add("hidden");
    return;
  }
  const isBusy = (u.state === "downloading" || u.state === "verifying" || u.state === "applying" || u.state === "restarting");
  if (!isBusy && (updateDismissed || sessionStorage.getItem("dt_update_dismissed_" + u.latestVersion) === "1")) {
    card.classList.add("hidden");
    return;
  }

  activeUpdate = u;
  card.classList.remove("hidden");
  document.getElementById("update-title").textContent = (u.latestVersion || "New version") + " is available!";
  const sub = document.getElementById("update-sub");
  const progressBox = document.getElementById("update-progress-box");
  const progressFill = document.getElementById("update-progress-fill");
  const progressText = document.getElementById("update-progress-text");
  const btnNow = document.getElementById("btn-update-now");
  const btnManual = document.getElementById("btn-update-manual");
  const btnLater = document.getElementById("btn-update-later");
  const btnDismiss = document.getElementById("btn-update-dismiss");

  if (isBusy) {
    progressBox.classList.remove("hidden");
    btnNow.disabled = true;
    btnLater.classList.add("hidden");
    btnDismiss.classList.add("hidden");
    btnManual.classList.add("hidden");

    if (u.state === "downloading") {
      const pct = u.progress || 0;
      progressFill.style.width = pct + "%";
      progressFill.style.background = "linear-gradient(90deg, #10b981, #38bdf8)";
      progressText.textContent = "Downloading update… " + pct + "%";
      btnNow.textContent = "⏳ Downloading (" + pct + "%)…";
      sub.textContent = "Downloading the latest version in the background…";
    } else if (u.state === "verifying") {
      progressFill.style.width = "75%";
      progressText.textContent = "Verifying cryptographic checksum…";
      btnNow.textContent = "⏳ Verifying…";
      sub.textContent = "Verifying package integrity…";
    } else if (u.state === "applying") {
      progressFill.style.width = "90%";
      progressText.textContent = "Installing update files…";
      btnNow.textContent = "⏳ Installing…";
      sub.textContent = "Replacing executable atomically…";
    } else if (u.state === "restarting") {
      progressFill.style.width = "100%";
      progressText.textContent = "✨ Restarting DeckTap…";
      btnNow.textContent = "Restarting…";
      sub.textContent = "Update complete! DeckTap is restarting now…";
    }
  } else if (u.state === "error") {
    progressBox.classList.remove("hidden");
    progressFill.style.width = "100%";
    progressFill.style.background = "#ef4444";
    progressText.textContent = "⚠️ " + (u.error || "Update encountered an error");
    btnNow.disabled = false;
    btnNow.textContent = "🔄 Retry Update";
    btnManual.classList.remove("hidden");
    btnLater.classList.remove("hidden");
    btnDismiss.classList.remove("hidden");
    sub.textContent = "An error occurred while updating automatically. You can retry or download manually.";
  } else {
    // idle
    progressBox.classList.add("hidden");
    btnNow.disabled = false;
    btnNow.textContent = "🚀 Update Now";
    btnManual.classList.add("hidden");
    btnLater.classList.remove("hidden");
    btnDismiss.classList.remove("hidden");
    if (u.assetSize > 0) {
      const mb = (u.assetSize / (1024 * 1024)).toFixed(1);
      sub.textContent = "A newer version is ready to install (" + mb + " MB). Click Update Now to install automatically.";
    } else {
      sub.textContent = "A newer version is ready to install. Click Update Now to install automatically.";
    }
  }

  const notesContent = document.getElementById("update-notes-content");
  if (notesContent) {
    notesContent.textContent = u.releaseNotes || "Performance improvements and bug fixes.";
  }
}

function dismissUpdate() {
  updateDismissed = true;
  if (activeUpdate && activeUpdate.latestVersion) {
    sessionStorage.setItem("dt_update_dismissed_" + activeUpdate.latestVersion, "1");
  }
  const card = document.getElementById("update-card");
  if (card) card.classList.add("hidden");
}

document.getElementById("btn-update-dismiss").addEventListener("click", dismissUpdate);
document.getElementById("btn-update-later").addEventListener("click", dismissUpdate);

document.getElementById("btn-update-notes").addEventListener("click", () => {
  const box = document.getElementById("update-notes-box");
  if (box) box.classList.toggle("hidden");
});

document.getElementById("btn-update-now").addEventListener("click", async () => {
  const btn = document.getElementById("btn-update-now");
  btn.disabled = true;
  btn.textContent = "⏳ Starting Update…";
  try {
    const res = await fetch("update/apply", { method: "POST" });
    const data = await res.json();
    if (!data.ok && data.error) {
      alert("Could not start update: " + data.error);
      btn.disabled = false;
      btn.textContent = "🚀 Update Now";
    }
  } catch (e) {
    btn.disabled = false;
    btn.textContent = "🚀 Update Now";
  }
  refreshStatus();
});

document.getElementById("btn-update-manual").addEventListener("click", async () => {
  try {
    await fetch("update/manual", { method: "POST" });
  } catch {}
});

setInterval(refreshStatus, 1500);

document.getElementById("btn-copy-link").addEventListener("click", () => {
  if (lastPairingURL) {
    navigator.clipboard.writeText(lastPairingURL).then(() => {
      const b = document.getElementById("btn-copy-link");
      const old = b.textContent;
      b.textContent = "✓ Copied!";
      setTimeout(() => b.textContent = old, 1800);
    });
  }
});

async function reloadDeckNow() {
  const btns = [document.getElementById("btn-reload-deck"), document.getElementById("btn-footer-reload-deck")].filter(Boolean);
  btns.forEach(b => { b.disabled = true; });
  try {
    const res = await fetch("deck/reload", { method: "POST" });
    if (res.ok) {
      showUpdateToast("✓ Speaker notes & slides reloaded from PowerPoint");
    } else {
      showUpdateToast("Failed to reload deck", true);
    }
  } catch (e) {
    showUpdateToast("Error communicating with DeckTap", true);
  } finally {
    btns.forEach(b => { b.disabled = false; });
  }
}

async function checkUpdateNow() {
  const btn = document.getElementById("btn-check-update");
  if (!btn) return;
  btn.disabled = true;
  const originalText = "Check for updates";
  btn.textContent = "Checking…";

  try {
    const res = await fetch("update/check", { method: "POST" });
    const data = await res.json();
    if (!data.ok) {
      showUpdateToast("Check failed: " + (data.error || "error"), true);
      btn.textContent = originalText;
      btn.disabled = false;
      return;
    }
    if (data.available) {
      renderUpdate({
        available: true,
        latestVersion: data.latestVersion,
        releaseURL: data.releaseURL,
        releaseNotes: data.releaseNotes,
        state: "idle"
      });
      showUpdateToast("✨ Update available: " + data.latestVersion);
      btn.textContent = "Update available (" + data.latestVersion + ")";
      btn.disabled = false;
    } else {
      showUpdateToast("✓ DeckTap is up to date (" + (data.currentVersion || "latest") + ")");
      btn.textContent = originalText;
      btn.disabled = false;
    }
  } catch (err) {
    showUpdateToast("Network error checking for updates", true);
    btn.textContent = originalText;
    btn.disabled = false;
  }
}

function showUpdateToast(msg, isErr) {
  const toast = document.getElementById("update-toast");
  if (!toast) return;
  toast.textContent = msg;
  toast.className = "update-toast" + (isErr ? " error" : "");
  setTimeout(() => {
    toast.classList.add("hidden");
  }, 4000);
}

async function quitDeckTap() {
  if (!confirm("Are you sure you want to quit DeckTap?")) return;
  const btns = [document.getElementById("btn-quit"), document.getElementById("btn-header-quit")];
  btns.forEach(b => { if (b) { b.disabled = true; b.textContent = "⏳ Quitting…"; } });
  try {
    await fetch("quit", { method: "POST" });
  } catch {}
  document.body.innerHTML = '<div style="text-align:center;padding:50px 20px;font-family:-apple-system,BlinkMacSystemFont,sans-serif;color:#94a3b8;">' +
    '<div style="font-size:44px;margin-bottom:12px;">👋</div>' +
    '<h2 style="color:#fff;margin-bottom:8px;font-size:20px;">DeckTap has shut down</h2>' +
    '<p style="font-size:14px;color:#64748b;">The application has exited cleanly. You may close this window.</p>' +
    '</div>';
}
const bQuit = document.getElementById("btn-quit");
if (bQuit) bQuit.addEventListener("click", quitDeckTap);
const bHQuit = document.getElementById("btn-header-quit");
if (bHQuit) bHQuit.addEventListener("click", quitDeckTap);
</script>
</body>
</html>`)

	_ = statusColor // reserved for dynamic themes
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(b.String()))
}
