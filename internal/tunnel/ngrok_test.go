package tunnel

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const fakeNgrokSuccess = `#!/bin/sh
echo '{"lvl":"info","msg":"client session established","id":"123"}'
sleep 0.1
echo '{"lvl":"info","msg":"started tunnel","obj":"tunnels","name":"command_line","addr":"http://localhost:8080","url":"https://swift-fox-89.ngrok-free.app"}'
sleep 30
`

const fakeNgrokAuthFail = `#!/bin/sh
echo '{"lvl":"err","msg":"authentication failed: an authtoken is required","err":"ERR_NGROK_4018"}'
exit 1
`

func TestStartNgrokParsesURLAndStops(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake")
	}
	bin := filepath.Join(t.TempDir(), "ngrok")
	if err := os.WriteFile(bin, []byte(fakeNgrokSuccess), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tn, err := StartNgrok(ctx, bin, 8080, "test-token", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if tn.URL != "https://swift-fox-89.ngrok-free.app" || tn.Host != "swift-fox-89.ngrok-free.app" {
		t.Fatalf("unexpected url %q or host %q", tn.URL, tn.Host)
	}
	if tn.Provider != "ngrok" {
		t.Fatalf("unexpected provider %q", tn.Provider)
	}
	tn.Stop()

	select {
	case <-tn.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("ngrok process not killed")
	}
}

func TestStartNgrokAuthTokenError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake")
	}
	bin := filepath.Join(t.TempDir(), "ngrok")
	if err := os.WriteFile(bin, []byte(fakeNgrokAuthFail), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := StartNgrok(ctx, bin, 8080, "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("expected auth error")
	}
	if !strings.Contains(err.Error(), "authtoken") {
		t.Fatalf("expected authtoken error message, got %v", err)
	}
}
