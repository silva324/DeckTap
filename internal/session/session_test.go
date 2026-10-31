package session

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/silva324/DeckTap/internal/e2ee"
	"github.com/silva324/DeckTap/internal/ws"
)

type phone struct {
	t     *testing.T
	conn  *ws.Conn
	c     *e2ee.Cipher
	seq   uint64
	hseq  uint64
	sid   string
	token string
	pin   string
}

func newHub(t *testing.T, opts Options) (*Hub, *httptest.Server, chan Command) {
	cmds := make(chan Command, 10)
	opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	opts.OnCommand = func(c Command) { cmds <- c }
	opts.CurrentState = func() map[string]any { return map[string]any{"currentSlide": 1} }
	h := New(opts)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return h, srv, cmds
}

func dial(t *testing.T, srv *httptest.Server, h *Hub, fragment string) *phone {
	t.Helper()
	parts := strings.SplitN(fragment, ".", 2)
	key, _ := e2ee.B64.DecodeString(parts[1])
	c, _ := e2ee.NewCipher(key)
	conn, err := ws.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	pin := ""
	if h != nil {
		pin = h.Pin()
	}
	return &phone{t: t, conn: conn, c: c, token: parts[0], pin: pin}
}

func (p *phone) send(v map[string]any) {
	frame, _ := p.c.SealJSON(e2ee.ClientToHost, v)
	if err := p.conn.WriteText(frame); err != nil {
		p.t.Fatal(err)
	}
}

func (p *phone) recv() (map[string]any, error) {
	p.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := p.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	pt, err := p.c.OpenFrame(e2ee.HostToClient, data, false)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	json.Unmarshal(pt, &m)
	if s := uint64(m["seq"].(float64)); s <= p.hseq {
		p.t.Fatalf("host seq not increasing: %d <= %d", s, p.hseq)
	} else {
		p.hseq = s
	}
	return m, nil
}

func (p *phone) hello() map[string]any {
	p.seq++
	p.send(map[string]any{"type": "AUTH_HELLO", "pairingToken": p.token, "clientNonce": "n1", "seq": p.seq, "ts": time.Now().Unix(), "pin": p.pin})
	m, err := p.recv()
	if err != nil || m["type"] != "AUTH_OK" || m["clientNonce"] != "n1" {
		p.t.Fatalf("hello failed: %v %v", m, err)
	}
	p.sid = m["sessionId"].(string)
	return m
}

func closeCode(err error) int {
	var ce *ws.CloseError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return -1
}

func TestPairCommandAndTokenBurn(t *testing.T) {
	h, srv, cmds := newHub(t, Options{})
	frag := h.Fragment()
	p := dial(t, srv, h, frag)
	m := p.hello()
	if m["seq"].(float64) != 1 {
		t.Fatal("AUTH_OK must have seq 1")
	}
	if h.Fragment() != "" || h.Status().State != Paired {
		t.Fatal("token must be burned and hub locked to the client")
	}
	p.seq++
	p.send(map[string]any{"seq": p.seq, "sid": p.sid, "action": "NEXT"})
	if c := <-cmds; c.Action != "NEXT" {
		t.Fatal(c)
	}
	// a second device scanning the same QR is rejected
	q := dial(t, srv, h, frag)
	q.hello2Fail(t)
}

func TestWrongPINFollowedByCorrectPIN(t *testing.T) {
	h, srv, cmds := newHub(t, Options{})
	frag := h.Fragment()

	// 1. Phone attempts with incorrect PIN
	p := dial(t, srv, h, frag)
	p.pin = "0000"
	if h.Pin() == "0000" {
		p.pin = "1111"
	}
	p.seq++
	p.send(map[string]any{"type": "AUTH_HELLO", "pairingToken": p.token, "clientNonce": "n-bad", "seq": p.seq, "ts": time.Now().Unix(), "pin": p.pin})
	m, err := p.recv()
	if err != nil || m["type"] != "AUTH_FAIL" {
		t.Fatalf("expected AUTH_FAIL on bad PIN, got %v %v", m, err)
	}
	if s := uint64(m["seq"].(float64)); s != 1 {
		t.Fatalf("expected AUTH_FAIL seq 1, got %d", s)
	}

	// 2. Phone re-attempts with correct PIN
	p2 := dial(t, srv, h, frag)
	p2.hseq = p.hseq // client remembers previous host sequence seen (1)
	p2.seq = 1
	p2.pin = h.Pin()
	m2 := p2.hello() // hello() calls recv() which verifies s > p2.hseq and updates p2.hseq
	if s := uint64(m2["seq"].(float64)); s != 2 {
		t.Fatalf("AUTH_OK expected seq 2, got %d", s)
	}

	p2.seq++
	p2.send(map[string]any{"seq": p2.seq, "sid": p2.sid, "action": "NEXT"})
	if c := <-cmds; c.Action != "NEXT" {
		t.Fatal(c)
	}
}

func (p *phone) hello2Fail(t *testing.T) {
	p.seq = 99
	p.send(map[string]any{"type": "AUTH_HELLO", "pairingToken": p.token, "clientNonce": "x", "seq": p.seq, "pin": p.pin})
	if _, err := p.recv(); closeCode(err) != ws.ClosePolicyViolation {
		t.Fatalf("expected 1008, got %v", err)
	}
}

func TestReplayIsRejected(t *testing.T) {
	h, srv, cmds := newHub(t, Options{})
	p := dial(t, srv, h, h.Fragment())
	p.hello()
	p.seq++
	frame, _ := p.c.SealJSON(e2ee.ClientToHost, map[string]any{"seq": p.seq, "sid": p.sid, "action": "NEXT"})
	p.conn.WriteText(frame)
	<-cmds
	p.conn.WriteText(frame) // exact replay of a captured frame
	if _, err := p.recv(); closeCode(err) != ws.ClosePolicyViolation {
		t.Fatalf("replay must close with 1008, got %v", err)
	}
	if h.Status().Failures != 1 {
		t.Fatal("replay must count as failure")
	}
}

func TestResumeRotatesSessionAndSupersedes(t *testing.T) {
	h, srv, cmds := newHub(t, Options{})
	frag := h.Fragment()
	p := dial(t, srv, h, frag)
	p.hello()
	oldSID := p.sid

	// "screen woke up": new socket, same keys, continuing counters
	p2 := dial(t, srv, h, frag)
	p2.seq, p2.hseq = p.seq+1, p.hseq
	p2.send(map[string]any{"type": "AUTH_RESUME", "sessionId": oldSID, "clientNonce": "r1", "seq": p2.seq})
	m, err := p2.recv()
	if err != nil || m["type"] != "AUTH_OK" || m["sessionId"] == oldSID {
		t.Fatalf("resume: %v %v", m, err)
	}
	p2.sid = m["sessionId"].(string)
	if _, err := p.recv(); closeCode(err) != CloseSuperseded {
		t.Fatalf("old socket must get 4000, got %v", err)
	}
	p2.seq++
	p2.send(map[string]any{"seq": p2.seq, "sid": p2.sid, "action": "GOTO", "slide": 7})
	if c := <-cmds; c.Action != "GOTO" || c.Slide != 7 {
		t.Fatal(c)
	}
	// resuming with the rotated-away SID fails
	p3 := dial(t, srv, h, frag)
	p3.seq = 1000
	p3.send(map[string]any{"type": "AUTH_RESUME", "sessionId": oldSID, "seq": p3.seq})
	if _, err := p3.recv(); closeCode(err) != ws.ClosePolicyViolation {
		t.Fatalf("stale sid must fail, got %v", err)
	}
}

func TestWrongKeyLockout(t *testing.T) {
	var locked atomic.Bool
	h, srv, _ := newHub(t, Options{OnLockout: func() { locked.Store(true) }})
	bogus := e2ee.B64.EncodeToString(e2ee.RandomBytes(16)) + "." + e2ee.B64.EncodeToString(e2ee.RandomBytes(32))
	for i := 0; i < 5; i++ {
		p := dial(t, srv, h, bogus)
		p.hello2Fail(t)
	}
	time.Sleep(50 * time.Millisecond)
	if !locked.Load() || h.Status().State != LockedOut || h.Fragment() != "" {
		t.Fatal("5 failures must lock out")
	}
	resp, _ := http.Get(srv.URL + "/ws")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatal("locked hub must refuse upgrades")
	}
	h.Reset()
	if h.Status().State != Unpaired || h.Fragment() == "" {
		t.Fatal("reset must re-arm pairing")
	}
}

func TestAuthTimeout(t *testing.T) {
	h, srv, _ := newHub(t, Options{AuthTimeout: 150 * time.Millisecond})
	p := dial(t, srv, h, h.Fragment())
	if _, err := p.recv(); err == nil {
		t.Fatal("silent client must be dropped")
	}
	if h.Status().Failures != 0 {
		t.Fatal("timeouts are not counted as crypto failures")
	}
}

func TestResetKicksClient(t *testing.T) {
	h, srv, _ := newHub(t, Options{})
	old := h.Fragment()
	p := dial(t, srv, h, old)
	p.hello()
	h.Reset()
	if _, err := p.recv(); closeCode(err) != CloseReset {
		t.Fatalf("expected 4001, got %v", err)
	}
	if h.Fragment() == old {
		t.Fatal("secrets must rotate")
	}
}

func TestCrossOriginRejected(t *testing.T) {
	_, srv, _ := newHub(t, Options{})
	_, err := ws.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", http.Header{"Origin": {"https://evil.example"}})
	if err == nil {
		t.Fatal("cross-origin upgrade must fail")
	}
}
