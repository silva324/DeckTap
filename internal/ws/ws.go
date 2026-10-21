// Package ws is a minimal, dependency-free RFC 6455 WebSocket implementation.
//
// Scope: what DeckTap needs and nothing more — server upgrade, text/binary messages,
// fragmentation, ping/pong, close handshake, message size limits and deadlines.
// A tiny client (Dial) is included for tests and tooling.
package ws

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	OpContinuation = 0x0
	OpText         = 0x1
	OpBinary       = 0x2
	OpClose        = 0x8
	OpPing         = 0x9
	OpPong         = 0xA
)

// Close codes used by DeckTap.
const (
	CloseNormal          = 1000
	CloseGoingAway       = 1001
	ClosePolicyViolation = 1008
	CloseTooBig          = 1009
)

const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

var ErrClosed = errors.New("ws: connection closed")

// CloseError is returned by ReadMessage when the peer closes the connection.
type CloseError struct {
	Code   int
	Reason string
}

func (e *CloseError) Error() string { return fmt.Sprintf("ws: closed %d %s", e.Code, e.Reason) }

// Conn is a WebSocket connection. ReadMessage must be called from one goroutine;
// write methods are safe for concurrent use.
type Conn struct {
	nc       net.Conn
	br       *bufio.Reader
	isClient bool
	maxSize  int64

	wmu       sync.Mutex
	closeOnce sync.Once
	closed    bool
}

func headerContains(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

func acceptKey(key string) string {
	h := sha1.Sum([]byte(key + guid))
	return base64.StdEncoding.EncodeToString(h[:])
}

// IsUpgrade reports whether r is a WebSocket upgrade request.
func IsUpgrade(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		headerContains(r.Header, "Connection", "upgrade") &&
		headerContains(r.Header, "Upgrade", "websocket")
}

// Upgrade performs the server handshake. maxMessage limits inbound message size.
func Upgrade(w http.ResponseWriter, r *http.Request, maxMessage int64) (*Conn, error) {
	if !IsUpgrade(r) {
		http.Error(w, "websocket upgrade required", http.StatusBadRequest)
		return nil, errors.New("ws: not an upgrade request")
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		w.Header().Set("Sec-WebSocket-Version", "13")
		http.Error(w, "unsupported version", http.StatusUpgradeRequired)
		return nil, errors.New("ws: bad version")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if decoded, err := base64.StdEncoding.DecodeString(key); err != nil || len(decoded) != 16 {
		http.Error(w, "bad key", http.StatusBadRequest)
		return nil, errors.New("ws: bad key")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		return nil, errors.New("ws: hijack unsupported")
	}
	nc, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey(key) + "\r\n\r\n"
	if _, err := nc.Write([]byte(resp)); err != nil {
		nc.Close()
		return nil, err
	}
	return &Conn{nc: nc, br: rw.Reader, maxSize: maxMessage}, nil
}

// Dial opens a client connection (ws:// only; used by tests).
func Dial(rawURL string, header http.Header) (*Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	nc, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	if err != nil {
		return nil, err
	}
	keyRaw := make([]byte, 16)
	rand.Read(keyRaw)
	key := base64.StdEncoding.EncodeToString(keyRaw)
	var b strings.Builder
	fmt.Fprintf(&b, "GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n", u.RequestURI(), u.Host)
	fmt.Fprintf(&b, "Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n", key)
	for k, vs := range header {
		for _, v := range vs {
			fmt.Fprintf(&b, "%s: %s\r\n", k, v)
		}
	}
	b.WriteString("\r\n")
	if _, err := nc.Write([]byte(b.String())); err != nil {
		nc.Close()
		return nil, err
	}
	br := bufio.NewReader(nc)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		nc.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols || resp.Header.Get("Sec-WebSocket-Accept") != acceptKey(key) {
		nc.Close()
		return nil, fmt.Errorf("ws: handshake failed: %s", resp.Status)
	}
	return &Conn{nc: nc, br: br, isClient: true, maxSize: 1 << 20}, nil
}

func (c *Conn) RemoteAddr() net.Addr               { return c.nc.RemoteAddr() }
func (c *Conn) SetReadDeadline(t time.Time) error  { return c.nc.SetReadDeadline(t) }
func (c *Conn) SetWriteDeadline(t time.Time) error { return c.nc.SetWriteDeadline(t) }

// ReadMessage returns the next complete data message. Ping/pong/close are handled internally.
func (c *Conn) ReadMessage() (op int, payload []byte, err error) {
	var msgOp int
	var buf []byte
	for {
		fin, frameOp, data, err := c.readFrame()
		if err != nil {
			return 0, nil, err
		}
		switch frameOp {
		case OpPing:
			if err := c.writeFrame(OpPong, data); err != nil {
				return 0, nil, err
			}
			continue
		case OpPong:
			continue
		case OpClose:
			code, reason := CloseNormal, ""
			if len(data) >= 2 {
				code = int(binary.BigEndian.Uint16(data[:2]))
				reason = string(data[2:])
			}
			c.WriteClose(code, "")
			c.nc.Close()
			return 0, nil, &CloseError{Code: code, Reason: reason}
		case OpText, OpBinary:
			if msgOp != 0 {
				return 0, nil, c.fail(ClosePolicyViolation, "unexpected new message")
			}
			msgOp = frameOp
		case OpContinuation:
			if msgOp == 0 {
				return 0, nil, c.fail(ClosePolicyViolation, "unexpected continuation")
			}
		default:
			return 0, nil, c.fail(ClosePolicyViolation, "bad opcode")
		}
		if int64(len(buf)+len(data)) > c.maxSize {
			return 0, nil, c.fail(CloseTooBig, "message too big")
		}
		buf = append(buf, data...)
		if fin {
			return msgOp, buf, nil
		}
	}
}

func (c *Conn) fail(code int, reason string) error {
	c.WriteClose(code, reason)
	c.nc.Close()
	return &CloseError{Code: code, Reason: reason}
}

func (c *Conn) readFrame() (fin bool, op int, data []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(c.br, h[:]); err != nil {
		return
	}
	fin = h[0]&0x80 != 0
	if h[0]&0x70 != 0 {
		return false, 0, nil, c.fail(ClosePolicyViolation, "reserved bits set")
	}
	op = int(h[0] & 0x0F)
	masked := h[1]&0x80 != 0
	if masked == c.isClient { // client frames MUST be masked, server frames MUST NOT
		return false, 0, nil, c.fail(ClosePolicyViolation, "bad masking")
	}
	length := int64(h[1] & 0x7F)
	switch length {
	case 126:
		var ext [2]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return
		}
		length = int64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return
		}
		length = int64(binary.BigEndian.Uint64(ext[:]))
	}
	if op >= 0x8 && (length > 125 || !fin) {
		return false, 0, nil, c.fail(ClosePolicyViolation, "bad control frame")
	}
	if length > c.maxSize || length < 0 {
		return false, 0, nil, c.fail(CloseTooBig, "frame too big")
	}
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(c.br, mask[:]); err != nil {
			return
		}
	}
	data = make([]byte, length)
	if _, err = io.ReadFull(c.br, data); err != nil {
		return
	}
	if masked {
		for i := range data {
			data[i] ^= mask[i%4]
		}
	}
	return fin, op, data, nil
}

func (c *Conn) writeFrame(op int, data []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if c.closed && op != OpClose {
		return ErrClosed
	}
	hdr := make([]byte, 0, 14)
	hdr = append(hdr, 0x80|byte(op))
	maskBit := byte(0)
	if c.isClient {
		maskBit = 0x80
	}
	n := len(data)
	switch {
	case n <= 125:
		hdr = append(hdr, maskBit|byte(n))
	case n <= 0xFFFF:
		hdr = append(hdr, maskBit|126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, maskBit|127)
		hdr = binary.BigEndian.AppendUint64(hdr, uint64(n))
	}
	payload := data
	if c.isClient {
		var mask [4]byte
		rand.Read(mask[:])
		hdr = append(hdr, mask[:]...)
		payload = make([]byte, n)
		for i := range data {
			payload[i] = data[i] ^ mask[i%4]
		}
	}
	c.nc.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := c.nc.Write(append(hdr, payload...)); err != nil {
		return err
	}
	return nil
}

func (c *Conn) WriteMessage(op int, data []byte) error { return c.writeFrame(op, data) }
func (c *Conn) WriteText(data []byte) error            { return c.writeFrame(OpText, data) }
func (c *Conn) Ping() error                            { return c.writeFrame(OpPing, nil) }

// WriteClose sends a close frame once and marks the connection closed for writing.
func (c *Conn) WriteClose(code int, reason string) {
	c.closeOnce.Do(func() {
		if len(reason) > 120 {
			reason = reason[:120]
		}
		payload := binary.BigEndian.AppendUint16(nil, uint16(code))
		payload = append(payload, reason...)
		c.writeFrame(OpClose, payload)
		c.wmu.Lock()
		c.closed = true
		c.wmu.Unlock()
	})
}

// CloseWith sends a close frame and tears the TCP connection down shortly after.
func (c *Conn) CloseWith(code int, reason string) {
	c.WriteClose(code, reason)
	go func() {
		time.Sleep(250 * time.Millisecond) // give the frame a chance to flush through the proxy
		c.nc.Close()
	}()
}

// Close closes the TCP connection immediately.
func (c *Conn) Close() error { return c.nc.Close() }
