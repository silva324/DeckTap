//go:build windows

package presenter

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
	"unicode/utf16"

	"pptx-remote/internal/pptx"
	"pptx-remote/internal/procutil"
)

//go:embed bridge.ps1
var bridgeScript string

// Windows drives PowerPoint through COM using a long-lived Windows PowerShell 5.1
// process (bridge.ps1). This keeps the Go binary free of cgo/COM bindings while
// giving full fidelity: real slide position, animations, black screen, hidden slides
// and decks opened from OneDrive/SharePoint.
type Windows struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan string
	nextID int
}

func NewPlatform() (Controller, error) { return &Windows{}, nil }

func (w *Windows) Name() string { return "powerpoint-windows" }

func encodedCommand(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(u))
	for i, r := range u {
		binary.LittleEndian.PutUint16(b[2*i:], r)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func (w *Windows) startLocked() error {
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-STA",
		"-ExecutionPolicy", "Bypass", "-EncodedCommand", encodedCommand(bridgeScript))
	procutil.Hide(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start PowerShell bridge: %w", err)
	}
	lines := make(chan string, 4)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1<<20), 16<<20)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
		cmd.Wait()
	}()
	w.cmd, w.stdin, w.lines = cmd, stdin, lines
	return nil
}

func (w *Windows) killLocked() {
	if w.cmd != nil && w.cmd.Process != nil {
		w.cmd.Process.Kill()
	}
	w.cmd, w.stdin, w.lines = nil, nil, nil
}

type bridgeResp struct {
	ID    int             `json:"id"`
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

func (w *Windows) call(ctx context.Context, cmd string, n int) (json.RawMessage, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cmd == nil {
		if err := w.startLocked(); err != nil {
			return nil, err
		}
	}
	w.nextID++
	id := w.nextID
	req, _ := json.Marshal(map[string]any{"id": id, "cmd": cmd, "n": n})
	if _, err := w.stdin.Write(append(req, '\n')); err != nil {
		w.killLocked()
		return nil, fmt.Errorf("bridge write: %w", err)
	}
	timeout := 8 * time.Second
	if cmd == "slides" {
		timeout = 60 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case line, ok := <-w.lines:
			if !ok {
				w.killLocked()
				return nil, errors.New("PowerShell bridge exited")
			}
			var r bridgeResp
			if json.Unmarshal([]byte(line), &r) != nil || r.ID != id {
				continue // stray output or a late answer to a timed-out request
			}
			if !r.OK {
				return nil, errors.New(r.Error)
			}
			return r.Data, nil
		case <-timer.C:
			w.killLocked() // a hung COM call: restart the bridge next time
			return nil, fmt.Errorf("PowerPoint did not answer %q in time", cmd)
		case <-ctx.Done():
			w.killLocked()
			return nil, ctx.Err()
		}
	}
}

func (w *Windows) Status(ctx context.Context) (Status, error) {
	raw, err := w.call(ctx, "status", 0)
	if err != nil {
		return Status{}, err
	}
	var s struct {
		App     bool       `json:"app"`
		Name    string     `json:"name"`
		Path    string     `json:"path"`
		Total   int        `json:"total"`
		Showing bool       `json:"showing"`
		Idx     int        `json:"idx"`
		Notes   string     `json:"notes"`
		Title   string     `json:"title"`
		Black   bool       `json:"black"`
		Done    bool       `json:"done"`
		Decks   []DeckInfo `json:"decks"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return Status{}, err
	}
	return Status{AppRunning: s.App, DeckName: s.Name, DeckPath: s.Path, Total: s.Total,
		Presenting: s.Showing, Slide: s.Idx, SlideTitle: CleanText(s.Title), SlideNotes: CleanText(s.Notes),
		Blackout: s.Black, Ended: s.Done, Decks: s.Decks}, nil
}

func (w *Windows) AppSlides(ctx context.Context) ([]pptx.Slide, error) {
	raw, err := w.call(ctx, "slides", 0)
	if err != nil {
		return nil, err
	}
	type item struct {
		N      int    `json:"n"`
		Title  string `json:"title"`
		Notes  string `json:"notes"`
		Hidden bool   `json:"hidden"`
	}
	var items []item
	if err := json.Unmarshal(raw, &items); err != nil { // PowerShell may unwrap 1-element arrays
		var one item
		if err2 := json.Unmarshal(raw, &one); err2 != nil {
			return nil, err
		}
		items = []item{one}
	}
	out := make([]pptx.Slide, len(items))
	for i, it := range items {
		out[i] = pptx.Slide{Index: it.N, Title: CleanText(it.Title), Notes: CleanText(it.Notes), Hidden: it.Hidden}
	}
	return out, nil
}

func (w *Windows) do(ctx context.Context, cmd string, n int) error {
	_, err := w.call(ctx, cmd, n)
	return err
}

func (w *Windows) Start(ctx context.Context, from int) error { return w.do(ctx, "start", from) }
func (w *Windows) End(ctx context.Context) error             { return w.do(ctx, "end", 0) }
func (w *Windows) Next(ctx context.Context) error            { return w.do(ctx, "next", 0) }
func (w *Windows) Prev(ctx context.Context) error            { return w.do(ctx, "prev", 0) }
func (w *Windows) Goto(ctx context.Context, n int) error     { return w.do(ctx, "goto", n) }
func (w *Windows) ToggleBlack(ctx context.Context) error     { return w.do(ctx, "black", 0) }

func (w *Windows) SwitchDeck(ctx context.Context, name string, startShow bool) error {
	arg := 0
	if startShow {
		arg = 1
	}
	_, err := w.call(ctx, "switch:"+name, arg)
	return err
}

func (w *Windows) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stdin != nil {
		w.stdin.Close()
	}
	w.killLocked()
}

func (w *Windows) Diagnose(ctx context.Context) []string {
	var out []string
	st, err := w.Status(ctx)
	if err != nil {
		return []string{"FAIL PowerShell/COM bridge: " + err.Error()}
	}
	out = append(out, "OK   PowerShell/COM bridge")
	if !st.AppRunning {
		return append(out, "INFO PowerPoint is not running - open a presentation and run doctor again")
	}
	out = append(out, fmt.Sprintf("OK   active presentation %q (%d slides) path=%q", st.DeckName, st.Total, st.DeckPath))
	if st.Presenting {
		out = append(out, fmt.Sprintf("OK   slide show running, slide %d, black=%v ended=%v", st.Slide, st.Blackout, st.Ended))
	}
	slides, err := w.AppSlides(ctx)
	if err != nil {
		out = append(out, "FAIL notes via COM: "+err.Error())
	} else {
		out = append(out, fmt.Sprintf("OK   notes via COM: %d slides", len(slides)))
	}
	return out
}
