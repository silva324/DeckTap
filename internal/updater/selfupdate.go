package updater

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// TargetBinaryNames returns candidate executable names within archives for the given OS and architecture.
func TargetBinaryNames(goos, goarch string) []string {
	switch goos {
	case "darwin":
		return []string{"decktap-universal", "DeckTap", "decktap"}
	case "windows":
		return []string{"DeckTap.exe", "decktap.exe"}
	case "linux":
		return []string{"decktap-linux-" + goarch, "DeckTap", "decktap"}
	default:
		return []string{"DeckTap", "decktap"}
	}
}

// SelectUpdateAsset finds the best matching extractable archive asset for automated self-update.
// It prefers .tar.gz for macOS/Linux (containing the universal or arch binary) and .zip for Windows.
func SelectUpdateAsset(assets []Asset, goos, goarch string) (Asset, bool) {
	var preferredExt []string
	switch goos {
	case "darwin":
		preferredExt = []string{".tar.gz", ".zip"}
	case "windows":
		preferredExt = []string{".zip"}
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

	return Asset{}, false
}

// ParseChecksums parses the content of a checksums.txt file (standard sha256sum format: "<hash>  <filename>").
func ParseChecksums(content string) map[string]string {
	result := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hash := strings.ToLower(parts[0])
			filename := filepath.Base(strings.TrimPrefix(parts[1], "*"))
			result[filename] = hash
		}
	}
	return result
}

// ExtractBinaryFromTarGz decompresses a .tar.gz archive and extracts the binary matching one of targetNames.
func ExtractBinaryFromTarGz(r io.Reader, targetNames []string) ([]byte, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip decompression error: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var singleFileData []byte
	fileCount := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar reading error: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(hdr.Name)
		for _, target := range targetNames {
			if strings.EqualFold(base, target) {
				return io.ReadAll(tr)
			}
		}
		if fileCount == 0 {
			singleFileData, _ = io.ReadAll(tr)
		}
		fileCount++
	}

	if fileCount == 1 && len(singleFileData) > 0 {
		return singleFileData, nil
	}

	return nil, fmt.Errorf("no matching executable found in tar.gz archive")
}

// ExtractBinaryFromZip extracts a binary matching one of targetNames from a zip archive.
func ExtractBinaryFromZip(zipBytes []byte, targetNames []string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("zip reading error: %w", err)
	}
	var singleFileData []byte
	fileCount := 0

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		for _, target := range targetNames {
			if strings.EqualFold(base, target) {
				rc, err := f.Open()
				if err != nil {
					return nil, fmt.Errorf("opening zip entry %s: %w", f.Name, err)
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		if fileCount == 0 {
			if rc, err := f.Open(); err == nil {
				singleFileData, _ = io.ReadAll(rc)
				rc.Close()
			}
		}
		fileCount++
	}

	if fileCount == 1 && len(singleFileData) > 0 {
		return singleFileData, nil
	}

	return nil, fmt.Errorf("no matching executable found in zip archive")
}

// ReplaceBinary atomically replaces the executable at targetPath with newBytes.
// If targetPath is empty, it locates the currently running executable via os.Executable().
func ReplaceBinary(targetPath string, newBytes []byte) error {
	if targetPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to locate current executable: %w", err)
		}
		targetPath, err = filepath.EvalSymlinks(exe)
		if err != nil {
			return fmt.Errorf("failed to resolve symlink for executable: %w", err)
		}
	}

	dir := filepath.Dir(targetPath)
	tempFile, err := os.CreateTemp(dir, ".decktap_update_*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file in %s: %w", dir, err)
	}
	tempPath := tempFile.Name()

	success := false
	defer func() {
		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(newBytes); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to write update content: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to sync update content: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close update file: %w", err)
	}

	// Make executable
	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	if runtime.GOOS == "windows" {
		oldPath := targetPath + ".old"
		_ = os.Remove(oldPath) // remove previous old file if present
		if err := os.Rename(targetPath, oldPath); err != nil {
			return fmt.Errorf("failed to rename existing Windows executable: %w", err)
		}
		if err := os.Rename(tempPath, targetPath); err != nil {
			_ = os.Rename(oldPath, targetPath) // rollback
			return fmt.Errorf("failed to replace Windows executable: %w", err)
		}
	} else {
		// POSIX atomic rename replaces file entry even if process is currently executing it
		if err := os.Rename(tempPath, targetPath); err != nil {
			return fmt.Errorf("failed to atomically rename executable: %w", err)
		}
	}

	success = true
	return nil
}

// CleanupOldExecutables removes leftover .old binaries from previous updates (Windows).
func CleanupOldExecutables() {
	if runtime.GOOS != "windows" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return
	}
	oldPath := exe + ".old"
	_ = os.Remove(oldPath)
}

// Relaunch restarts DeckTap with the newly installed executable.
func Relaunch() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate executable for relaunch: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("failed to resolve executable symlinks: %w", err)
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" && strings.Contains(exe, ".app/Contents/MacOS/") {
		idx := strings.Index(exe, ".app/Contents/MacOS/")
		bundlePath := exe[:idx+4] // includes ".app"
		cmd = exec.Command("open", "-n", bundlePath)
	} else {
		cmd = exec.Command(exe, os.Args[1:]...)
	}

	cmd.Dir = filepath.Dir(exe)
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to spawn new DeckTap process: %w", err)
	}
	return nil
}

// StartSelfUpdate begins the asynchronous download, verification, replacement, and relaunch flow.
func (s *Service) StartSelfUpdate(ctx context.Context) error {
	s.mu.Lock()
	if s.status.State == "downloading" || s.status.State == "verifying" || s.status.State == "applying" || s.status.State == "restarting" {
		s.mu.Unlock()
		return fmt.Errorf("update already in progress")
	}
	asset := s.updateAsset
	checksumsURL := s.checksumsURL
	s.status.State = "downloading"
	s.status.Progress = 0
	s.status.Error = ""
	s.mu.Unlock()

	if asset.BrowserDownloadURL == "" && s.status.DownloadURL != "" {
		asset.BrowserDownloadURL = s.status.DownloadURL
		asset.Name = s.status.AssetName
		asset.Size = s.status.AssetSize
	}
	if asset.BrowserDownloadURL == "" {
		s.setError("no compatible update package found for this platform")
		return s.OpenReleasePage()
	}

	go s.runSelfUpdate(ctx, asset, checksumsURL)
	return nil
}

func (s *Service) runSelfUpdate(ctx context.Context, asset Asset, checksumsURL string) {
	// 1. Download asset with progress reporting
	tempFile, err := os.CreateTemp("", "decktap_asset_*")
	if err != nil {
		s.setError(fmt.Sprintf("failed to create temp file: %v", err))
		return
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		s.setError(fmt.Sprintf("download request failed: %v", err))
		return
	}
	req.Header.Set("User-Agent", fmt.Sprintf("DeckTap-Updater/%s", s.currentVersion))

	resp, err := s.client.Do(req)
	if err != nil {
		s.setError(fmt.Sprintf("download failed: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.setError(fmt.Sprintf("download failed with status %d", resp.StatusCode))
		return
	}

	contentLength := resp.ContentLength
	if contentLength <= 0 {
		contentLength = asset.Size
	}

	hasher := sha256.New()
	mw := io.MultiWriter(tempFile, hasher)

	buf := make([]byte, 32*1024)
	var downloaded int64
	lastReportedPct := -1

	for {
		select {
		case <-ctx.Done():
			s.setError("update cancelled")
			return
		default:
		}

		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := mw.Write(buf[:n]); wErr != nil {
				s.setError(fmt.Sprintf("write error: %v", wErr))
				return
			}
			downloaded += int64(n)
			if contentLength > 0 {
				pct := int((downloaded * 70) / contentLength) // 0-70% allocated for download
				if pct != lastReportedPct {
					lastReportedPct = pct
					s.setProgress(pct)
				}
			}
		}
		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			s.setError(fmt.Sprintf("download error: %v", rErr))
			return
		}
	}
	_ = tempFile.Close()

	// 2. Verifying checksum
	s.setState("verifying", 75)
	if checksumsURL != "" {
		if err := s.verifyChecksum(ctx, checksumsURL, asset.Name, hasher.Sum(nil)); err != nil {
			s.setError(fmt.Sprintf("integrity verification failed: %v", err))
			return
		}
	}

	// 3. Extracting binary
	s.setState("applying", 85)
	rawBytes, err := os.ReadFile(tempPath)
	if err != nil {
		s.setError(fmt.Sprintf("failed to read downloaded archive: %v", err))
		return
	}

	var extractedBinary []byte
	nameLower := strings.ToLower(asset.Name)
	targetNames := TargetBinaryNames(runtime.GOOS, runtime.GOARCH)

	if strings.HasSuffix(nameLower, ".tar.gz") || strings.HasSuffix(nameLower, ".tgz") {
		extractedBinary, err = ExtractBinaryFromTarGz(bytes.NewReader(rawBytes), targetNames)
	} else if strings.HasSuffix(nameLower, ".zip") {
		extractedBinary, err = ExtractBinaryFromZip(rawBytes, targetNames)
	} else {
		extractedBinary = rawBytes
	}

	if err != nil {
		s.setError(fmt.Sprintf("failed to extract binary: %v", err))
		return
	}

	// 4. Replacing binary
	s.setProgress(92)
	replaceFn := s.replaceBinary
	if replaceFn == nil {
		replaceFn = ReplaceBinary
	}
	if err := replaceFn("", extractedBinary); err != nil {
		s.setError(fmt.Sprintf("failed to install update: %v", err))
		return
	}

	// 5. Restarting
	s.setState("restarting", 100)

	relaunchFn := s.relaunch
	if relaunchFn == nil {
		relaunchFn = Relaunch
	}
	if err := relaunchFn(); err != nil {
		s.setError(fmt.Sprintf("update installed but restart failed: %v", err))
		return
	}

	if s.onRestart != nil {
		_ = s.onRestart()
	}
}

func (s *Service) verifyChecksum(ctx context.Context, checksumsURL, assetName string, actualSum []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumsURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("DeckTap-Updater/%s", s.currentVersion))
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksums download returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	sums := ParseChecksums(string(body))
	expectedHex, exists := sums[assetName]
	if !exists {
		for k, v := range sums {
			if strings.EqualFold(k, assetName) {
				expectedHex = v
				exists = true
				break
			}
		}
	}
	if !exists {
		return nil // asset not explicitly listed in checksums.txt, continue
	}

	actualHex := hex.EncodeToString(actualSum)
	if !strings.EqualFold(actualHex, expectedHex) {
		return fmt.Errorf("checksum mismatch (expected %s, got %s)", expectedHex, actualHex)
	}
	return nil
}

func (s *Service) setProgress(pct int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Progress = pct
}

func (s *Service) setState(state string, pct int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.State = state
	s.status.Progress = pct
}

func (s *Service) setError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.State = "error"
	s.status.Error = msg
}
