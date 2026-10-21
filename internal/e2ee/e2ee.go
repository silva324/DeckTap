// Package e2ee implements the SPEC-SEC-001 application-layer encryption:
// AES-256-GCM with a 96-bit random nonce per frame, keys generated with crypto/rand
// and held only in memory.
//
// Hardening on top of the spec: every frame is bound to its direction through the
// GCM additional authenticated data (AAD), so a frame captured in one direction
// can never be reflected back in the other direction.
package e2ee

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	TokenSize = 16 // 128-bit pairing token  -> 22 chars RawURLEncoding
	KeySize   = 32 // 256-bit AES key        -> 43 chars RawURLEncoding
	NonceSize = 12
	TagSize   = 16
)

// Direction is authenticated as AAD. The values are part of the wire protocol.
type Direction string

const (
	ClientToHost Direction = "slidecue/v1 c2h"
	HostToClient Direction = "slidecue/v1 h2c"
)

var (
	ErrEnvelope = errors.New("e2ee: malformed envelope")
	ErrAuth     = errors.New("e2ee: authentication failed")
)

// B64 is the encoding used for every binary value on the wire and in the QR fragment.
var B64 = base64.RawURLEncoding

// RandomBytes returns n bytes from crypto/rand.
func RandomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error()) // unrecoverable: never fall back to weak randomness
	}
	return b
}

// Cipher wraps an AES-256-GCM AEAD.
type Cipher struct{ aead cipher.AEAD }

func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("e2ee: key must be %d bytes", KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Seal returns (iv, ciphertext||tag).
func (c *Cipher) Seal(dir Direction, plaintext []byte) (iv, ct []byte) {
	iv = RandomBytes(NonceSize)
	return iv, c.aead.Seal(nil, iv, plaintext, []byte(dir))
}

func (c *Cipher) Open(dir Direction, iv, ct []byte) ([]byte, error) {
	if len(iv) != NonceSize || len(ct) < TagSize {
		return nil, ErrEnvelope
	}
	pt, err := c.aead.Open(nil, iv, ct, []byte(dir))
	if err != nil {
		return nil, ErrAuth
	}
	return pt, nil
}

// Envelope is wire format Option A (JSON text frame).
type Envelope struct {
	IV string `json:"iv"`
	CT string `json:"ct"`
}

// SealJSON marshals v and returns an Option A text frame.
func (c *Cipher) SealJSON(dir Direction, v any) ([]byte, error) {
	pt, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	iv, ct := c.Seal(dir, pt)
	return json.Marshal(Envelope{IV: B64.EncodeToString(iv), CT: B64.EncodeToString(ct)})
}

// OpenFrame decrypts an Option A (text/JSON) or Option B (binary: iv||ct||tag) frame.
func (c *Cipher) OpenFrame(dir Direction, frame []byte, binary bool) ([]byte, error) {
	var iv, ct []byte
	if binary {
		if len(frame) < NonceSize+TagSize {
			return nil, ErrEnvelope
		}
		iv, ct = frame[:NonceSize], frame[NonceSize:]
	} else {
		var env Envelope
		if err := json.Unmarshal(frame, &env); err != nil {
			return nil, ErrEnvelope
		}
		var err1, err2 error
		iv, err1 = B64.DecodeString(env.IV)
		ct, err2 = B64.DecodeString(env.CT)
		if err1 != nil || err2 != nil {
			return nil, ErrEnvelope
		}
	}
	return c.Open(dir, iv, ct)
}
