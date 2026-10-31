// Package session implements the SPEC-SEC-001 pairing state machine and E2EE transport.
//
//	UNPAIRED ──AUTH_HELLO(valid token)──▶ PAIRED(LOCKED) ──AUTH_RESUME(valid sid)──▶ PAIRED (sid rotated)
//	    ▲                                     │
//	    └──────────── Reset() ◀───────────────┘          UNPAIRED ──N failures──▶ LOCKED_OUT
//
// Rules implemented:
//   - Pairing token is single-use: burned on the first successful AUTH_HELLO.
//   - While PAIRED, only AUTH_RESUME with the active SessionID is accepted; the SessionID
//     rotates on every resume and the previous socket is closed (4000).
//   - Every frame is AES-256-GCM encrypted (see package e2ee) and carries a strictly
//     increasing sequence number per direction for the whole session (across reconnects),
//     so captured frames cannot be replayed.
//   - The first frame must arrive within AuthTimeout (3 s). GCM/auth/replay failures close
//     the socket with 1008.
//   - Only failed pairing attempts by a key holder (bad token/PIN, replayed or out-of-state
//     handshake while UNPAIRED) count towards MaxFailures; reaching it locks the hub out and
//     fires OnLockout (the app then destroys the tunnel). Undecryptable frames prove nothing
//     (there is no key to guess) and failures while PAIRED cannot advance an attacker (the
//     SessionID is 128-bit), so neither may lock out or evict the presenter's phone.
package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"pptx-remote/internal/e2ee"
	"pptx-remote/internal/ws"
)

type State string

const (
	Unpaired  State = "UNPAIRED"
	Paired    State = "PAIRED"
	LockedOut State = "LOCKED_OUT"
)

// Application close codes (4000-4999 are private use).
const (
	CloseSuperseded = 4000 // another connection resumed this session
	CloseReset      = 4001 // host rotated all secrets (hard reset)
	CloseShutdown   = 4002 // host is quitting
)

const maxInboundMessage = 64 << 10

// Command is a decrypted, authenticated client action.
type Command struct {
	Action        string
	Slide         int
	Deck          string
	PointerActive bool
	PointerX      float64
	PointerY      float64
	PointerMode   string
}

type Options struct {
	AuthTimeout  time.Duration // default 3s
	IdleTimeout  time.Duration // default 60s; client sends PING every ~20s
	PingInterval time.Duration // default 15s (WebSocket ping, keeps proxies alive)
	MaxFailures  int           // failed pairing attempts by a key holder before lockout, default 5
	MaxPending   int           // concurrent unauthenticated sockets, default 4

	// AllowedOrigin reports whether a browser Origin may open /ws (CSWSH defence).
	AllowedOrigin func(origin string) bool

	CurrentState    func() map[string]any // embedded in AUTH_OK
	OnAuthenticated func()                // e.g. push the deck outline
	OnCommand       func(Command)
	OnLockout       func()
	OnChange        func()
	Logger          *slog.Logger
}

type client struct {
	conn  *ws.Conn
	epoch uint64
}

// Hub owns the secrets and the (single) active client.
type Hub struct {
	opts Options

	mu            sync.Mutex
	state         State
	token         []byte // nil once burned
	key           []byte
	pin           string // 4-digit Anti-Shoulder-Surfing PIN
	cipher        *e2ee.Cipher
	sid           []byte
	lastClientSeq uint64
	hostSeq       uint64
	active        *client
	failures      int // counts towards MaxFailures
	rejected      int // every connection closed with 1008 (diagnostics)
	pending       int
	epoch         uint64
	lastSeen      time.Time

	sendMu sync.Mutex // keeps seq order == wire order
}

// Status is a snapshot for the UI.
type Status struct {
	State     State
	Connected bool
	Failures  int // failed pairing attempts counting towards lockout
	Rejected  int // all connections rejected for security reasons
	LastSeen  time.Time
}

func New(opts Options) *Hub {
	if opts.AuthTimeout == 0 {
		opts.AuthTimeout = 3 * time.Second
	}
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = 60 * time.Second
	}
	if opts.PingInterval == 0 {
		opts.PingInterval = 15 * time.Second
	}
	if opts.MaxFailures == 0 {
		opts.MaxFailures = 5
	}
	if opts.MaxPending == 0 {
		opts.MaxPending = 4
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	h := &Hub{opts: opts}
	h.mu.Lock()
	h.rotateLocked()
	h.mu.Unlock()
	return h
}

func (h *Hub) rotateLocked() {
	h.token = e2ee.RandomBytes(e2ee.TokenSize)
	h.key = e2ee.RandomBytes(e2ee.KeySize)
	max := big.NewInt(10000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic(err)
	}
	h.pin = fmt.Sprintf("%04d", n.Int64())

	c, err := e2ee.NewCipher(h.key)
	if err != nil {
		panic(err)
	}
	h.cipher = c
	h.sid = nil
	h.lastClientSeq, h.hostSeq = 0, 0
	h.state = Unpaired
	h.failures = 0
	h.epoch++
}

// Pin returns the active 4-digit visual PIN.
func (h *Hub) Pin() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pin
}

// Fragment returns "<PairingToken>.<MasterKey>" for the QR code, or "" when the token
// has been burned or the hub is locked out.
func (h *Hub) Fragment() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state != Unpaired || h.token == nil {
		return ""
	}
	return e2ee.B64.EncodeToString(h.token) + "." + e2ee.B64.EncodeToString(h.key)
}

func (h *Hub) Status() Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	return Status{State: h.state, Connected: h.active != nil, Failures: h.failures, Rejected: h.rejected, LastSeen: h.lastSeen}
}

// Reset is the "hard reset": kicks the client and rotates token, key and session.
func (h *Hub) Reset() {
	h.mu.Lock()
	old := h.active
	h.active = nil
	h.rotateLocked()
	h.mu.Unlock()
	if old != nil {
		old.conn.CloseWith(CloseReset, "reset by host")
	}
	h.opts.Logger.Info("pairing reset: new token, key and session")
	h.changed()
}

// Shutdown closes the active client.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	old := h.active
	h.active = nil
	h.mu.Unlock()
	if old != nil {
		old.conn.CloseWith(CloseShutdown, "host shutting down")
	}
}

func (h *Hub) changed() {
	if h.opts.OnChange != nil {
		h.opts.OnChange()
	}
}

// Send encrypts payload (adding "seq") and sends it to the active client, if any.
func (h *Hub) Send(payload map[string]any) error {
	h.sendMu.Lock()
	defer h.sendMu.Unlock()
	h.mu.Lock()
	cl, c := h.active, h.cipher
	if cl == nil {
		h.mu.Unlock()
		return nil
	}
	h.hostSeq++
	payload["seq"] = h.hostSeq
	h.mu.Unlock()
	frame, err := c.SealJSON(e2ee.HostToClient, payload)
	if err != nil {
		return err
	}
	return cl.conn.WriteText(frame)
}

// ---------------------------------------------------------------- HTTP

type authFrame struct {
	Type         string `json:"type"`
	PairingToken string `json:"pairingToken"`
	SessionID    string `json:"sessionId"`
	ClientNonce  string `json:"clientNonce"`
	PIN          string `json:"pin"`
	Seq          uint64 `json:"seq"`
	TS           int64  `json:"ts"`
}

type commandFrame struct {
	Seq           uint64  `json:"seq"`
	SID           string  `json:"sid"`
	Action        string  `json:"action"`
	Slide         int     `json:"slide,omitempty"`
	Deck          string  `json:"deck,omitempty"`
	PointerActive bool    `json:"active,omitempty"`
	PointerX      float64 `json:"x,omitempty"`
	PointerY      float64 `json:"y,omitempty"`
	PointerMode   string  `json:"mode,omitempty"`
}

func sameOrigin(r *http.Request) bool {
	u, err := url.Parse(r.Header.Get("Origin"))
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

// ServeHTTP handles GET /ws.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := h.opts.Logger
	if origin := r.Header.Get("Origin"); origin != "" {
		ok := sameOrigin(r)
		if h.opts.AllowedOrigin != nil {
			ok = ok || h.opts.AllowedOrigin(origin)
		}
		if !ok {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
	}
	h.mu.Lock()
	if h.state == LockedOut {
		h.mu.Unlock()
		http.Error(w, "locked", http.StatusForbidden)
		return
	}
	if h.pending >= h.opts.MaxPending {
		h.mu.Unlock()
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	h.pending++
	h.mu.Unlock()

	conn, err := ws.Upgrade(w, r, maxInboundMessage)
	if err != nil {
		h.mu.Lock()
		h.pending--
		h.mu.Unlock()
		return
	}
	cl, ok := h.authenticate(conn)
	h.mu.Lock()
	h.pending--
	h.mu.Unlock()
	if !ok {
		return
	}
	if h.opts.OnAuthenticated != nil {
		h.opts.OnAuthenticated()
	}
	h.changed()
	h.serve(cl)
	log.Debug("client loop ended")
}

func (h *Hub) authenticate(conn *ws.Conn) (*client, bool) {
	conn.SetReadDeadline(time.Now().Add(h.opts.AuthTimeout))
	op, data, err := conn.ReadMessage()
	if err != nil {
		conn.CloseWith(ws.ClosePolicyViolation, "auth timeout")
		return nil, false
	}
	conn.SetReadDeadline(time.Time{})

	h.mu.Lock()
	c, epoch, state := h.cipher, h.epoch, h.state
	h.mu.Unlock()

	pt, err := c.OpenFrame(e2ee.ClientToHost, data, op == ws.OpBinary)
	if err != nil {
		h.reject(conn, "handshake decryption failed", 0)
		return nil, false
	}
	var f authFrame
	if json.Unmarshal(pt, &f) != nil {
		h.reject(conn, "handshake malformed", 0)
		return nil, false
	}

	h.mu.Lock()
	if h.epoch != epoch || h.state != state { // reset raced with this handshake
		h.mu.Unlock()
		conn.CloseWith(CloseReset, "reset by host")
		return nil, false
	}
	var reason string
	switch {
	case state == Unpaired && f.Type == "AUTH_HELLO":
		tok, err := e2ee.B64.DecodeString(f.PairingToken)
		if err != nil || h.token == nil || subtle.ConstantTimeCompare(tok, h.token) != 1 {
			reason = "bad pairing token"
		} else if len(h.pin) > 0 && subtle.ConstantTimeCompare([]byte(f.PIN), []byte(h.pin)) != 1 {
			reason = "bad PIN"
			h.hostSeq++
			if failFrame, err := c.SealJSON(e2ee.HostToClient, map[string]any{
				"type":  "AUTH_FAIL",
				"error": "Invalid 4-digit PIN code. Please check presenter screen.",
				"seq":   h.hostSeq,
			}); err == nil {
				_ = conn.WriteText(failFrame)
			}
		}
	case state == Paired && f.Type == "AUTH_RESUME":
		sid, err := e2ee.B64.DecodeString(f.SessionID)
		if err != nil || h.sid == nil || subtle.ConstantTimeCompare(sid, h.sid) != 1 {
			reason = "bad session"
		}
	default:
		reason = "handshake not allowed in state " + string(state)
	}
	if reason == "" && f.Seq <= h.lastClientSeq {
		reason = "replayed handshake"
	}
	if reason != "" {
		h.mu.Unlock()
		var lockoutEpoch uint64
		if state == Unpaired {
			lockoutEpoch = epoch // the sender decrypted the frame: it holds the QR key
		}
		h.reject(conn, reason, lockoutEpoch)
		return nil, false
	}
	if state == Unpaired {
		h.token = nil // burn
		h.state = Paired
	}
	h.sid = e2ee.RandomBytes(16) // issued on pairing, rotated on every resume
	h.lastClientSeq = f.Seq
	h.lastSeen = time.Now()
	prev := h.active
	cl := &client{conn: conn, epoch: h.epoch}
	h.active = cl
	sid := e2ee.B64.EncodeToString(h.sid)
	h.mu.Unlock()

	if prev != nil {
		prev.conn.CloseWith(CloseSuperseded, "superseded")
	}
	h.opts.Logger.Info("client authenticated", "type", f.Type, "remote", conn.RemoteAddr().String())

	st := map[string]any{}
	if h.opts.CurrentState != nil {
		st = h.opts.CurrentState()
	}
	h.Send(map[string]any{
		"type": "AUTH_OK", "sessionId": sid, "clientNonce": f.ClientNonce,
		"serverTime": time.Now().UnixMilli(), "state": st,
	})
	return cl, true
}

func (h *Hub) isActive(cl *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.active == cl && h.epoch == cl.epoch
}

func (h *Hub) serve(cl *client) {
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		t := time.NewTicker(h.opts.PingInterval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if cl.conn.Ping() != nil {
					return
				}
			}
		}
	}()
	defer func() {
		h.mu.Lock()
		if h.active == cl {
			h.active = nil
		}
		h.mu.Unlock()
		cl.conn.Close()
		h.changed()
	}()

	for {
		cl.conn.SetReadDeadline(time.Now().Add(h.opts.IdleTimeout))
		op, data, err := cl.conn.ReadMessage()
		if err != nil || !h.isActive(cl) {
			return
		}
		h.mu.Lock()
		c := h.cipher
		h.mu.Unlock()
		pt, err := c.OpenFrame(e2ee.ClientToHost, data, op == ws.OpBinary)
		if err != nil {
			h.reject(cl.conn, "frame decryption failed", 0)
			return
		}
		var f commandFrame
		if json.Unmarshal(pt, &f) != nil {
			h.reject(cl.conn, "frame malformed", 0)
			return
		}
		h.mu.Lock()
		sid, _ := e2ee.B64.DecodeString(f.SID)
		valid := h.active == cl && h.sid != nil && subtle.ConstantTimeCompare(sid, h.sid) == 1 && f.Seq > h.lastClientSeq
		if valid {
			h.lastClientSeq = f.Seq
			h.lastSeen = time.Now()
		}
		h.mu.Unlock()
		if !valid {
			h.reject(cl.conn, "bad sid or replayed seq", 0)
			return
		}
		if f.Action == "PING" {
			h.Send(map[string]any{"type": "PONG"})
			continue
		}
		if h.opts.OnCommand != nil {
			h.opts.OnCommand(Command{
				Action:        f.Action,
				Slide:         f.Slide,
				Deck:          f.Deck,
				PointerActive: f.PointerActive,
				PointerX:      f.PointerX,
				PointerY:      f.PointerY,
				PointerMode:   f.PointerMode,
			})
		}
	}
}

// reject closes conn with 1008. lockoutEpoch is the hub epoch of a failed pairing attempt
// by a key holder, or 0 when the failure must not count towards MaxFailures (see the
// package doc). Lockout is only reachable while UNPAIRED, where there is no client to evict.
func (h *Hub) reject(conn *ws.Conn, reason string, lockoutEpoch uint64) {
	h.mu.Lock()
	h.rejected++
	counted := lockoutEpoch != 0 && lockoutEpoch == h.epoch && h.state == Unpaired
	if counted {
		h.failures++
	}
	n := h.failures
	locked := counted && n >= h.opts.MaxFailures
	if locked {
		h.state = LockedOut
		h.token = nil
	}
	h.mu.Unlock()
	h.opts.Logger.Warn("security: connection rejected", "reason", reason, "counted", counted,
		"failures", n, "remote", conn.RemoteAddr().String())
	conn.CloseWith(ws.ClosePolicyViolation, "policy violation")
	if locked {
		h.opts.Logger.Error("security: too many failed attempts, locking out", "failures", n)
		if h.opts.OnLockout != nil {
			go h.opts.OnLockout()
		}
	}
	h.changed()
}
