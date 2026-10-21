package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
)

const (
	MasterKeyLength    = 32 // 256 bits for AES-256-GCM
	PairingTokenLength = 16 // 128 bits
	SessionIDLength    = 16 // 128 bits
	GCMNonceSize       = 12 // 96 bits standard for GCM
)

var (
	ErrInvalidKeySize    = errors.New("invalid key size: must be 32 bytes for AES-256")
	ErrDecryptionFailed  = errors.New("decryption failed: authentication tag mismatch or corrupted ciphertext")
	ErrInvalidCiphertext = errors.New("invalid ciphertext or iv encoding")
)

// GenerateMasterKey creates a 256-bit cryptographically secure random key.
func GenerateMasterKey() ([]byte, string, error) {
	key := make([]byte, MasterKeyLength)
	if _, err := rand.Read(key); err != nil {
		return nil, "", fmt.Errorf("failed to generate master key: %w", err)
	}
	return key, base64.RawURLEncoding.EncodeToString(key), nil
}

// GeneratePairingToken creates a 128-bit cryptographically secure one-time pairing token.
func GeneratePairingToken() ([]byte, string, error) {
	token := make([]byte, PairingTokenLength)
	if _, err := rand.Read(token); err != nil {
		return nil, "", fmt.Errorf("failed to generate pairing token: %w", err)
	}
	return token, base64.RawURLEncoding.EncodeToString(token), nil
}

// GeneratePinCode creates a 4-digit visual PIN ("0000" to "9999") using crypto/rand.
func GeneratePinCode() (string, error) {
	max := big.NewInt(10000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("failed to generate pin code: %w", err)
	}
	return fmt.Sprintf("%04d", n.Int64()), nil
}

// GenerateSessionID creates a 128-bit cryptographically secure session ID.
func GenerateSessionID() ([]byte, string, error) {
	id := make([]byte, SessionIDLength)
	if _, err := rand.Read(id); err != nil {
		return nil, "", fmt.Errorf("failed to generate session id: %w", err)
	}
	return id, base64.RawURLEncoding.EncodeToString(id), nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns base64url encoded IV and ciphertext+tag.
func Encrypt(key []byte, plaintext []byte) (string, string, error) {
	if len(key) != MasterKeyLength {
		return "", "", ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", fmt.Errorf("aes.NewCipher failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", fmt.Errorf("cipher.NewGCM failed: %w", err)
	}

	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return "", "", fmt.Errorf("failed to read random IV: %w", err)
	}

	// Seal appends authentication tag (16 bytes) to ciphertext
	sealed := gcm.Seal(nil, iv, plaintext, nil)

	ivStr := base64.RawURLEncoding.EncodeToString(iv)
	dataStr := base64.RawURLEncoding.EncodeToString(sealed)
	return ivStr, dataStr, nil
}

// Decrypt decrypts base64url encoded IV and ciphertext+tag using AES-256-GCM.
func Decrypt(key []byte, ivStr string, dataStr string) ([]byte, error) {
	if len(key) != MasterKeyLength {
		return nil, ErrInvalidKeySize
	}

	iv, err := base64.RawURLEncoding.DecodeString(ivStr)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid base64 iv", ErrInvalidCiphertext)
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(dataStr)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid base64 data", ErrInvalidCiphertext)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM failed: %w", err)
	}

	if len(iv) != gcm.NonceSize() {
		return nil, fmt.Errorf("%w: invalid nonce length %d (expected %d)", ErrInvalidCiphertext, len(iv), gcm.NonceSize())
	}

	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// ConstantTimeCompare compares two strings in constant time to prevent timing attacks.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
