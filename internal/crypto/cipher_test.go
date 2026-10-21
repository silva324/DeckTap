package crypto

import (
	"bytes"
	"testing"
)

func TestKeyGeneration(t *testing.T) {
	rawKey, keyStr, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey error: %v", err)
	}
	if len(rawKey) != MasterKeyLength {
		t.Errorf("expected %d bytes, got %d", MasterKeyLength, len(rawKey))
	}
	if len(keyStr) == 0 {
		t.Error("expected non-empty key string")
	}

	rawToken, tokenStr, err := GeneratePairingToken()
	if err != nil {
		t.Fatalf("GeneratePairingToken error: %v", err)
	}
	if len(rawToken) != PairingTokenLength {
		t.Errorf("expected %d bytes, got %d", PairingTokenLength, len(rawToken))
	}
	if len(tokenStr) == 0 {
		t.Error("expected non-empty token string")
	}

	pin, err := GeneratePinCode()
	if err != nil {
		t.Fatalf("GeneratePinCode error: %v", err)
	}
	if len(pin) != 4 {
		t.Errorf("expected 4-digit PIN, got %q", pin)
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			t.Errorf("PIN contains non-digit: %c", c)
		}
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	rawKey, _, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey error: %v", err)
	}

	messages := []string{
		"Hello, World!",
		"",
		"{\"type\":\"ACTION\",\"action\":\"NEXT_SLIDE\"}",
		"Slide Notes: Zero-Trust presentation remote with AES-256-GCM E2EE & anti-shoulder-surfing PIN verification.",
	}

	for _, msg := range messages {
		iv, data, err := Encrypt(rawKey, []byte(msg))
		if err != nil {
			t.Fatalf("Encrypt error on %q: %v", msg, err)
		}

		decrypted, err := Decrypt(rawKey, iv, data)
		if err != nil {
			t.Fatalf("Decrypt error on %q: %v", msg, err)
		}

		if !bytes.Equal([]byte(msg), decrypted) {
			t.Errorf("expected %q, got %q", msg, string(decrypted))
		}
	}
}

func TestTamperDetection(t *testing.T) {
	rawKey, _, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey error: %v", err)
	}

	iv, data, err := Encrypt(rawKey, []byte("Sensitive command"))
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	// Tampering with data
	tamperedData := data[:len(data)-2] + "AA"
	_, err = Decrypt(rawKey, iv, tamperedData)
	if err == nil {
		t.Fatal("expected error on tampered data, got nil")
	}

	// Different key
	otherKey, _, _ := GenerateMasterKey()
	_, err = Decrypt(otherKey, iv, data)
	if err == nil {
		t.Fatal("expected error with different key, got nil")
	}
}

func TestConstantTimeCompare(t *testing.T) {
	if !ConstantTimeCompare("1234", "1234") {
		t.Error("expected matching strings to return true")
	}
	if ConstantTimeCompare("1234", "1235") {
		t.Error("expected non-matching strings to return false")
	}
	if ConstantTimeCompare("1234", "123") {
		t.Error("expected different length strings to return false")
	}
}
