package updater

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultRepoOwner and DefaultRepoName define the GitHub repository.
const (
	DefaultRepoOwner = "silva324"
	DefaultRepoName  = "DeckTap"
)

// Asset represents a downloadable release asset from GitHub.
type Asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// GitHubRelease represents the GitHub API release response.
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

// Status is the state exposed to the UI and tray.
type Status struct {
	Checked      bool   `json:"checked"`
	Available    bool   `json:"available"`
	CurrentVer   string `json:"currentVersion"`
	LatestVer    string `json:"latestVersion"`
	ReleaseTitle string `json:"releaseTitle"`
	ReleaseURL   string `json:"releaseURL"`
	ReleaseNotes string `json:"releaseNotes"`
	DownloadURL  string `json:"downloadURL"`
	AssetName    string `json:"assetName"`
	AssetSize    int64  `json:"assetSize"`
	State        string `json:"state,omitempty"`    // "idle", "downloading", "verifying", "applying", "restarting", "error"
	Progress     int    `json:"progress,omitempty"` // 0-100
	Error        string `json:"error,omitempty"`
}

// Service manages checking and applying updates.
type Service struct {
	owner          string
	repo           string
	currentVersion string
	client         *http.Client
	openURL        func(string) error
	relaunch       func() error
	replaceBinary  func(string, []byte) error
	onRestart      func() error
	publicKey      ed25519.PublicKey // nil: self-update disabled

	mu           sync.RWMutex
	status       Status
	lastChecked  time.Time
	cachedETag   string
	updateAsset  Asset
	checksumsURL string
	signatureURL string
}

// OpenURL opens the specified URL in the system default web browser.
func OpenURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// NewService creates a new updater service.
func NewService(currentVersion string, openURL func(string) error) *Service {
	if openURL == nil {
		openURL = OpenURL
	}
	pub, _ := ParsePublicKey(ReleasePublicKey) // empty or invalid: self-update stays disabled
	return &Service{
		owner:          DefaultRepoOwner,
		repo:           DefaultRepoName,
		currentVersion: currentVersion,
		client:         &http.Client{Timeout: 15 * time.Second},
		openURL:        openURL,
		publicKey:      pub,
		status: Status{
			CurrentVer: currentVersion,
			State:      "idle",
		},
	}
}

// SetRestartHandler registers a callback invoked before process restart.
func (s *Service) SetRestartHandler(fn func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onRestart = fn
}

// SetRelaunchHandler overrides the relaunch executor (useful for testing).
func (s *Service) SetRelaunchHandler(fn func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.relaunch = fn
}

// SetReplaceBinaryHandler overrides the binary replacement routine (useful for testing).
func (s *Service) SetReplaceBinaryHandler(fn func(string, []byte) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replaceBinary = fn
}

// Status returns the current cached update status.
func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// InvalidateCache clears the cached ETag to force a full re-fetch on the next check.
func (s *Service) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedETag = ""
}

// Check queries GitHub Releases for the latest version.
func (s *Service) Check(ctx context.Context) (Status, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", s.owner, s.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return s.recordError(err)
	}
	req.Header.Set("User-Agent", fmt.Sprintf("DeckTap-Updater/%s", s.currentVersion))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	s.mu.RLock()
	etag := s.cachedETag
	s.mu.RUnlock()
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return s.recordError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		s.mu.Lock()
		s.lastChecked = time.Now()
		s.status.Checked = true
		st := s.status
		s.mu.Unlock()
		return st, nil
	}

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("github api status: %d", resp.StatusCode)
		return s.recordError(err)
	}

	newETag := resp.Header.Get("ETag")

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return s.recordError(err)
	}

	var checksumsURL, signatureURL string
	for _, a := range rel.Assets {
		switch {
		case strings.EqualFold(a.Name, ChecksumsAssetName):
			checksumsURL = a.BrowserDownloadURL
		case strings.EqualFold(a.Name, SignatureAssetName):
			signatureURL = a.BrowserDownloadURL
		}
	}

	available := IsNewer(rel.TagName, s.currentVersion)
	matchedAsset, _ := SelectAsset(rel.Assets, runtime.GOOS, runtime.GOARCH)
	updateAsset, _ := SelectUpdateAsset(rel.Assets, runtime.GOOS, runtime.GOARCH)

	downloadURL := ""
	assetName := ""
	var assetSize int64
	if updateAsset.BrowserDownloadURL != "" {
		downloadURL = updateAsset.BrowserDownloadURL
		assetName = updateAsset.Name
		assetSize = updateAsset.Size
	} else if matchedAsset.BrowserDownloadURL != "" {
		downloadURL = matchedAsset.BrowserDownloadURL
		assetName = matchedAsset.Name
		assetSize = matchedAsset.Size
	}

	s.mu.Lock()
	currentState := s.status.State
	if currentState == "" {
		currentState = "idle"
	}
	s.updateAsset = updateAsset
	if s.updateAsset.BrowserDownloadURL == "" {
		s.updateAsset = matchedAsset
	}
	s.checksumsURL = checksumsURL
	s.signatureURL = signatureURL

	st := Status{
		Checked:      true,
		Available:    available,
		CurrentVer:   s.currentVersion,
		LatestVer:    rel.TagName,
		ReleaseTitle: rel.Name,
		ReleaseURL:   rel.HTMLURL,
		ReleaseNotes: rel.Body,
		DownloadURL:  downloadURL,
		AssetName:    assetName,
		AssetSize:    assetSize,
		State:        currentState,
		Progress:     s.status.Progress,
		Error:        s.status.Error,
	}

	s.status = st
	if newETag != "" {
		s.cachedETag = newETag
	}
	s.lastChecked = time.Now()
	s.mu.Unlock()

	return st, nil
}

// StartBackgroundCheck starts a periodic background check loop.
func (s *Service) StartBackgroundCheck(ctx context.Context, interval time.Duration) {
	// Initial check after short delay so app startup is instant
	time.AfterFunc(2*time.Second, func() {
		_, _ = s.Check(ctx)
	})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = s.Check(ctx)
			}
		}
	}()
}

// ApplyUpdate starts the automated self-update workflow.
func (s *Service) ApplyUpdate() error {
	return s.StartSelfUpdate(context.Background())
}

// OpenReleasePage opens the release page in the default web browser.
func (s *Service) OpenReleasePage() error {
	st := s.Status()
	targetURL := st.ReleaseURL
	if targetURL == "" {
		targetURL = fmt.Sprintf("https://github.com/%s/%s/releases", s.owner, s.repo)
	}
	if s.openURL != nil {
		return s.openURL(targetURL)
	}
	return nil
}

func (s *Service) recordError(err error) (Status, error) {
	s.mu.Lock()
	s.status.Checked = true
	s.status.Error = err.Error()
	st := s.status
	s.mu.Unlock()
	return st, err
}

// CleanVersion strips leading 'v' or 'V' and trims whitespace.
func CleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// SplitPre splits version string into base and prerelease (e.g. 2.1.0-rc.1 -> 2.1.0, rc.1).
func SplitPre(v string) (base, pre string) {
	if idx := strings.IndexByte(v, '-'); idx != -1 {
		return v[:idx], v[idx+1:]
	}
	return v, ""
}

// Compare returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func Compare(v1, v2 string) int {
	v1 = CleanVersion(v1)
	v2 = CleanVersion(v2)

	base1, pre1 := SplitPre(v1)
	base2, pre2 := SplitPre(v2)

	parts1 := strings.Split(base1, ".")
	parts2 := strings.Split(base2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}
		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	// Base versions are equal. A final release (no prerelease) is newer than a prerelease
	if pre1 == "" && pre2 != "" {
		return 1
	}
	if pre1 != "" && pre2 == "" {
		return -1
	}
	if pre1 != "" && pre2 != "" {
		if pre1 < pre2 {
			return -1
		}
		if pre1 > pre2 {
			return 1
		}
	}

	return 0
}

// IsNewer reports whether latest is strictly newer than current.
func IsNewer(latest, current string) bool {
	return Compare(latest, current) > 0
}

// SelectAsset finds the best matching asset for the target OS and Architecture.
func SelectAsset(assets []Asset, goos, goarch string) (Asset, bool) {
	var preferredExt []string
	switch goos {
	case "darwin":
		preferredExt = []string{".dmg", ".tar.gz", ".zip"}
	case "windows":
		preferredExt = []string{".exe", ".zip"}
	case "linux":
		preferredExt = []string{".tar.gz", ".zip"}
	default:
		preferredExt = []string{".tar.gz", ".zip"}
	}

	for _, ext := range preferredExt {
		for _, a := range assets {
			nameLower := strings.ToLower(a.Name)
			if !strings.HasSuffix(nameLower, ext) {
				continue
			}

			switch goos {
			case "darwin":
				if strings.Contains(nameLower, "macos") || strings.Contains(nameLower, "darwin") {
					return a, true
				}
			case "windows":
				if strings.Contains(nameLower, "windows") || strings.Contains(nameLower, "win") {
					return a, true
				}
			case "linux":
				if strings.Contains(nameLower, "linux") {
					if goarch == "arm64" && strings.Contains(nameLower, "arm64") {
						return a, true
					}
					if (goarch == "amd64" || goarch == "x86_64") && (strings.Contains(nameLower, "amd64") || strings.Contains(nameLower, "x86_64")) {
						return a, true
					}
				}
			}
		}
	}

	// Fallback to any asset mentioning the OS
	for _, a := range assets {
		nameLower := strings.ToLower(a.Name)
		if strings.Contains(nameLower, goos) {
			return a, true
		}
	}

	return Asset{}, false
}
