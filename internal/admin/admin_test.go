package admin

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAdminPageGuarded(t *testing.T) {
	resets := 0
	s, err := Start(func() View {
		return View{Headline: "Running", PairingURL: "https://x.trycloudflare.com/#tok.key"}
	}, func() { resets++ })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	resp, err := http.Get(s.URL())
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("skipping live socket test: network dial not permitted in sandbox: %v", err)
		}
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), "<svg") || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("page: %d", resp.StatusCode)
	}
	base := s.URL()[:strings.Index(s.URL()[7:], "/")+7]
	if r, _ := http.Get(base + "/"); r.StatusCode != 404 {
		t.Fatal("page must require the path secret")
	}
	req, _ := http.NewRequest("GET", s.URL(), nil)
	req.Host = "attacker.example" // DNS rebinding
	if r, _ := http.DefaultClient.Do(req); r.StatusCode != 403 {
		t.Fatal("foreign Host must be rejected")
	}
	if r, _ := http.Get(s.URL() + "reset"); r.StatusCode != 405 || resets != 0 {
		t.Fatal("reset must be POST")
	}
	req, _ = http.NewRequest("POST", s.URL()+"reset", nil)
	req.Header.Set("Origin", "https://evil.example")
	if r, _ := http.DefaultClient.Do(req); r.StatusCode != 403 || resets != 0 {
		t.Fatal("cross-origin reset must fail")
	}
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if r, _ := noRedirect.Post(s.URL()+"reset", "", nil); r.StatusCode != 303 || resets != 1 {
		t.Fatal("reset failed")
	}

	applied := false
	s2, err := Start(func() View {
		return View{Headline: "Running"}
	}, func() {}, func() error {
		applied = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	// Test status endpoint
	resStatus, err := http.Get(s2.URL() + "status")
	if err != nil || resStatus.StatusCode != 200 {
		t.Fatalf("status endpoint failed: %v", err)
	}
	statusBytes, _ := io.ReadAll(resStatus.Body)
	if !strings.Contains(string(statusBytes), `"update"`) {
		t.Fatalf("expected update field in status: %s", string(statusBytes))
	}

	// Test update apply endpoint
	resApply, err := http.Post(s2.URL()+"update/apply", "application/json", nil)
	if err != nil || resApply.StatusCode != 200 || !applied {
		t.Fatalf("update/apply endpoint failed: applied=%v, err=%v", applied, err)
	}

	// Test quit endpoint
	quitCh := make(chan struct{}, 1)
	s2.SetQuitHandler(func() {
		select {
		case quitCh <- struct{}{}:
		default:
		}
	})
	resQuit, err := http.Post(s2.URL()+"quit", "application/json", nil)
	if err != nil || resQuit.StatusCode != 200 {
		t.Fatalf("quit endpoint failed: %v", err)
	}
	select {
	case <-quitCh:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("expected quit handler to be called")
	}

	// Test deck/reload endpoint
	reloaded := false
	s2.SetReloadDeckHandler(func() {
		reloaded = true
	})
	resReload, err := http.Post(s2.URL()+"deck/reload", "application/json", nil)
	if err != nil || resReload.StatusCode != 200 || !reloaded {
		t.Fatalf("deck/reload endpoint failed: reloaded=%v, err=%v", reloaded, err)
	}
}

func TestFontsServedBehindSecret(t *testing.T) {
	s, err := Start(func() View { return View{} }, func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	resp, err := http.Get(s.URL() + "fonts/outfit-latin.woff2")
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("skipping live socket test: network dial not permitted in sandbox: %v", err)
		}
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "font/woff2" || !strings.HasPrefix(string(body), "wOF2") {
		t.Fatalf("font: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	base := s.URL()[:strings.Index(s.URL()[7:], "/")+7]
	for _, path := range []string{
		base + "/fonts/outfit-latin.woff2",       // no path secret
		s.URL() + "fonts/OFL-Outfit.txt",         // not a font
		s.URL() + "fonts/..%2Findex.html",        // traversal
		s.URL() + "fonts/../fonts/missing.woff2", // unknown
	} {
		if r, _ := http.Get(path); r.StatusCode != 404 {
			t.Errorf("%s: got %d, want 404", path, r.StatusCode)
		}
	}
}
