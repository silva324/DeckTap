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
	"testing"
	"time"

	"pptx-remote/internal/e2ee"
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
	if st["totalSlides"].(float64) != 5 {
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
