package presenter

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/silva324/DeckTap/internal/pptx"
)

// Demo is an in-memory controller backed by a .pptx file. Used for UI work, demos and tests.
type Demo struct {
	mu      sync.Mutex
	path    string
	total   int
	idx     int
	showing bool
	black   bool
	ended   bool
	slides  []pptx.Slide
}

func NewDemo(path string) (*Demo, error) {
	slides, err := pptx.Read(path)
	if err != nil {
		return nil, err
	}
	abs, _ := filepath.Abs(path)
	return &Demo{path: abs, total: len(slides), idx: 1, slides: slides}, nil
}

func (d *Demo) Name() string { return "demo" }
func (d *Demo) Close()       {}

func (d *Demo) Status(context.Context) (Status, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	deckName := filepath.Base(d.path)
	var title, notes string
	if d.idx > 0 && d.idx <= len(d.slides) {
		title = d.slides[d.idx-1].Title
		notes = d.slides[d.idx-1].Notes
	}
	return Status{
		AppRunning: true, DeckName: deckName, DeckPath: d.path, Total: d.total,
		Presenting: d.showing, Slide: d.idx, SlideTitle: title, SlideNotes: notes,
		Blackout: d.black, Ended: d.ended,
		Decks: []DeckInfo{
			{Name: deckName, Path: d.path, Total: d.total, Active: true, Showing: d.showing},
		},
	}, nil
}

func (d *Demo) AppSlides(context.Context) ([]pptx.Slide, error) { return nil, ErrUnsupported }

func (d *Demo) Start(_ context.Context, from int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.showing, d.ended, d.black = true, false, false
	d.idx = clamp(from, 1, d.total)
	return nil
}

func (d *Demo) running() error {
	if !d.showing {
		return ErrNoShow
	}
	return nil
}

func (d *Demo) End(context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.showing, d.ended, d.black = false, false, false
	return nil
}

func (d *Demo) Next(context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.running(); err != nil {
		return err
	}
	d.black = false
	if d.idx >= d.total {
		d.ended = true
	} else {
		d.idx++
	}
	return nil
}

func (d *Demo) Prev(context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.running(); err != nil {
		return err
	}
	d.black = false
	if d.ended {
		d.ended = false
	} else if d.idx > 1 {
		d.idx--
	}
	return nil
}

func (d *Demo) Goto(_ context.Context, n int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.running(); err != nil {
		return err
	}
	d.ended, d.idx = false, clamp(n, 1, d.total)
	return nil
}

func (d *Demo) ToggleBlack(context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.running(); err != nil {
		return err
	}
	d.black = !d.black
	return nil
}

func (d *Demo) SwitchDeck(_ context.Context, _ string, _ bool) error {
	return nil
}

func (d *Demo) Diagnose(context.Context) []string {
	return []string{"OK   demo controller using " + d.path}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
