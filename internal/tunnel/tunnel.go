package tunnel

import (
	"os/exec"
	"sync"
	"time"
)

// Tunnel represents one active tunnel process (Cloudflare quick tunnel).
type Tunnel struct {
	URL      string
	Host     string
	Provider string

	cmd      *exec.Cmd
	done     chan struct{}
	exitErr  error
	stopOnce sync.Once
}

// Done is closed when the underlying tunnel process exits.
func (t *Tunnel) Done() <-chan struct{} { return t.done }

// Err returns the exit error once Done is closed.
func (t *Tunnel) Err() error { return t.exitErr }

// Stop terminates the tunnel process (idempotent) and waits briefly for exit.
func (t *Tunnel) Stop() {
	t.stopOnce.Do(func() {
		if t.cmd != nil && t.cmd.Process != nil {
			_ = t.cmd.Process.Kill()
		}
		select {
		case <-t.done:
		case <-time.After(3 * time.Second):
		}
	})
}
