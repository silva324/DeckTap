// Package presenter abstracts the presentation application (PowerPoint on macOS or
// Windows, or an in-memory demo). Implementations must be safe for concurrent use.
package presenter

import (
	"context"
	"errors"
	"strings"

	"pptx-remote/internal/pptx"
)

// DeckInfo represents an open presentation deck.
type DeckInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Total   int    `json:"total"`
	Active  bool   `json:"active"`
	Showing bool   `json:"showing"`
}

// Status is what the presentation app reports right now.
type Status struct {
	AppRunning bool
	DeckName   string
	DeckPath   string // local path or URL (OneDrive/SharePoint); "" if unknown
	Total      int
	Presenting bool
	Slide      int    // 1-based; 0 = unknown
	SlideTitle string // live title of current slide from PowerPoint
	SlideNotes string // live speaker notes of current slide from PowerPoint
	Blackout   bool
	Ended      bool // "end of slide show" screen
	Decks      []DeckInfo
}

// Controller drives a presentation application.
type Controller interface {
	Name() string
	Status(ctx context.Context) (Status, error)
	// AppSlides reads titles/notes through the app (used when the file is not local,
	// e.g. a SharePoint URL). May return ErrUnsupported.
	AppSlides(ctx context.Context) ([]pptx.Slide, error)
	Start(ctx context.Context, from int) error
	End(ctx context.Context) error
	Next(ctx context.Context) error
	Prev(ctx context.Context) error
	Goto(ctx context.Context, n int) error
	ToggleBlack(ctx context.Context) error
	SwitchDeck(ctx context.Context, name string, startShow bool) error
	// Diagnose returns human-readable self-test lines (used by `slidecue doctor`).
	Diagnose(ctx context.Context) []string
	Close()
}

var (
	ErrUnsupported = errors.New("not supported by this controller")
	ErrNoShow      = errors.New("slide show is not running")
	ErrNoDeck      = errors.New("no presentation is open in PowerPoint")
	ErrNotRunning  = errors.New("PowerPoint is not running")
)

// CleanText normalises PowerPoint line endings (\r paragraphs, \v soft breaks)
// and scrubs AppleScript missing value tokens.
func CleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\v", "\n")
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "missing value") || strings.EqualFold(s, "<missing value>") {
		return ""
	}
	return s
}
