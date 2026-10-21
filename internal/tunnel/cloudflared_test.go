package tunnel

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// A fake cloudflared that prints the same lines as the real one.
const fake = `#!/bin/sh
echo "INF Requesting new quick Tunnel on trycloudflare.com..." >&2
echo "ERR failed to reach https://api.trycloudflare.com once" >&2
echo "INF |  https://swift-fox-89.trycloudflare.com                                   |" >&2
sleep 0.2
echo "INF Registered tunnel connection connIndex=0 location=lis01 protocol=quic" >&2
sleep 30
`

func TestStartParsesURLAndStops(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake")
	}
	bin := filepath.Join(t.TempDir(), "cloudflared")
	os.WriteFile(bin, []byte(fake), 0o755)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tn, err := Start(ctx, bin, 1234, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if tn.URL != "https://swift-fox-89.trycloudflare.com" || tn.Host != "swift-fox-89.trycloudflare.com" {
		t.Fatalf("url %q", tn.URL)
	}
	tn.Stop()
	select {
	case <-tn.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("process not killed")
	}
}

func TestStartFailsWhenProcessExits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake")
	}
	bin := filepath.Join(t.TempDir(), "cloudflared")
	os.WriteFile(bin, []byte("#!/bin/sh\necho 'ERR no network' >&2\nexit 1\n"), 0o755)
	_, err := Start(context.Background(), bin, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("expected error")
	}
}
