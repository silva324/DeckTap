package e2ee

import (
	"bytes"
	"testing"
)

func TestRoundTripAndTamper(t *testing.T) {
	c, err := NewCipher(RandomBytes(KeySize))
	if err != nil {
		t.Fatal(err)
	}
	frame, _ := c.SealJSON(ClientToHost, map[string]any{"action": "NEXT", "seq": 2})
	pt, err := c.OpenFrame(ClientToHost, frame, false)
	if err != nil || !bytes.Contains(pt, []byte(`"NEXT"`)) {
		t.Fatalf("roundtrip: %v %s", err, pt)
	}
	if _, err := c.OpenFrame(HostToClient, frame, false); err != ErrAuth {
		t.Fatal("reflection across directions must fail")
	}
	other, _ := NewCipher(RandomBytes(KeySize))
	if _, err := other.OpenFrame(ClientToHost, frame, false); err != ErrAuth {
		t.Fatal("wrong key must fail")
	}
	iv, ct := c.Seal(ClientToHost, []byte("hi"))
	ct[0] ^= 1
	if _, err := c.Open(ClientToHost, iv, ct); err != ErrAuth {
		t.Fatal("tampered ct must fail")
	}
	bin := append(append([]byte{}, iv...), ct...)
	bin[NonceSize] ^= 1 // restore
	if pt, err := c.OpenFrame(ClientToHost, bin, true); err != nil || string(pt) != "hi" {
		t.Fatalf("binary frame: %v", err)
	}
	if _, err := c.OpenFrame(ClientToHost, []byte("{bad"), false); err != ErrEnvelope {
		t.Fatal("malformed")
	}
}

func TestEncodedLengths(t *testing.T) {
	if n := len(B64.EncodeToString(RandomBytes(TokenSize))); n != 22 {
		t.Fatal(n)
	}
	if n := len(B64.EncodeToString(RandomBytes(KeySize))); n != 43 {
		t.Fatal(n)
	}
}
