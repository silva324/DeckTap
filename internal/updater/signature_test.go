package updater

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestReleasePublicKeyIsValidOrEmpty(t *testing.T) {
	if ReleasePublicKey == "" {
		t.Skip("no release signing key embedded yet: self-update is disabled")
	}
	if _, err := ParsePublicKey(ReleasePublicKey); err != nil {
		t.Fatalf("ReleasePublicKey is set but invalid: %v", err)
	}
}

func TestParsePublicKey(t *testing.T) {
	pub, _ := testKey(t)
	got, err := ParsePublicKey(" " + base64.StdEncoding.EncodeToString(pub) + "\n")
	if err != nil || !bytes.Equal(got, pub) {
		t.Fatalf("round trip failed: %v", err)
	}
	for _, bad := range []string{"", "not-base64!", base64.StdEncoding.EncodeToString(pub[:31])} {
		if _, err := ParsePublicKey(bad); err == nil {
			t.Errorf("ParsePublicKey(%q) must fail", bad)
		}
	}
}

func TestVerifyChecksums(t *testing.T) {
	pub, priv := testKey(t)
	otherPub, _ := testKey(t)
	manifest := []byte("abc  DeckTap-v2.2.0-macOS-Universal.tar.gz\n")
	sig := SignChecksums(priv, "v2.2.0", manifest)

	if err := VerifyChecksums(pub, "v2.2.0", manifest, sig); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	cases := []struct {
		name     string
		pub      ed25519.PublicKey
		tag      string
		manifest []byte
		sig      []byte
		want     error
	}{
		{"no key embedded", nil, "v2.2.0", manifest, sig, ErrNoSigningKey},
		{"other key", otherPub, "v2.2.0", manifest, sig, ErrInvalidSignature},
		{"other tag (downgrade)", pub, "v9.9.9", manifest, sig, ErrInvalidSignature},
		{"tampered manifest", pub, "v2.2.0", append([]byte("evil  x\n"), manifest...), sig, ErrInvalidSignature},
		{"garbage signature", pub, "v2.2.0", manifest, []byte("not base64"), ErrInvalidSignature},
		{"empty signature", pub, "v2.2.0", manifest, nil, ErrInvalidSignature},
	}
	for _, c := range cases {
		if err := VerifyChecksums(c.pub, c.tag, c.manifest, c.sig); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

// signedFixture serves a release over a mock transport and counts package downloads.
type signedFixture struct {
	asset     []byte
	checksums []byte
	sig       []byte
	downloads atomic.Int32
}

func (f *signedFixture) service(pub ed25519.PublicKey) *Service {
	svc := NewService("2.1.0", func(string) error { return nil })
	svc.client = &http.Client{Transport: mockRoundTripper(func(r *http.Request) (*http.Response, error) {
		var body []byte
		switch r.URL.Path {
		case "/download/asset":
			f.downloads.Add(1)
			body = f.asset
		case "/download/checksums":
			body = f.checksums
		case "/download/sig":
			body = f.sig
		}
		if body == nil {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("404"))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body))}, nil
	})}
	svc.updateAsset = Asset{
		Name:               "DeckTap-v2.2.0-macOS-Universal.tar.gz",
		Size:               int64(len(f.asset)),
		BrowserDownloadURL: "https://example.com/download/asset",
	}
	svc.checksumsURL = "https://example.com/download/checksums"
	svc.signatureURL = "https://example.com/download/sig"
	svc.publicKey = pub
	svc.status.LatestVer = "v2.2.0"
	return svc
}

func TestSelfUpdateFailsClosed(t *testing.T) {
	pub, priv := testKey(t)
	_, otherPriv := testKey(t)
	asset := []byte("package-bytes")
	sum := sha256.Sum256(asset)
	manifest := []byte(fmt.Sprintf("%s  DeckTap-v2.2.0-macOS-Universal.tar.gz\n", hex.EncodeToString(sum[:])))
	otherManifest := []byte(fmt.Sprintf("%s  DeckTap-v2.2.0-windows-amd64.zip\n", hex.EncodeToString(sum[:])))

	cases := []struct {
		name            string
		setup           func(f *signedFixture, svc *Service)
		want            string
		downloadAllowed bool
	}{
		{"no signature asset", func(f *signedFixture, svc *Service) { svc.signatureURL = "" }, "no signed checksums", false},
		{"no checksums asset", func(f *signedFixture, svc *Service) { svc.checksumsURL = "" }, "no signed checksums", false},
		{"signature missing on server", func(f *signedFixture, svc *Service) { f.sig = nil }, "status 404", false},
		{"signed by another key", func(f *signedFixture, svc *Service) {
			f.sig = SignChecksums(otherPriv, "v2.2.0", manifest)
		}, "signature is invalid", false},
		{"older release replayed as newer tag", func(f *signedFixture, svc *Service) {
			f.sig = SignChecksums(priv, "v2.1.9", manifest)
		}, "signature is invalid", false},
		{"manifest edited after signing", func(f *signedFixture, svc *Service) {
			f.checksums = append(append([]byte{}, manifest...), "deadbeef  extra.zip\n"...)
		}, "signature is invalid", false},
		{"asset not listed in signed manifest", func(f *signedFixture, svc *Service) {
			f.checksums = otherManifest
			f.sig = SignChecksums(priv, "v2.2.0", otherManifest)
		}, "not listed in the signed checksums", false},
		{"package tampered", func(f *signedFixture, svc *Service) { f.asset = []byte("evil-bytes") }, "checksum mismatch", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &signedFixture{asset: asset, checksums: manifest, sig: SignChecksums(priv, "v2.2.0", manifest)}
			svc := f.service(pub)
			c.setup(f, svc)
			var mu sync.Mutex
			replaced := false
			svc.SetReplaceBinaryHandler(func(string, []byte) error { mu.Lock(); replaced = true; mu.Unlock(); return nil })

			if err := svc.StartSelfUpdate(context.Background()); err != nil {
				t.Fatal(err)
			}
			st := waitForState(t, svc, "error")
			if !strings.Contains(st.Error, c.want) {
				t.Fatalf("error %q does not mention %q", st.Error, c.want)
			}
			mu.Lock()
			defer mu.Unlock()
			if replaced {
				t.Fatal("binary must not be replaced")
			}
			if !c.downloadAllowed && f.downloads.Load() != 0 {
				t.Fatal("package must not be downloaded before the release is authenticated")
			}
		})
	}
}

func TestSelfUpdateWithoutSigningKeyOpensReleasePage(t *testing.T) {
	f := &signedFixture{asset: []byte("x")}
	svc := f.service(nil)
	var opened string
	svc.openURL = func(u string) error { opened = u; return nil }
	svc.status.ReleaseURL = "https://example.com/releases/tag/v2.2.0"

	if err := svc.StartSelfUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := svc.Status(); st.State != "error" || !strings.Contains(st.Error, ErrNoSigningKey.Error()) {
		t.Fatalf("unexpected status %+v", st)
	}
	if opened == "" {
		t.Fatal("expected the release page to open for a manual download")
	}
	if f.downloads.Load() != 0 {
		t.Fatal("nothing may be downloaded without a signing key")
	}
}

func waitForState(t *testing.T, svc *Service, state string) Status {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if st := svc.Status(); st.State == state {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	st := svc.Status()
	t.Fatalf("timed out waiting for state %q, got %q (%s)", state, st.State, st.Error)
	return st
}
