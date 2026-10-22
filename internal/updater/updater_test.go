package updater

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		wantComp int
		isNewer  bool
	}{
		{"2.1.0", "2.0.0", 1, true},
		{"v2.1.0", "2.0.0", 1, true},
		{"2.0.1", "2.0.0", 1, true},
		{"2.0.0", "2.0.0", 0, false},
		{"v2.0.0", "2.0.0", 0, false},
		{"1.9.9", "2.0.0", -1, false},
		{"2.1.0-rc.1", "2.0.0", 1, true},
		{"2.0.0", "2.0.0-rc.1", 1, true},
		{"2.0.0-rc.1", "2.0.0", -1, false},
		{"2.0.0-rc.2", "2.0.0-rc.1", 1, true},
		{"3.0.0", "2.9.9", 1, true},
	}

	for _, tt := range tests {
		gotComp := Compare(tt.v1, tt.v2)
		if gotComp != tt.wantComp {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.v1, tt.v2, gotComp, tt.wantComp)
		}
		gotNewer := IsNewer(tt.v1, tt.v2)
		if gotNewer != tt.isNewer {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.v1, tt.v2, gotNewer, tt.isNewer)
		}
	}
}

func TestSelectAsset(t *testing.T) {
	assets := []Asset{
		{Name: "DeckTap-2.1.0-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux-amd64"},
		{Name: "DeckTap-2.1.0-linux-arm64.tar.gz", BrowserDownloadURL: "https://example.com/linux-arm64"},
		{Name: "DeckTap-2.1.0-macOS-Universal.dmg", BrowserDownloadURL: "https://example.com/macOS-dmg"},
		{Name: "DeckTap-2.1.0-macOS-Universal.tar.gz", BrowserDownloadURL: "https://example.com/macOS-tar"},
		{Name: "DeckTap-2.1.0-windows-amd64.zip", BrowserDownloadURL: "https://example.com/windows-zip"},
	}

	// macOS
	a, ok := SelectAsset(assets, "darwin", "arm64")
	if !ok || a.Name != "DeckTap-2.1.0-macOS-Universal.dmg" {
		t.Errorf("darwin expected DMG, got %v (%v)", a, ok)
	}

	// Windows
	a, ok = SelectAsset(assets, "windows", "amd64")
	if !ok || a.Name != "DeckTap-2.1.0-windows-amd64.zip" {
		t.Errorf("windows expected ZIP, got %v (%v)", a, ok)
	}

	// Linux ARM64
	a, ok = SelectAsset(assets, "linux", "arm64")
	if !ok || a.Name != "DeckTap-2.1.0-linux-arm64.tar.gz" {
		t.Errorf("linux arm64 expected arm64 tarball, got %v (%v)", a, ok)
	}

	// Linux AMD64
	a, ok = SelectAsset(assets, "linux", "amd64")
	if !ok || a.Name != "DeckTap-2.1.0-linux-amd64.tar.gz" {
		t.Errorf("linux amd64 expected amd64 tarball, got %v (%v)", a, ok)
	}
}

type mockRoundTripper func(*http.Request) (*http.Response, error)

func (m mockRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return m(r)
}

func TestServiceCheckMock(t *testing.T) {
	mockBody := `{
		"tag_name": "v2.1.0",
		"name": "DeckTap v2.1.0",
		"html_url": "https://github.com/silva324/DeckTap/releases/tag/v2.1.0",
		"body": "Awesome new features!",
		"assets": [
			{
				"name": "DeckTap-2.1.0-macOS-Universal.dmg",
				"size": 12345,
				"browser_download_url": "https://github.com/silva324/DeckTap/releases/download/v2.1.0/DeckTap-2.1.0-macOS-Universal.dmg"
			}
		]
	}`

	var openedURL string
	svc := NewService("2.0.0", func(u string) error {
		openedURL = u
		return nil
	})
	svc.client = &http.Client{
		Transport: mockRoundTripper(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(mockBody)),
			}, nil
		}),
	}

	if !IsNewer("v2.1.0", svc.currentVersion) {
		t.Fatal("expected v2.1.0 to be newer than 2.0.0")
	}

	svc.status = Status{
		Checked:      true,
		Available:    true,
		CurrentVer:   "2.0.0",
		LatestVer:    "v2.1.0",
		ReleaseTitle: "DeckTap v2.1.0",
		DownloadURL:  "https://github.com/silva324/DeckTap/releases/download/v2.1.0/DeckTap-2.1.0-macOS-Universal.dmg",
	}

	st := svc.Status()
	if !st.Available {
		t.Fatal("expected update available")
	}

	if err := svc.OpenReleasePage(); err != nil {
		t.Fatal(err)
	}
	if openedURL != "https://github.com/silva324/DeckTap/releases" {
		t.Fatalf("expected opened release page URL, got %q", openedURL)
	}
}

func TestServiceCheckETagConditional(t *testing.T) {
	mockBody := `{
		"tag_name": "v0.9.0",
		"name": "DeckTap v0.9.0",
		"html_url": "https://github.com/silva324/DeckTap/releases/tag/v0.9.0",
		"body": "Release 0.9.0",
		"assets": []
	}`

	callCount := 0
	var lastIfNoneMatch string

	svc := NewService("0.8.0", nil)
	svc.client = &http.Client{
		Transport: mockRoundTripper(func(r *http.Request) (*http.Response, error) {
			callCount++
			lastIfNoneMatch = r.Header.Get("If-None-Match")

			if lastIfNoneMatch == `"etag-test-123"` {
				return &http.Response{
					StatusCode: http.StatusNotModified,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}

			h := make(http.Header)
			h.Set("Content-Type", "application/json")
			h.Set("ETag", `"etag-test-123"`)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     h,
				Body:       io.NopCloser(strings.NewReader(mockBody)),
			}, nil
		}),
	}

	// First check: should fetch 200 OK and cache ETag
	st1, err := svc.Check(t.Context())
	if err != nil {
		t.Fatalf("first check failed: %v", err)
	}
	if !st1.Available || st1.LatestVer != "v0.9.0" {
		t.Fatalf("expected update available v0.9.0, got %+v", st1)
	}
	if lastIfNoneMatch != "" {
		t.Fatalf("expected empty If-None-Match on first check, got %q", lastIfNoneMatch)
	}
	if svc.cachedETag != `"etag-test-123"` {
		t.Fatalf("expected cachedETag to be stored, got %q", svc.cachedETag)
	}

	// Second check: should send If-None-Match and receive 304 Not Modified
	st2, err := svc.Check(t.Context())
	if err != nil {
		t.Fatalf("second check failed: %v", err)
	}
	if lastIfNoneMatch != `"etag-test-123"` {
		t.Fatalf("expected If-None-Match header, got %q", lastIfNoneMatch)
	}
	if !st2.Available || st2.LatestVer != "v0.9.0" {
		t.Fatalf("expected cached status preserved on 304, got %+v", st2)
	}

	// Test InvalidateCache()
	svc.InvalidateCache()
	if svc.cachedETag != "" {
		t.Fatalf("expected empty cachedETag after InvalidateCache()")
	}
}
