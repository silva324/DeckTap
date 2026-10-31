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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/silva324/DeckTap/internal/e2ee"
	"github.com/silva324/DeckTap/internal/qr"
	"github.com/silva324/DeckTap/internal/updater"
	"github.com/silva324/DeckTap/internal/webui"
)

// View is what the page shows.
type View struct {
	Mode           string
	Headline       string
	Detail         string
	PairingURL     string // "" = no QR
	DirectURL      string // direct tunnel or loopback URL
	PublicHost     string
	LANIP          string
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
	modeHandler  func(string) error
}

// SetModeHandler registers the callback invoked to change the network mode (e.g. tunnel vs lan).
func (s *Server) SetModeHandler(fn func(string) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modeHandler = fn
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
	mux.HandleFunc("/"+s.secret+"/mode", s.doSetMode)
	mux.HandleFunc("/"+s.secret+"/status", s.apiStatus)
	mux.HandleFunc("/"+s.secret+"/update/apply", s.doUpdateApply)
	mux.HandleFunc("/"+s.secret+"/update/manual", s.doUpdateManual)
	mux.HandleFunc("/"+s.secret+"/update/check", s.doUpdateCheck)
	mux.HandleFunc("/"+s.secret+"/deck/reload", s.doReloadDeck)
	mux.HandleFunc("/"+s.secret+"/quit", s.doQuit)
	mux.HandleFunc("/"+s.secret+"/fonts/", s.font)
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
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src data:; font-src 'self'; form-action 'self'; frame-ancestors 'none'")
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

func (s *Server) doSetMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != "http://"+s.ln.Addr().String() && o != "null" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Mode == "" {
		req.Mode = r.FormValue("mode")
	}
	s.mu.Lock()
	handler := s.modeHandler
	s.mu.Unlock()

	if handler != nil && (req.Mode == "tunnel" || req.Mode == "lan" || req.Mode == "local") {
		if err := handler(req.Mode); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "mode": req.Mode})
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
		"mode":           v.Mode,
		"headline":       v.Headline,
		"detail":         v.Detail,
		"pairingURL":     v.PairingURL,
		"directURL":      v.DirectURL,
		"publicHost":     v.PublicHost,
		"lanIP":          v.LANIP,
		"pinCode":        v.PinCode,
		"paired":         v.Paired,
		"problem":        v.Problem,
		"qrSVG":          qrSVG,
		"currentVersion": v.CurrentVersion,
		"update":         v.Update,
	})
}

// font serves the embedded web fonts, so the page loads Volaro's typefaces without a CDN.
func (s *Server) font(w http.ResponseWriter, r *http.Request) {
	b, ok := webui.Font(strings.TrimPrefix(r.URL.Path, "/"+s.secret+"/fonts/"))
	if !ok || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "font/woff2")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Write(b)
}

// iconSVGBase64 is the shared app icon, inlined as a data URI (the page CSP allows data: images).
var iconSVGBase64 = base64.StdEncoding.EncodeToString(webui.IconSVG())

// connectionLine describes how the phone connects, for the window footer.
func connectionLine(v View) string {
	switch v.Mode {
	case "lan":
		return "Same Wi-Fi · " + v.LANIP
	case "tunnel":
		if v.PublicHost != "" {
			return "Internet · " + v.PublicHost
		}
		return "Internet"
	}
	return "Test mode · this computer only"
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
	badgeClass, badgeText := "", "Ready to pair"
	switch {
	case v.Paired:
		badgeClass, badgeText = "ok", "Phone connected"
	case v.Problem:
		badgeClass, badgeText = "warn", "Needs attention"
	}
	modeTab := func(m string) string {
		if v.Mode == m {
			return " active"
		}
		return ""
	}
	hideIf := func(hide bool) string {
		if hide {
			return ` hidden`
		}
		return ""
	}

	var b strings.Builder
	b.WriteString(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="light dark">
<title>DeckTap</title>
<style>
@font-face { font-family: "Outfit"; font-style: normal; font-weight: 300 500; font-display: swap; src: url("fonts/outfit-latin.woff2") format("woff2"); }
@font-face { font-family: "DM Mono"; font-style: normal; font-weight: 400; font-display: swap; src: url("fonts/dm-mono-400-latin.woff2") format("woff2"); }
/* Volaro design tokens (design-system/tokens.json); Dawn is DeckTap's accent. */
:root {
  --bg: #f7f9fc; --panel: #ffffff; --sunken: #eef2f8; --line: #dfe6f1; --line-strong: #7d8da4;
  --text: #0e1a2b; --muted: #4a5a70; --accent: #ff8a6b; --accent-ink: #0e1a2b; --accent-soft: #ffe4db;
  --accent-text: #b9431f; --focus: #2557e6; --success: #1a7350; --warning: #946000; --danger: #c4352b;
  --font: "Outfit", system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  --mono: "DM Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  --ease: cubic-bezier(.2, .8, .2, 1);
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #0b1422; --panel: #111d2e; --sunken: #08101b; --line: #22324a; --line-strong: #5f7596;
    --text: #f2f5fa; --muted: #a9b8cc; --accent: #ff9b80; --accent-ink: #0b1422; --accent-soft: #3a2420;
    --accent-text: #ffaa91; --focus: #8fb2ff; --success: #5bd39a; --warning: #f2b84b; --danger: #ff8a80;
  }
}
* { box-sizing: border-box; margin: 0; padding: 0; }
html, body { height: 100%; }
body { font: 14px/1.5 var(--font); background: var(--bg); color: var(--text); user-select: none;
  display: flex; flex-direction: column; padding: 16px 20px 14px; }
button { font: inherit; color: inherit; cursor: pointer; }
:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
.hidden { display: none !important; }
.ic { width: 18px; height: 18px; flex-shrink: 0; fill: none; stroke: currentColor; stroke-width: 1.5; stroke-linecap: round; stroke-linejoin: round; }
.eyebrow { font-family: var(--mono); font-size: 11px; letter-spacing: .08em; text-transform: uppercase; color: var(--muted); }

.header { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
.brand { display: flex; align-items: center; gap: 9px; font-size: 17px; font-weight: 500; letter-spacing: -0.01em; margin-right: auto; }
.brand img { width: 28px; height: 28px; border-radius: 7px; }
.status { display: inline-flex; align-items: center; gap: 7px; padding: 4px 11px; border-radius: 999px;
  font-size: 12px; font-weight: 500; border: 1px solid var(--line); background: var(--panel); color: var(--muted); }
.status .dot { width: 7px; height: 7px; border-radius: 50%; background: var(--line-strong); }
.status.ok { color: var(--success); } .status.ok .dot { background: var(--success); }
.status.warn { color: var(--warning); } .status.warn .dot { background: var(--warning); }
.icon-btn { width: 32px; height: 32px; border-radius: 999px; border: 0; background: transparent; color: var(--muted);
  display: inline-flex; align-items: center; justify-content: center; }
.icon-btn:hover { background: var(--sunken); color: var(--text); }

.menu-wrap { position: relative; }
.menu { position: absolute; right: 0; top: 38px; z-index: 10; min-width: 210px; padding: 6px; border-radius: 16px;
  background: var(--panel); border: 1px solid var(--line); box-shadow: 0 12px 32px rgba(5, 16, 29, .18); }
.menu button { width: 100%; display: flex; align-items: center; gap: 10px; padding: 9px 10px; border: 0; border-radius: 10px;
  background: transparent; text-align: left; font-size: 13px; }
.menu button:hover { background: var(--sunken); }
.menu button:disabled { opacity: .5; cursor: default; }
.menu .danger { color: var(--danger); }
.menu hr { border: 0; border-top: 1px solid var(--line); margin: 6px 4px; }
.menu .meta { padding: 6px 10px 4px; font-size: 11px; color: var(--muted); }

.modes { display: flex; padding: 3px; gap: 3px; border-radius: 999px; background: var(--sunken); border: 1px solid var(--line); margin-bottom: 16px; }
.mode { flex: 1; display: inline-flex; align-items: center; justify-content: center; gap: 7px; padding: 7px 10px;
  border: 0; border-radius: 999px; background: transparent; color: var(--muted); font-size: 13px; font-weight: 500;
  transition: background .2s var(--ease), color .2s var(--ease); }
.mode:hover { color: var(--text); }
.mode.active { background: var(--panel); color: var(--text); box-shadow: 0 1px 3px rgba(5, 16, 29, .12); }

.main { flex: 1; display: flex; flex-direction: column; align-items: center; text-align: center; min-height: 0; }
.headline { font-size: 21px; font-weight: 300; letter-spacing: -0.015em; }
.detail { font-size: 13px; color: var(--muted); margin-top: 2px; }
.qr-box { background: #ffffff; border-radius: 24px; padding: 14px; margin: 16px 0 14px; border: 1px solid var(--line); line-height: 0; }
.qr-box svg { width: 196px; height: 196px; display: block; }
.pin { display: flex; flex-direction: column; align-items: center; gap: 2px; }
.pin-digits { font-size: 40px; font-weight: 300; letter-spacing: .22em; margin-right: -.22em; color: var(--accent-text); font-variant-numeric: tabular-nums; }
.privacy { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; color: var(--muted); margin-top: 10px; }
.privacy .ic { width: 14px; height: 14px; }

.connected { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; width: 100%; }
.connected .ring { width: 72px; height: 72px; border-radius: 50%; background: var(--accent-soft); color: var(--accent-text);
  display: flex; align-items: center; justify-content: center; }
.connected .ring .ic { width: 32px; height: 32px; }

.actions { display: flex; gap: 8px; justify-content: center; margin-top: 14px; }
.btn { display: inline-flex; align-items: center; justify-content: center; gap: 7px; padding: 9px 18px; border-radius: 999px;
  border: 1px solid var(--line-strong); background: var(--panel); font-size: 13px; font-weight: 500;
  transition: transform .2s var(--ease), background .2s var(--ease); }
.btn:hover { background: var(--sunken); }
.btn:active { transform: scale(.98); }
.btn:disabled { opacity: .5; cursor: default; }
.btn.primary { background: var(--accent); border-color: var(--accent); color: var(--accent-ink); }
.btn.primary:hover { filter: brightness(.96); }
.btn.ghost { border-color: transparent; background: transparent; color: var(--muted); }
.btn.ghost:hover { color: var(--text); background: var(--sunken); }

.footer { display: flex; justify-content: space-between; gap: 10px; margin-top: 12px; padding-top: 10px; border-top: 1px solid var(--line);
  font-size: 11px; color: var(--muted); }
.footer span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.toast { position: fixed; left: 50%; bottom: 48px; transform: translateX(-50%); padding: 8px 14px; border-radius: 999px;
  background: var(--text); color: var(--bg); font-size: 12px; font-weight: 500; white-space: nowrap; }
.toast.error { background: var(--danger); color: #fff; }

.update { border: 1px solid var(--line); background: var(--panel); border-radius: 16px; padding: 12px 14px; margin-bottom: 14px; text-align: left; }
.update-top { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.update-title { font-size: 14px; font-weight: 500; }
.update-sub { font-size: 12px; color: var(--muted); margin: 2px 0 10px; }
.update-notes { background: var(--sunken); border-radius: 8px; padding: 8px 10px; margin-bottom: 10px; max-height: 110px;
  overflow-y: auto; font-size: 12px; color: var(--muted); white-space: pre-wrap; user-select: text; }
.track { height: 4px; border-radius: 999px; background: var(--sunken); overflow: hidden; margin-bottom: 6px; }
.fill { height: 100%; width: 0; background: var(--accent); border-radius: 999px; transition: width .3s var(--ease); }
.fill.error { background: var(--danger); }
.progress-text { font-size: 11px; color: var(--muted); margin-bottom: 10px; }
.update-row { display: flex; gap: 6px; flex-wrap: wrap; }
.update-row .btn { padding: 7px 14px; }
@media (prefers-reduced-motion: reduce) { * { transition: none !important; } }
</style>
</head>
<body>
<svg width="0" height="0" style="position:absolute" aria-hidden="true"><defs>
  <symbol id="i-globe" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20M2 12h20"/></symbol>
  <symbol id="i-wifi" viewBox="0 0 24 24"><path d="M12 20h.01M2 8.82a15 15 0 0 1 20 0M5 12.86a10 10 0 0 1 14 0M8.5 16.43a5 5 0 0 1 7 0"/></symbol>
  <symbol id="i-more" viewBox="0 0 24 24"><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/></symbol>
  <symbol id="i-rotate" viewBox="0 0 24 24"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/></symbol>
  <symbol id="i-copy" viewBox="0 0 24 24"><rect width="14" height="14" x="8" y="8" rx="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></symbol>
  <symbol id="i-refresh" viewBox="0 0 24 24"><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/></symbol>
  <symbol id="i-download" viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="m7 10 5 5 5-5M12 15V3"/></symbol>
  <symbol id="i-power" viewBox="0 0 24 24"><path d="M12 2v10M18.4 6.6a9 9 0 1 1-12.77.04"/></symbol>
  <symbol id="i-eye-off" viewBox="0 0 24 24"><path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68M6.61 6.61A13.53 13.53 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61M2 2l20 20M14.12 14.12a3 3 0 1 1-4.24-4.24"/></symbol>
  <symbol id="i-smartphone" viewBox="0 0 24 24"><rect width="14" height="20" x="5" y="2" rx="2"/><path d="M12 18h.01"/></symbol>
  <symbol id="i-x" viewBox="0 0 24 24"><path d="M18 6 6 18M6 6l12 12"/></symbol>
</defs></svg>

<header class="header">
  <div class="brand"><img src="` + "data:image/svg+xml;base64," + iconSVGBase64 + `" alt="">DeckTap</div>
  <div class="status ` + badgeClass + `" id="badge-status"><span class="dot"></span><span id="badge-text">` + badgeText + `</span></div>
  <div class="menu-wrap">
    <button type="button" class="icon-btn" id="btn-more" aria-label="More" aria-haspopup="menu" aria-expanded="false"><svg class="ic"><use href="#i-more"/></svg></button>
    <div class="menu hidden" id="menu" role="menu">
      <button type="button" role="menuitem" id="btn-reload-deck"><svg class="ic"><use href="#i-refresh"/></svg>Reload notes</button>
      <button type="button" role="menuitem" id="btn-check-update"><svg class="ic"><use href="#i-download"/></svg><span>Check for updates</span></button>
      <hr>
      <button type="button" role="menuitem" class="danger" id="btn-quit"><svg class="ic"><use href="#i-power"/></svg>Quit DeckTap</button>
      <div class="meta" id="footer-version-link">DeckTap by Volaro · ` + html.EscapeString(v.CurrentVersion) + `</div>
    </div>
  </div>
</header>

<div id="update-card" class="update hidden">
  <div class="update-top">
    <div class="update-title" id="update-title">Update available</div>
    <button type="button" class="icon-btn" id="btn-update-dismiss" aria-label="Dismiss"><svg class="ic"><use href="#i-x"/></svg></button>
  </div>
  <div class="update-sub" id="update-sub">A newer version is ready to install.</div>
  <div id="update-progress-box" class="hidden">
    <div class="track"><div class="fill" id="update-progress-fill"></div></div>
    <div class="progress-text" id="update-progress-text">Downloading…</div>
  </div>
  <div id="update-notes-box" class="update-notes hidden"><div id="update-notes-content"></div></div>
  <div class="update-row" id="update-btn-row">
    <button type="button" class="btn primary" id="btn-update-now">Update now</button>
    <button type="button" class="btn" id="btn-update-notes">What's new</button>
    <button type="button" class="btn hidden" id="btn-update-manual">Download manually</button>
    <button type="button" class="btn ghost" id="btn-update-later">Later</button>
  </div>
</div>

<div class="modes" id="mode-switcher" role="radiogroup" aria-label="How your phone connects">
  <button type="button" class="mode` + modeTab("tunnel") + `" id="tab-tunnel" role="radio" onclick="switchMode('tunnel')" title="Works on any network, including mobile data"><svg class="ic"><use href="#i-globe"/></svg>Internet</button>
  <button type="button" class="mode` + modeTab("lan") + `" id="tab-lan" role="radio" onclick="switchMode('lan')" title="Phone and computer on the same Wi-Fi. No internet needed."><svg class="ic"><use href="#i-wifi"/></svg>Same Wi-Fi</button>
</div>

<main class="main">
  <section id="unpaired-section" class="main` + hideIf(v.Paired) + `">
    <div class="headline" id="st-headline">` + html.EscapeString(v.Headline) + `</div>
    <div class="detail" id="st-detail">` + html.EscapeString(v.Detail) + `</div>
    ` + qrHTML + `
    <div class="pin` + hideIf(v.PinCode == "") + `" id="pin-card">
      <div class="eyebrow">PIN</div>
      <div class="pin-digits" id="pin-digits">` + html.EscapeString(v.PinCode) + `</div>
    </div>
    <div class="privacy"><svg class="ic"><use href="#i-eye-off"/></svg>Keep this window off the projector</div>
  </section>

  <section class="connected` + hideIf(!v.Paired) + `" id="connected-card">
    <div class="ring"><svg class="ic"><use href="#i-smartphone"/></svg></div>
    <div class="headline" id="conn-title">Your phone is in control</div>
    <div class="detail" id="conn-sub">` + html.EscapeString(v.Detail) + `</div>
  </section>

  <div class="actions">
    <form method="post" action="/` + s.secret + `/reset">
      <button type="submit" class="btn" id="btn-new-code" title="Make a new code and disconnect any phone"><svg class="ic"><use href="#i-rotate"/></svg><span id="new-code-label">` + map[bool]string{true: "Disconnect phone", false: "New code"}[v.Paired] + `</span></button>
    </form>
    <button type="button" class="btn ghost` + hideIf(v.Paired) + `" id="btn-copy-link"><svg class="ic"><use href="#i-copy"/></svg><span>Copy link</span></button>
  </div>
</main>

<footer class="footer">
  <span id="tunnel-info">` + html.EscapeString(connectionLine(v)) + `</span>
  <span>DeckTap by Volaro</span>
</footer>
<div id="update-toast" class="toast hidden" role="status"></div>

<script>
let lastPairingURL = ` + fmt.Sprintf("%q", v.PairingURL) + `;
const $ = (id) => document.getElementById(id);
const show = (el, on) => el && el.classList.toggle("hidden", !on);

const menu = $("menu"), moreBtn = $("btn-more");
function setMenu(open) { show(menu, open); moreBtn.setAttribute("aria-expanded", open ? "true" : "false"); }
moreBtn.addEventListener("click", (e) => { e.stopPropagation(); setMenu(menu.classList.contains("hidden")); });
document.addEventListener("click", (e) => { if (!menu.contains(e.target)) setMenu(false); });
document.addEventListener("keydown", (e) => { if (e.key === "Escape") setMenu(false); });

async function switchMode(mode) {
  try {
    const res = await fetch("mode", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ mode }) });
    if (res.ok) { updateModeTabs(mode); refreshStatus(); }
  } catch (e) { console.error("switchMode failed", e); }
}

// Only the real mode is highlighted; local test mode highlights neither.
function updateModeTabs(mode) {
  for (const [id, m] of [["tab-tunnel", "tunnel"], ["tab-lan", "lan"]]) {
    const tab = $(id);
    tab.classList.toggle("active", mode === m);
    tab.setAttribute("aria-checked", mode === m ? "true" : "false");
  }
}

function connectionLine(d) {
  if (d.mode === "lan") return "Same Wi-Fi · " + (d.lanIP || "local network");
  if (d.mode === "tunnel") return d.publicHost ? "Internet · " + d.publicHost : "Internet";
  return "Test mode · this computer only";
}

async function refreshStatus() {
  try {
    const res = await fetch("status");
    if (!res.ok) return;
    const data = await res.json();
    updateModeTabs(data.mode);

    const badge = $("badge-status"), badgeTxt = $("badge-text");
    if (data.paired) { badge.className = "status ok"; badgeTxt.textContent = "Phone connected"; }
    else if (data.problem) { badge.className = "status warn"; badgeTxt.textContent = "Needs attention"; }
    else { badge.className = "status"; badgeTxt.textContent = "Ready to pair"; }

    show($("unpaired-section"), !data.paired);
    show($("connected-card"), !!data.paired);
    show($("btn-copy-link"), !data.paired);
    $("new-code-label").textContent = data.paired ? "Disconnect phone" : "New code";
    $("conn-sub").textContent = data.detail || "";
    $("st-headline").textContent = data.headline || "";
    $("st-detail").textContent = data.detail || "";

    show($("pin-card"), !!data.pinCode);
    if (data.pinCode) $("pin-digits").textContent = data.pinCode;

    if (data.pairingURL && data.pairingURL !== lastPairingURL && data.qrSVG) {
      lastPairingURL = data.pairingURL;
      const qr = $("qr-container");
      if (qr) qr.innerHTML = data.qrSVG;
    }
    $("tunnel-info").textContent = connectionLine(data);
    if (data.currentVersion) $("footer-version-link").textContent = "DeckTap by Volaro · " + data.currentVersion;
    if (data.update) renderUpdate(data.update);
  } catch {}
}

let activeUpdate = null, updateDismissed = false;
function renderUpdate(u) {
  const card = $("update-card");
  if (!u || !u.available) { show(card, false); return; }
  const busy = ["downloading", "verifying", "applying", "restarting"].includes(u.state);
  if (!busy && (updateDismissed || sessionStorage.getItem("dt_update_dismissed_" + u.latestVersion) === "1")) { show(card, false); return; }

  activeUpdate = u;
  show(card, true);
  $("update-title").textContent = "DeckTap " + (u.latestVersion || "update") + " is available";
  const sub = $("update-sub"), fill = $("update-progress-fill"), ptxt = $("update-progress-text"), now = $("btn-update-now");
  fill.classList.toggle("error", u.state === "error");
  show($("update-progress-box"), busy || u.state === "error");
  show($("btn-update-later"), !busy);
  show($("btn-update-dismiss"), !busy);
  show($("btn-update-manual"), u.state === "error");
  now.disabled = busy;

  const steps = {
    downloading: [u.progress || 0, "Downloading… " + (u.progress || 0) + "%", "Downloading"],
    verifying: [75, "Checking the release signature…", "Checking"],
    applying: [90, "Installing…", "Installing"],
    restarting: [100, "Restarting DeckTap…", "Restarting"],
  };
  if (busy) {
    const [pct, text, label] = steps[u.state];
    fill.style.width = pct + "%"; ptxt.textContent = text; now.textContent = label + "…";
    sub.textContent = "You can keep presenting. DeckTap restarts when it's done.";
  } else if (u.state === "error") {
    fill.style.width = "100%"; ptxt.textContent = u.error || "The update didn't finish.";
    now.textContent = "Try again";
    sub.textContent = "The update didn't finish. Try again, or download it manually.";
  } else {
    now.textContent = "Update now";
    const mb = u.assetSize > 0 ? " (" + (u.assetSize / 1048576).toFixed(1) + " MB)" : "";
    sub.textContent = "A newer version is ready to install" + mb + ".";
  }
  $("update-notes-content").textContent = u.releaseNotes || "Improvements and fixes.";
}

function dismissUpdate() {
  updateDismissed = true;
  if (activeUpdate && activeUpdate.latestVersion) sessionStorage.setItem("dt_update_dismissed_" + activeUpdate.latestVersion, "1");
  show($("update-card"), false);
}
$("btn-update-dismiss").addEventListener("click", dismissUpdate);
$("btn-update-later").addEventListener("click", dismissUpdate);
$("btn-update-notes").addEventListener("click", () => $("update-notes-box").classList.toggle("hidden"));
$("btn-update-now").addEventListener("click", async () => {
  const btn = $("btn-update-now");
  btn.disabled = true; btn.textContent = "Starting…";
  try {
    const data = await (await fetch("update/apply", { method: "POST" })).json();
    if (!data.ok && data.error) { showToast("Couldn't start the update: " + data.error, true); btn.disabled = false; btn.textContent = "Update now"; }
  } catch { btn.disabled = false; btn.textContent = "Update now"; }
  refreshStatus();
});
$("btn-update-manual").addEventListener("click", () => fetch("update/manual", { method: "POST" }).catch(() => {}));

setInterval(refreshStatus, 1500);

$("btn-copy-link").addEventListener("click", () => {
  if (!lastPairingURL) return;
  navigator.clipboard.writeText(lastPairingURL).then(() => showToast("Pairing link copied"));
});

async function reloadDeckNow() {
  setMenu(false);
  try {
    const res = await fetch("deck/reload", { method: "POST" });
    showToast(res.ok ? "Notes reloaded from PowerPoint" : "Couldn't reload notes", !res.ok);
  } catch { showToast("Couldn't reach DeckTap", true); }
}
$("btn-reload-deck").addEventListener("click", reloadDeckNow);

async function checkUpdateNow() {
  const btn = $("btn-check-update"), label = btn.querySelector("span");
  btn.disabled = true; label.textContent = "Checking…";
  try {
    const data = await (await fetch("update/check", { method: "POST" })).json();
    if (!data.ok) showToast("Couldn't check for updates", true);
    else if (data.available) {
      updateDismissed = false;
      renderUpdate({ available: true, latestVersion: data.latestVersion, releaseURL: data.releaseURL, releaseNotes: data.releaseNotes, state: "idle" });
      setMenu(false);
    } else showToast("DeckTap is up to date");
  } catch { showToast("Couldn't check for updates", true); }
  btn.disabled = false; label.textContent = "Check for updates";
}
$("btn-check-update").addEventListener("click", checkUpdateNow);

let toastTimer = null;
function showToast(msg, isErr) {
  const t = $("update-toast");
  t.textContent = msg;
  t.className = "toast" + (isErr ? " error" : "");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => show(t, false), 3000);
}

let isQuitting = false;
$("btn-quit").addEventListener("click", async () => {
  if (isQuitting) return;
  isQuitting = true;
  try { await fetch("quit", { method: "POST" }); } catch (e) { console.warn("quit fetch failed", e); }
  document.body.innerHTML = '<div style="margin:auto;text-align:center"><div style="font-size:21px;font-weight:300">DeckTap has quit</div>' +
    '<div style="font-size:13px;color:var(--muted);margin-top:4px">You can close this window.</div></div>';
  setTimeout(() => { try { window.close(); } catch {} }, 300);
});
</script>
</body>
</html>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(b.String()))
}
