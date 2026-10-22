// Package tunnel manages public HTTPS tunnels for DeckTap.
package tunnel

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
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
	ngrokURLRe = regexp.MustCompile(`https://[a-zA-Z0-9.-]+\.ngrok[a-z0-9.-]*`)
)

// ErrNgrokNotInstalled is returned when no ngrok binary can be found or downloaded.
var ErrNgrokNotInstalled = errors.New("ngrok not found (macOS: brew install ngrok · Windows: winget install ngrok · or https://ngrok.com/download)")

// FindNgrokBinary looks for ngrok in PATH, explicit path, or common install locations.
func FindNgrokBinary(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("ngrok at %s: %w", explicit, err)
		}
		return explicit, nil
	}
	if p, err := exec.LookPath("ngrok"); err == nil {
		return p, nil
	}
	var candidates []string
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			filepath.Join(exeDir, "ngrok"),
			filepath.Join(exeDir, "..", "Resources", "ngrok"),
			"/opt/homebrew/bin/ngrok", "/usr/local/bin/ngrok",
		}
	case "windows":
		pf := os.Getenv("ProgramFiles")
		pf86 := os.Getenv("ProgramFiles(x86)")
		candidates = []string{
			filepath.Join(exeDir, "ngrok.exe"),
			filepath.Join(pf, "ngrok", "ngrok.exe"),
			filepath.Join(pf86, "ngrok", "ngrok.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WinGet", "Links", "ngrok.exe"),
		}
	default:
		candidates = []string{
			filepath.Join(exeDir, "ngrok"),
			"/usr/local/bin/ngrok",
			"/usr/bin/ngrok",
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}

	// Check local user cache directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	binName := "ngrok"
	if runtime.GOOS == "windows" {
		binName = "ngrok.exe"
	}
	cacheDir := filepath.Join(homeDir, ".decktap", "bin")
	cachedPath := filepath.Join(cacheDir, binName)

	if st, err := os.Stat(cachedPath); err == nil && !st.IsDir() {
		return cachedPath, nil
	}

	// Download official ngrok release if not installed
	if err := os.MkdirAll(cacheDir, 0755); err == nil {
		if err := downloadNgrokRelease(cachedPath); err == nil {
			_ = os.Chmod(cachedPath, 0755)
			return cachedPath, nil
		}
	}

	return "", ErrNgrokNotInstalled
}

func downloadNgrokRelease(targetPath string) error {
	var downloadURL string
	var isZip bool

	switch runtime.GOOS {
	case "darwin":
		isZip = true
		switch runtime.GOARCH {
		case "arm64":
			downloadURL = "https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-darwin-arm64.zip"
		default:
			downloadURL = "https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-darwin-amd64.zip"
		}
	case "windows":
		isZip = true
		switch runtime.GOARCH {
		case "386":
			downloadURL = "https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-windows-386.zip"
		default:
			downloadURL = "https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-windows-amd64.zip"
		}
	case "linux":
		isZip = false
		switch runtime.GOARCH {
		case "arm64":
			downloadURL = "https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-arm64.tgz"
		default:
			downloadURL = "https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-amd64.tgz"
		}
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "DeckTap/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ngrok download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ngrok download responded with status: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	tempFile := targetPath + ".tmp"
	defer os.Remove(tempFile)

	targetExeName := "ngrok"
	if runtime.GOOS == "windows" {
		targetExeName = "ngrok.exe"
	}

	if isZip {
		zr, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
		if err != nil {
			return err
		}
		found := false
		for _, f := range zr.File {
			if filepath.Base(f.Name) == targetExeName {
				rc, err := f.Open()
				if err != nil {
					return err
				}
				out, err := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
				if err != nil {
					rc.Close()
					return err
				}
				_, err = io.Copy(out, rc)
				rc.Close()
				out.Close()
				if err != nil {
					return err
				}
				found = true
				break
			}
		}
		if !found {
			return errors.New("zip archive did not contain ngrok executable")
		}
	} else {
		gzr, err := gzip.NewReader(bytes.NewReader(bodyBytes))
		if err != nil {
			return err
		}
		defer gzr.Close()
		tr := tar.NewReader(gzr)
		found := false
		for {
			header, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			if filepath.Base(header.Name) == targetExeName {
				out, err := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
				if err != nil {
					return err
				}
				_, err = io.Copy(out, tr)
				out.Close()
				if err != nil {
					return err
				}
				found = true
				break
			}
		}
		if !found {
			return errors.New("tar archive did not contain ngrok executable")
		}
	}

	return os.Rename(tempFile, targetPath)
}

// StartNgrok launches ngrok and blocks until the public HTTPS URL is active or ctx expires.
func StartNgrok(ctx context.Context, binary string, localPort int, authToken string, log *slog.Logger) (*Tunnel, error) {
	args := []string{
		"http",
		fmt.Sprintf("http://127.0.0.1:%d", localPort),
		"--log=stdout",
		"--log-format=json",
	}
	if authToken != "" {
		args = append(args, "--authtoken", authToken)
	}

	cmd := exec.Command(binary, args...)
	procutil.Hide(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = nil
	cmd.WaitDelay = 2 * time.Second

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ngrok: %w", err)
	}

	t := &Tunnel{cmd: cmd, done: make(chan struct{}), Provider: "ngrok"}
	urlCh := make(chan string, 1)

	var lastMu sync.Mutex
	var lastLine string
	var authErr string

	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			log.Debug("ngrok", "line", line)

			lastMu.Lock()
			lastLine = line
			lastMu.Unlock()

			// Check structured JSON output
			var logEntry struct {
				URL string `json:"url"`
				Msg string `json:"msg"`
				Err string `json:"err"`
			}
			if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
				if logEntry.URL != "" && strings.HasPrefix(logEntry.URL, "https://") {
					select {
					case urlCh <- logEntry.URL:
					default:
					}
				}
				if strings.Contains(line, "ERR_NGROK_4018") || strings.Contains(logEntry.Err, "authtoken") || strings.Contains(logEntry.Msg, "authtoken") {
					authErr = "ngrok requires an authtoken: specify via --ngrok-token, DECKTAP_NGROK_TOKEN, or run 'ngrok config add-authtoken <token>' (get one free at https://dashboard.ngrok.com/get-started/your-authtoken)"
				}
			}

			// Fallback: regex search
			for _, u := range ngrokURLRe.FindAllString(line, -1) {
				select {
				case urlCh <- u:
				default:
				}
			}
		}
	}()

	go func() {
		err := cmd.Wait()
		if err == nil {
			err = errors.New("ngrok exited")
		}
		time.Sleep(50 * time.Millisecond)
		lastMu.Lock()
		if authErr != "" {
			err = errors.New(authErr)
		} else if lastLine != "" {
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
		return nil, fmt.Errorf("ngrok did not publish a URL: %w", ctx.Err())
	}

	t.Host = strings.TrimPrefix(t.URL, "https://")
	t.Host = strings.TrimRight(t.Host, "/")

	log.Info("ngrok Port 443 tunnel online", "url", t.URL)
	return t, nil
}
