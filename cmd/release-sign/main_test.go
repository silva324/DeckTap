package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/silva324/DeckTap/internal/updater"
)

func TestKeygenSignVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "signing.key")
	var out bytes.Buffer
	if err := run([]string{"keygen", "-out", keyFile}, nil, &out); err != nil {
		t.Fatal(err)
	}
	seed, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no Unix permission bits (os.Stat reports 0666 there).
	if fi, _ := os.Stat(keyFile); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode %v, want 0600", fi.Mode().Perm())
	}
	if strings.Contains(out.String(), strings.TrimSpace(string(seed))) {
		t.Fatal("keygen must never print the private key")
	}
	pubLine := strings.TrimSpace(strings.SplitN(strings.SplitN(out.String(), "Public key:\n", 2)[1], "\n", 2)[0])
	pub, err := updater.ParsePublicKey(pubLine)
	if err != nil {
		t.Fatalf("printed public key: %v", err)
	}

	checksums := filepath.Join(dir, "checksums.txt")
	sigFile := filepath.Join(dir, "checksums.txt.sig")
	manifest := []byte("abc  DeckTap-v1.2.3-macOS-Universal.tar.gz\n")
	os.WriteFile(checksums, manifest, 0o644)
	env := func(k string) string {
		if k == KeyEnv {
			return string(seed)
		}
		return ""
	}
	if err := run([]string{"sign", "-tag", "v1.2.3", "-in", checksums, "-out", sigFile}, env, &out); err != nil {
		t.Fatal(err)
	}
	sig, _ := os.ReadFile(sigFile)
	if err := updater.VerifyChecksums(pub, "v1.2.3", manifest, sig); err != nil {
		t.Fatalf("updater rejected the CLI signature: %v", err)
	}
	if err := updater.VerifyChecksums(pub, "v1.2.4", manifest, sig); err == nil {
		t.Fatal("signature must be bound to its tag")
	}
}

func TestKeygenRefusesToOverwrite(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "signing.key")
	os.WriteFile(keyFile, []byte("existing"), 0o600)
	if err := run([]string{"keygen", "-out", keyFile}, nil, &bytes.Buffer{}); err == nil {
		t.Fatal("keygen must not overwrite an existing key")
	}
	if b, _ := os.ReadFile(keyFile); string(b) != "existing" {
		t.Fatal("existing key was modified")
	}
}

func TestSignFailsWithoutValidKey(t *testing.T) {
	dir := t.TempDir()
	checksums := filepath.Join(dir, "checksums.txt")
	os.WriteFile(checksums, []byte("abc  x\n"), 0o644)
	for name, val := range map[string]string{"missing": "", "not base64": "!!", "wrong size": "AAAA"} {
		env := func(string) string { return val }
		err := run([]string{"sign", "-tag", "v1.0.0", "-in", checksums, "-out", filepath.Join(dir, "sig")}, env, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), KeyEnv) {
			t.Errorf("%s: expected an error naming %s, got %v", name, KeyEnv, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "sig")); err == nil {
		t.Fatal("no signature file may be written without a valid key")
	}
}
