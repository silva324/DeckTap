package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"pptx-remote/internal/e2ee"
	"pptx-remote/internal/pptx"
	"pptx-remote/internal/presenter"
	"pptx-remote/internal/session"
	"pptx-remote/internal/ws"
)

const fakeCloudflared = `#!/bin/sh
n=$(date +%s%N | tail -c 6)
echo "INF |  https://fake-$n.trycloudflare.com  |" >&2
echo "INF Registered tunnel connection connIndex=0" >&2
exec sleep 60
`

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

func newTestHost(t *testing.T, mode Mode) *Host {
	t.Helper()
	ctl, err := presenter.NewDemo("../pptx/testdata/demo.pptx")
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Mode: mode, PollInterval: 50 * time.Millisecond}
	if mode == ModeTunnel {
		bin := filepath.Join(t.TempDir(), "cloudflared")
		os.WriteFile(bin, []byte(fakeCloudflared), 0o755)
		cfg.Cloudflared = bin
	}
	h := NewHost(cfg, ctl, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := h.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Shutdown)
	return h
}

type testPhone struct {
	conn *ws.Conn
	c    *e2ee.Cipher
	seq  uint64
	sid  string
}

func pair(t *testing.T, h *Host, origin string) *testPhone {
	t.Helper()
	url := h.PairingURL()
	frag := url[strings.Index(url, "#")+1:]
	if idx := strings.Index(frag, "@"); idx != -1 {
		frag = frag[:idx]
	}
	parts := strings.SplitN(frag, ".", 2)
	key, err := e2ee.B64.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode key: %v (parts[1]=%q)", err, parts[1])
	}
	c, err := e2ee.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	hdr := http.Header{}
	if origin != "" {
		hdr.Set("Origin", origin)
	}
	conn, err := ws.Dial("ws://127.0.0.1:"+strconv.Itoa(h.Port())+"/ws", hdr)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("skipping live socket test: network dial not permitted in sandbox: %v", err)
		}
		t.Fatal(err)
	}
	p := &testPhone{conn: conn, c: c}
	p.send(t, map[string]any{"type": "AUTH_HELLO", "pairingToken": parts[0], "clientNonce": "abc", "pin": h.Snapshot().Pin})
	m := p.recvType(t, "AUTH_OK")
	p.sid = m["sessionId"].(string)
	st := m["state"].(map[string]any)
	if st["totalSlides"].(float64) <= 0 {
		t.Fatalf("AUTH_OK state: %v", st)
	}
	return p
}

func (p *testPhone) send(t *testing.T, v map[string]any) {
	p.seq++
	v["seq"] = p.seq
	if p.sid != "" {
		v["sid"] = p.sid
	}
	f, _ := p.c.SealJSON(e2ee.ClientToHost, v)
	if err := p.conn.WriteText(f); err != nil {
		t.Fatal(err)
	}
}

func (p *testPhone) recvType(t *testing.T, typ string) map[string]any {
	t.Helper()
	for i := 0; i < 50; i++ {
		p.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, err := p.conn.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for %s: %v", typ, err)
		}
		pt, err := p.c.OpenFrame(e2ee.HostToClient, data, false)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		json.Unmarshal(pt, &m)
		if m["type"] == typ {
			return m
		}
	}
	t.Fatalf("no %s", typ)
	return nil
}

func TestLocalModeEndToEnd(t *testing.T) {
	h := newTestHost(t, ModeLocal)
	waitFor(t, "pairing url", func() bool { return h.PairingURL() != "" })
	waitFor(t, "deck ready", func() bool { return h.Sync.Status().Total > 0 })
	origin := "http://127.0.0.1:" + strconv.Itoa(h.Port())
	p := pair(t, h, origin)
	deck := p.recvType(t, "DECK")
	if len(deck["slides"].([]any)) != 5 {
		t.Fatal("deck outline")
	}
	p.send(t, map[string]any{"action": "START_SHOW", "slide": 2})
	for {
		m := p.recvType(t, "STATE_UPDATE")
		if m["isPresenting"] == true && m["currentSlide"].(float64) == 2 {
			if m["title"] != "How it works" || !strings.Contains(m["notes"].(string), "WebSocket") || m["nextTitle"] != "Speaker notes" {
				t.Fatalf("state: %v", m)
			}
			break
		}
	}
	p.send(t, map[string]any{"action": "NEXT"})
	for m := p.recvType(t, "STATE_UPDATE"); m["currentSlide"].(float64) != 3; m = p.recvType(t, "STATE_UPDATE") {
	}
	if h.PairingURL() != "" {
		t.Fatal("QR must not be available after pairing")
	}
	s := h.Snapshot()
	if !s.Session.Connected || !s.Show.Presenting {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestTunnelModeLockoutAndReset(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake cloudflared")
	}
	h := newTestHost(t, ModeTunnel)
	waitFor(t, "tunnel online", func() bool { return h.Snapshot().TunnelState == TunnelOnline })
	url := h.PairingURL()
	if !strings.HasPrefix(url, "https://silva324.github.io/DeckTap/#") || !strings.Contains(url, "@fake-") || !strings.Contains(url, ".trycloudflare.com") {
		t.Fatalf("pairing url %q", url)
	}
	directURL := h.DirectURL()
	if !strings.HasPrefix(directURL, "https://fake-") || !strings.Contains(directURL, ".trycloudflare.com/#") {
		t.Fatalf("direct url %q", directURL)
	}
	public := h.Snapshot().PublicURL

	// verify allowed origins
	if !h.allowedOrigin(public) {
		t.Fatalf("expected public origin %q to be allowed", public)
	}
	if !h.allowedOrigin("https://silva324.github.io") {
		t.Fatal("expected canonical PWA origin https://silva324.github.io to be allowed")
	}
	if !h.allowedOrigin("https://silva324.github.io/") {
		t.Fatal("expected canonical PWA origin with trailing slash to be allowed")
	}
	if !h.allowedOrigin("null") {
		t.Fatal("expected 'null' origin from iOS Safari to be allowed")
	}
	if !h.allowedOrigin("apple-web-clip://") {
		t.Fatal("expected apple-web-clip origin to be allowed")
	}
	if h.allowedOrigin("https://evil.example") {
		t.Fatal("foreign origin should be rejected")
	}

	// cloudflared forwards to 127.0.0.1, the browser Origin is the canonical PWA
	p := pair(t, h, "https://silva324.github.io")
	p.send(t, map[string]any{"action": "PING"})
	p.recvType(t, "PONG")

	// a foreign origin is refused on websocket dial
	if _, err := ws.Dial("ws://127.0.0.1:"+strconv.Itoa(h.Port())+"/ws", http.Header{"Origin": {"https://evil.example"}}); err == nil {
		t.Fatal("foreign origin accepted")
	}

	// 5 attackers with a wrong key → lockout → tunnel destroyed
	for i := 0; i < 5; i++ {
		c, err := ws.Dial("ws://127.0.0.1:"+strconv.Itoa(h.Port())+"/ws", http.Header{"Origin": {public}})
		if err != nil {
			break // already locked
		}
		bad, _ := e2ee.NewCipher(e2ee.RandomBytes(32))
		f, _ := bad.SealJSON(e2ee.ClientToHost, map[string]any{"type": "AUTH_RESUME", "seq": 99})
		c.WriteText(f)
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		c.ReadMessage()
	}
	waitFor(t, "lockout", func() bool {
		s := h.Snapshot()
		return s.Session.State == session.LockedOut && s.TunnelState == TunnelLocked && s.PublicURL == ""
	})

	h.Reset()
	waitFor(t, "new tunnel", func() bool {
		s := h.Snapshot()
		return s.TunnelState == TunnelOnline && s.PublicURL != "" && s.PublicURL != public && h.PairingURL() != ""
	})
}

type mockNotesPresenter struct {
	mu    sync.Mutex
	slide int
	title string
	notes string
}

func (m *mockNotesPresenter) Name() string { return "mock-notes" }
func (m *mockNotesPresenter) Close()       {}
func (m *mockNotesPresenter) Status(context.Context) (presenter.Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return presenter.Status{
		AppRunning: true,
		DeckName:   "MockDeck.pptx",
		Total:      3,
		Slide:      m.slide,
		SlideTitle: m.title,
		SlideNotes: m.notes,
	}, nil
}
func (m *mockNotesPresenter) AppSlides(context.Context) ([]pptx.Slide, error) {
	return nil, nil
}
func (m *mockNotesPresenter) Start(context.Context, int) error               { return nil }
func (m *mockNotesPresenter) End(context.Context) error                      { return nil }
func (m *mockNotesPresenter) Next(context.Context) error                     { return nil }
func (m *mockNotesPresenter) Prev(context.Context) error                     { return nil }
func (m *mockNotesPresenter) Goto(context.Context, int) error                { return nil }
func (m *mockNotesPresenter) ToggleBlack(context.Context) error              { return nil }
func (m *mockNotesPresenter) SwitchDeck(context.Context, string, bool) error { return nil }
func (m *mockNotesPresenter) Diagnose(context.Context) []string              { return nil }

func TestLiveNotesSyncAndRefresh(t *testing.T) {
	mockCtl := &mockNotesPresenter{
		slide: 1,
		title: "Initial Title",
		notes: "Original slide notes",
	}
	h := NewHost(Config{Mode: ModeLocal, PollInterval: 30 * time.Millisecond}, mockCtl, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := h.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Shutdown)

	origin := "http://127.0.0.1:" + strconv.Itoa(h.Port())
	p := pair(t, h, origin)

	// Presenter modifies the notes in PowerPoint in real-time
	mockCtl.mu.Lock()
	mockCtl.notes = "Updated in-memory notes without saving!"
	mockCtl.mu.Unlock()

	m := p.recvType(t, "STATE_UPDATE")
	if m["notes"] != "Updated in-memory notes without saving!" {
		t.Fatalf("expected updated live notes, got %v", m["notes"])
	}

	// Verify REFRESH command
	p.send(t, map[string]any{"action": "REFRESH"})
	h.ReloadDeck()
}

const fakeCloudflared7844Fail = `#!/bin/sh
echo "ERR DialContext error: dial tcp 198.41.200.63:7844: i/o timeout" >&2
exit 1
`

const fakeNgrokFallback = `#!/bin/sh
echo '{"lvl":"info","msg":"started tunnel","url":"https://corporate-bypass.ngrok-free.app"}'
exec sleep 60
`

func TestNgrokPort443Fallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake")
	}
	cfBin := filepath.Join(t.TempDir(), "cloudflared")
	os.WriteFile(cfBin, []byte(fakeCloudflared7844Fail), 0o755)

	ngrokBin := filepath.Join(t.TempDir(), "ngrok")
	os.WriteFile(ngrokBin, []byte(fakeNgrokFallback), 0o755)

	ctl, err := presenter.NewDemo("../pptx/testdata/demo.pptx")
	if err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Mode:           ModeTunnel,
		TunnelProvider: TunnelProviderAuto,
		Cloudflared:    cfBin,
		Ngrok:          ngrokBin,
		PollInterval:   50 * time.Millisecond,
	}

	h := NewHost(cfg, ctl, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := h.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Shutdown)

	waitFor(t, "ngrok fallback online", func() bool {
		s := h.Snapshot()
		return s.TunnelState == TunnelOnline && s.PublicURL == "https://corporate-bypass.ngrok-free.app" && s.TunnelProvider == "ngrok"
	})

	if !strings.Contains(h.PairingURL(), "corporate-bypass.ngrok-free.app") {
		t.Fatalf("pairing URL must contain ngrok host, got %q", h.PairingURL())
	}
}
