package updater

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// ReleasePublicKey is the base64 (standard encoding) Ed25519 public key that signs
// release checksums. Generate the pair with `go run ./cmd/release-sign keygen`; the
// private half lives only in the DECKTAP_UPDATE_SIGNING_KEY release secret.
//
// While it is empty, self-update is disabled and DeckTap falls back to opening the
// release page: an unsigned update is never installed.
const ReleasePublicKey = ""

// Asset names published next to the release packages.
const (
	ChecksumsAssetName = "checksums.txt"
	SignatureAssetName = "checksums.txt.sig"
)

// signingContext domain-separates release signatures from any other use of the key.
const signingContext = "decktap-release-checksums-v1"

var (
	ErrNoSigningKey     = errors.New("self-update is disabled in this build (no release signing key)")
	ErrInvalidSignature = errors.New("release signature is invalid")
)

// ParsePublicKey decodes a base64 Ed25519 public key.
func ParsePublicKey(s string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid Ed25519 public key")
	}
	return ed25519.PublicKey(raw), nil
}

// ChecksumsMessage is the exact byte string that is signed. Binding the release tag
// stops an old, genuinely signed manifest from being republished as a newer release
// to downgrade users onto a vulnerable build.
func ChecksumsMessage(tag string, checksums []byte) []byte {
	msg := make([]byte, 0, len(signingContext)+len(tag)+len(checksums)+2)
	msg = append(msg, signingContext...)
	msg = append(msg, '\n')
	msg = append(msg, strings.TrimSpace(tag)...)
	msg = append(msg, '\n')
	return append(msg, checksums...)
}

// SignChecksums returns the base64 signature file content for checksums.txt.
func SignChecksums(priv ed25519.PrivateKey, tag string, checksums []byte) []byte {
	sig := ed25519.Sign(priv, ChecksumsMessage(tag, checksums))
	return []byte(base64.StdEncoding.EncodeToString(sig) + "\n")
}

// VerifyChecksums checks a checksums.txt.sig file against checksums.txt for tag.
func VerifyChecksums(pub ed25519.PublicKey, tag string, checksums, sigFile []byte) error {
	if len(pub) != ed25519.PublicKeySize {
		return ErrNoSigningKey
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigFile)))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return ErrInvalidSignature
	}
	if !ed25519.Verify(pub, ChecksumsMessage(tag, checksums), sig) {
		return ErrInvalidSignature
	}
	return nil
}
