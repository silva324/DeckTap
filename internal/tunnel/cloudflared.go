// Package tunnel manages an ephemeral cloudflared "quick tunnel" (SPEC-SEC-001 §2).
//
// Lifecycle: spawn `cloudflared tunnel --url http://127.0.0.1:<port> --no-autoupdate`,
// scan stderr for the https://<name>.trycloudflare.com URL, wait for the first
// "Registered tunnel connection" so the URL is actually routable, keep draining stderr
// (a full pipe would block cloudflared), and kill the process on Stop.
package tunnel

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"pptx-remote/internal/procutil"
)

var (
	urlRe        = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
	registeredRe = regexp.MustCompile(`(?i)registered tunnel connection`)
)

// ErrNotInstalled is returned when no cloudflared binary can be found.
var ErrNotInstalled = errors.New("cloudflared not found (macOS: brew install cloudflared · Windows: winget install --id Cloudflare.cloudflared)")

// FindBinary looks in PATH and in the usual install locations. GUI apps on macOS do
// not inherit the shell PATH, so Homebrew locations are checked explicitly.
func FindBinary(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("cloudflared at %s: %w", explicit, err)
		}
		return explicit, nil
	}
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p, nil
	}
	var candidates []string
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			filepath.Join(exeDir, "cloudflared"),
			filepath.Join(exeDir, "..", "Resources", "cloudflared"), // inside DeckTap.app
			"/opt/homebrew/bin/cloudflared", "/usr/local/bin/cloudflared",
		}
	case "windows":
		pf := os.Getenv("ProgramFiles")
		pf86 := os.Getenv("ProgramFiles(x86)")
		candidates = []string{
			filepath.Join(exeDir, "cloudflared.exe"),
			filepath.Join(pf, "cloudflared", "cloudflared.exe"),
			filepath.Join(pf86, "cloudflared", "cloudflared.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WinGet", "Links", "cloudflared.exe"),
		}
	default:
		candidates = []string{filepath.Join(exeDir, "cloudflared"), "/usr/local/bin/cloudflared", "/usr/bin/cloudflared"}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}

	// Autonomous dependency management: download official release if not installed
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	binName := "cloudflared"
	if runtime.GOOS == "windows" {
		binName = "cloudflared.exe"
	}
	cacheDir := filepath.Join(homeDir, ".decktap", "bin")
	cachedPath := filepath.Join(cacheDir, binName)

	if st, err := os.Stat(cachedPath); err == nil && !st.IsDir() {
		return cachedPath, nil
	}

	// Legacy cache directory fallback
	legacyPath := filepath.Join(homeDir, ".pptx-remote", "bin", binName)
	if st, err := os.Stat(legacyPath); err == nil && !st.IsDir() {
		return legacyPath, nil
	}

	if err := os.MkdirAll(cacheDir, 0755); err == nil {
		if err := downloadRelease(cachedPath); err == nil {
			_ = os.Chmod(cachedPath, 0755)
			return cachedPath, nil
		}
	}

	return "", ErrNotInstalled
}

func downloadRelease(targetPath string) error {
	var assetName string
	var isTgz bool

	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			assetName = "cloudflared-darwin-arm64.tgz"
		case "amd64":
			assetName = "cloudflared-darwin-amd64.tgz"
		default:
			return fmt.Errorf("unsupported darwin architecture: %s", runtime.GOARCH)
		}
		isTgz = true
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			assetName = "cloudflared-windows-amd64.exe"
		case "386":
			assetName = "cloudflared-windows-386.exe"
		default:
			return fmt.Errorf("unsupported windows architecture: %s", runtime.GOARCH)
		}
		isTgz = false
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			assetName = "cloudflared-linux-amd64"
		case "arm64":
			assetName = "cloudflared-linux-arm64"
		default:
			return fmt.Errorf("unsupported linux architecture: %s", runtime.GOARCH)
		}
		isTgz = false
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	downloadURL := fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/latest/download/%s", assetName)

	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "DeckTap/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server responded with status: %s", resp.Status)
	}

	tempFile := targetPath + ".tmp"
	defer os.Remove(tempFile)

	if isTgz {
		gzr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer gzr.Close()

		tarReader := tar.NewReader(gzr)
		extracted := false
		for {
			header, err := tarReader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			if filepath.Base(header.Name) == "cloudflared" {
				out, err := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
				if err != nil {
					return err
				}
				if _, err := io.Copy(out, tarReader); err != nil {
					out.Close()
					return err
				}
				out.Close()
				extracted = true
				break
			}
		}
		if !extracted {
			return errors.New("archive did not contain 'cloudflared'")
		}
	} else {
		out, err := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, resp.Body); err != nil {
			out.Close()
			return err
		}
		out.Close()
	}

	return os.Rename(tempFile, targetPath)
}

// Start launches cloudflared and blocks until the public URL is routable or ctx expires.
func Start(ctx context.Context, binary string, localPort int, log *slog.Logger) (*Tunnel, error) {
	cmd := exec.Command(binary, "tunnel", "--url", fmt.Sprintf("http://127.0.0.1:%d", localPort), "--no-autoupdate")
	procutil.Hide(cmd)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = nil // /dev/null; a Go writer here would make Wait block on grandchildren
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start cloudflared: %w", err)
	}
	t := &Tunnel{cmd: cmd, done: make(chan struct{}), Provider: "cloudflared"}
	urlCh := make(chan string, 1)
	readyCh := make(chan struct{}, 1)

	var lastMu sync.Mutex
	var lastLine string
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			log.Debug("cloudflared", "line", line)
			lastMu.Lock()
			lastLine = line
			lastMu.Unlock()
			for _, u := range urlRe.FindAllString(line, -1) {
				if !strings.HasPrefix(u, "https://api.") { // API endpoint shows up in error logs
					select {
					case urlCh <- u:
					default:
					}
				}
			}
			if registeredRe.MatchString(line) {
				select {
				case readyCh <- struct{}{}:
				default:
				}
			}
		}
	}()
	go func() {
		// Wait independently of the scanner: it closes the pipe once the process exits,
		// even if a grandchild still holds stderr open.
		err := cmd.Wait()
		if err == nil {
			err = errors.New("cloudflared exited")
		}
		time.Sleep(50 * time.Millisecond) // let the scanner record the final line
		lastMu.Lock()
		if lastLine != "" {
			err = fmt.Errorf("%w: %s", err, lastLine)
		}
		lastMu.Unlock()
		t.exitErr = err
		close(t.done)
	}()

	select {
	case t.URL = <-urlCh:
	case <-t.done:
		return nil, t.exitErr
	case <-ctx.Done():
		t.Stop()
		return nil, fmt.Errorf("cloudflared did not publish a URL: %w", ctx.Err())
	}
	t.Host = strings.TrimPrefix(t.URL, "https://")

	select { // the URL is printed before the edge connection exists
	case <-readyCh:
	case <-t.done:
		return nil, t.exitErr
	case <-ctx.Done():
		t.Stop()
		return nil, fmt.Errorf("cloudflared did not connect: %w", ctx.Err())
	}
	// Give Cloudflare's DNS a moment so the phone does not cache an NXDOMAIN.
	select {
	case <-time.After(1500 * time.Millisecond):
	case <-t.done:
		return nil, t.exitErr
	}
	return t, nil
}
