package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSelectUpdateAsset(t *testing.T) {
	assets := []Asset{
		{Name: "DeckTap-2.1.0-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux-amd64.tar.gz"},
		{Name: "DeckTap-2.1.0-linux-arm64.tar.gz", BrowserDownloadURL: "https://example.com/linux-arm64.tar.gz"},
		{Name: "DeckTap-2.1.0-macOS-Universal.dmg", BrowserDownloadURL: "https://example.com/macOS.dmg"},
		{Name: "DeckTap-2.1.0-macOS-Universal.tar.gz", BrowserDownloadURL: "https://example.com/macOS-Universal.tar.gz"},
		{Name: "DeckTap-2.1.0-windows-amd64.zip", BrowserDownloadURL: "https://example.com/windows-amd64.zip"},
		{Name: "DeckTap-2.1.0-Setup-Windows-x64.exe", BrowserDownloadURL: "https://example.com/windows-setup.exe"},
	}

	// macOS: must select .tar.gz (NOT .dmg)
	a, ok := SelectUpdateAsset(assets, "darwin", "arm64")
	if !ok || a.Name != "DeckTap-2.1.0-macOS-Universal.tar.gz" {
		t.Fatalf("darwin expected .tar.gz, got %v (%v)", a, ok)
	}

	// Windows: must select .zip (NOT Setup.exe)
	a, ok = SelectUpdateAsset(assets, "windows", "amd64")
	if !ok || a.Name != "DeckTap-2.1.0-windows-amd64.zip" {
		t.Fatalf("windows expected .zip, got %v (%v)", a, ok)
	}

	// Linux AMD64
	a, ok = SelectUpdateAsset(assets, "linux", "amd64")
	if !ok || a.Name != "DeckTap-2.1.0-linux-amd64.tar.gz" {
		t.Fatalf("linux amd64 expected amd64 tarball, got %v (%v)", a, ok)
	}

	// Linux ARM64
	a, ok = SelectUpdateAsset(assets, "linux", "arm64")
	if !ok || a.Name != "DeckTap-2.1.0-linux-arm64.tar.gz" {
		t.Fatalf("linux arm64 expected arm64 tarball, got %v (%v)", a, ok)
	}
}

func TestParseChecksums(t *testing.T) {
	raw := `
# Release checksums
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  DeckTap-v2.1.0-macOS-Universal.tar.gz
ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad *DeckTap-v2.1.0-windows-amd64.zip
`
	sums := ParseChecksums(raw)
	if len(sums) != 2 {
		t.Fatalf("expected 2 checksum entries, got %d", len(sums))
	}
	if sums["DeckTap-v2.1.0-macOS-Universal.tar.gz"] != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("wrong checksum for macOS asset")
	}
	if sums["DeckTap-v2.1.0-windows-amd64.zip"] != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("wrong checksum for Windows asset")
	}
}

func createTarGz(t *testing.T, filename string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: filename,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func createZip(t *testing.T, filename string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	f, err := zw.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractBinaryFromTarGz(t *testing.T) {
	expectedBytes := []byte("fake-decktap-mach-o-binary")
	tarGzBytes := createTarGz(t, "decktap-universal", expectedBytes)

	extracted, err := ExtractBinaryFromTarGz(bytes.NewReader(tarGzBytes), []string{"decktap-universal", "DeckTap"})
	if err != nil {
		t.Fatalf("ExtractBinaryFromTarGz failed: %v", err)
	}
	if !bytes.Equal(extracted, expectedBytes) {
		t.Fatalf("extracted bytes mismatch: got %q, want %q", extracted, expectedBytes)
	}
}

func TestExtractBinaryFromZip(t *testing.T) {
	expectedBytes := []byte("fake-decktap-pe-windows-binary")
	zipBytes := createZip(t, "DeckTap.exe", expectedBytes)

	extracted, err := ExtractBinaryFromZip(zipBytes, []string{"DeckTap.exe", "decktap.exe"})
	if err != nil {
		t.Fatalf("ExtractBinaryFromZip failed: %v", err)
	}
	if !bytes.Equal(extracted, expectedBytes) {
		t.Fatalf("extracted bytes mismatch: got %q, want %q", extracted, expectedBytes)
	}
}

func TestReplaceBinary(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "decktap_target_bin")

	initialContent := []byte("version-1-initial-binary")
	if err := os.WriteFile(targetPath, initialContent, 0755); err != nil {
		t.Fatal(err)
	}

	updatedContent := []byte("version-2-updated-binary")
	if err := ReplaceBinary(targetPath, updatedContent); err != nil {
		t.Fatalf("ReplaceBinary failed: %v", err)
	}

	readBytes, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readBytes, updatedContent) {
		t.Fatalf("target binary content was not updated: got %q, want %q", readBytes, updatedContent)
	}
}

func TestSelfUpdateFullWorkflow(t *testing.T) {
	var newBinaryContent []byte
	switch runtime.GOOS {
	case "darwin":
		newBinaryContent = append([]byte{0xca, 0xfe, 0xba, 0xbe}, []byte("-decktap-v2.2.0-binary-payload")...)
	case "windows":
		newBinaryContent = append([]byte{'M', 'Z'}, []byte("-decktap-v2.2.0-binary-payload")...)
	default:
		newBinaryContent = append([]byte{0x7f, 'E', 'L', 'F'}, []byte("-decktap-v2.2.0-binary-payload")...)
	}
	assetContent := createTarGz(t, "decktap-universal", newBinaryContent)

	sum := sha256.Sum256(assetContent)
	sumHex := hex.EncodeToString(sum[:])
	checksumsFile := fmt.Sprintf("%s  DeckTap-v2.2.0-macOS-Universal.tar.gz\n", sumHex)

	var replacedBinary []byte
	var relaunchCalled bool
	var restartCallbackCalled bool
	var mu sync.Mutex

	svc := NewService("2.1.0", nil)

	// Mock HTTP client to serve release asset & checksums
	svc.client = &http.Client{
		Transport: mockRoundTripper(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/download/asset":
				return &http.Response{
					StatusCode:    http.StatusOK,
					Header:        http.Header{"Content-Length": []string{fmt.Sprintf("%d", len(assetContent))}},
					Body:          io.NopCloser(bytes.NewReader(assetContent)),
					ContentLength: int64(len(assetContent)),
				}, nil
			case "/download/checksums":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(checksumsFile)),
				}, nil
			default:
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("404"))}, nil
			}
		}),
	}

	svc.updateAsset = Asset{
		Name:               "DeckTap-v2.2.0-macOS-Universal.tar.gz",
		Size:               int64(len(assetContent)),
		BrowserDownloadURL: "https://example.com/download/asset",
	}
	svc.checksumsURL = "https://example.com/download/checksums"

	svc.SetReplaceBinaryHandler(func(target string, newBytes []byte) error {
		mu.Lock()
		replacedBinary = newBytes
		mu.Unlock()
		return nil
	})

	svc.SetRelaunchHandler(func() error {
		mu.Lock()
		relaunchCalled = true
		mu.Unlock()
		return nil
	})

	svc.SetRestartHandler(func() error {
		mu.Lock()
		restartCallbackCalled = true
		mu.Unlock()
		return nil
	})

	// Execute self update
	if err := svc.StartSelfUpdate(context.Background()); err != nil {
		t.Fatalf("StartSelfUpdate failed: %v", err)
	}

	// Poll until restarting state or timeout
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st := svc.Status()
		if st.State == "restarting" {
			break
		}
		if st.State == "error" {
			t.Fatalf("update failed with error: %s", st.Error)
		}
		time.Sleep(20 * time.Millisecond)
	}

	st := svc.Status()
	if st.State != "restarting" {
		t.Fatalf("expected state 'restarting', got %q (error: %s)", st.State, st.Error)
	}

	mu.Lock()
	defer mu.Unlock()

	if !bytes.Equal(replacedBinary, newBinaryContent) {
		t.Fatalf("replaced binary content mismatch: got %q, want %q", replacedBinary, newBinaryContent)
	}
	if !relaunchCalled {
		t.Fatal("expected relaunch handler to be invoked")
	}
	if !restartCallbackCalled {
		t.Fatal("expected restart callback to be invoked")
	}
}

func TestSelfUpdateChecksumMismatch(t *testing.T) {
	assetContent := []byte("tampered-asset-bytes")
	wrongChecksumsFile := "0000000000000000000000000000000000000000000000000000000000000000  DeckTap-v2.2.0-macOS-Universal.tar.gz\n"

	svc := NewService("2.1.0", nil)
	svc.client = &http.Client{
		Transport: mockRoundTripper(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/download/asset":
				return &http.Response{
					StatusCode:    http.StatusOK,
					Body:          io.NopCloser(bytes.NewReader(assetContent)),
					ContentLength: int64(len(assetContent)),
				}, nil
			case "/download/checksums":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(wrongChecksumsFile)),
				}, nil
			default:
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("404"))}, nil
			}
		}),
	}

	svc.updateAsset = Asset{
		Name:               "DeckTap-v2.2.0-macOS-Universal.tar.gz",
		Size:               int64(len(assetContent)),
		BrowserDownloadURL: "https://example.com/download/asset",
	}
	svc.checksumsURL = "https://example.com/download/checksums"

	var replaceCalled bool
	svc.SetReplaceBinaryHandler(func(target string, newBytes []byte) error {
		replaceCalled = true
		return nil
	})

	if err := svc.StartSelfUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st := svc.Status()
		if st.State == "error" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	st := svc.Status()
	if st.State != "error" {
		t.Fatalf("expected state 'error', got %q", st.State)
	}
	if !strings.Contains(st.Error, "checksum mismatch") {
		t.Fatalf("expected error mentioning checksum mismatch, got %q", st.Error)
	}
	if replaceCalled {
		t.Fatal("replaceBinary must NOT be called when checksum fails")
	}
}

func TestValidateExecutable(t *testing.T) {
	// Rejection of HTML error pages
	htmlPayload := []byte("<!DOCTYPE html><html><head><title>Release v0.5.0</title></head><body>404 Not Found</body></html>")
	if err := ValidateExecutable("darwin", htmlPayload); err == nil {
		t.Fatal("expected ValidateExecutable to reject HTML document on darwin")
	}
	if err := ValidateExecutable("windows", htmlPayload); err == nil {
		t.Fatal("expected ValidateExecutable to reject HTML document on windows")
	}
	if err := ValidateExecutable("linux", htmlPayload); err == nil {
		t.Fatal("expected ValidateExecutable to reject HTML document on linux")
	}

	// Rejection of tiny payload
	if err := ValidateExecutable("darwin", []byte("a")); err == nil {
		t.Fatal("expected error on tiny payload")
	}

	// Valid Mach-O FAT / 64-bit headers
	fatHeader := []byte{0xca, 0xfe, 0xba, 0xbe, 0x00, 0x00}
	if err := ValidateExecutable("darwin", fatHeader); err != nil {
		t.Fatalf("expected valid darwin FAT binary, got: %v", err)
	}

	macho64Header := []byte{0xcf, 0xfa, 0xed, 0xfe, 0x00, 0x00}
	if err := ValidateExecutable("darwin", macho64Header); err != nil {
		t.Fatalf("expected valid darwin Mach-O 64-bit binary, got: %v", err)
	}

	// Valid Windows PE header
	peHeader := []byte{'M', 'Z', 0x90, 0x00, 0x03}
	if err := ValidateExecutable("windows", peHeader); err != nil {
		t.Fatalf("expected valid Windows PE binary, got: %v", err)
	}

	// Valid Linux ELF header
	elfHeader := []byte{0x7f, 'E', 'L', 'F', 0x02}
	if err := ValidateExecutable("linux", elfHeader); err != nil {
		t.Fatalf("expected valid Linux ELF binary, got: %v", err)
	}
}
