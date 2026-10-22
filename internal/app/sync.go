// Package app wires the presentation controller, the secure session hub, the HTTP
// servers and the tunnel together.
package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"pptx-remote/internal/pptx"
	"pptx-remote/internal/presenter"
	"pptx-remote/internal/session"
)

// Sync keeps the phone in step with the presentation app: it polls the controller,
// caches the deck outline (titles/notes) and pushes STATE_UPDATE/DECK frames.
type Sync struct {
	ctl      presenter.Controller
	log      *slog.Logger
	interval time.Duration
	send     func(map[string]any) error
	version  string

	pollMu      sync.Mutex // one poll at a time, so state frames are never sent out of order
	mu          sync.Mutex
	st          presenter.Status
	deck        []pptx.Slide
	deckKey     string // path|mtime|total
	deckChecked time.Time
	showStarted time.Time
	lastErr     string
	lastState   string
	deckSig     string
	pollNow     chan struct{}
	onChange    func()
}

func NewSync(ctl presenter.Controller, log *slog.Logger, interval time.Duration) *Sync {
	return &Sync{ctl: ctl, log: log, interval: interval, pollNow: make(chan struct{}, 1)}
}

// Run polls until ctx is cancelled.
func (s *Sync) Run(ctx context.Context) {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		s.poll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-s.pollNow:
		}
	}
}

func (s *Sync) kick() {
	select {
	case s.pollNow <- struct{}{}:
	default:
	}
}

func (s *Sync) poll(ctx context.Context) {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	st, err := s.ctl.Status(cctx)
	cancel()
	if err != nil {
		s.log.Debug("status failed", "err", err)
		s.mu.Lock()
		if s.lastErr == "" {
			s.lastErr = err.Error()
		}
		s.mu.Unlock()
	}
	s.refreshDeck(ctx, st)

	s.mu.Lock()
	changedShow := st.Presenting != s.st.Presenting || st.DeckPath != s.st.DeckPath
	if st.Presenting && (s.showStarted.IsZero() || changedShow) {
		s.showStarted = time.Now()
	}
	if !st.Presenting {
		s.showStarted = time.Time{}
	}
	if err == nil && s.lastErr != "" && !strings.HasPrefix(s.lastErr, "!") {
		s.lastErr = "" // status errors clear themselves; command errors ("!") clear on next command
	}
	s.st = st
	idx := st.Slide
	if idx == 0 && st.Total > 0 {
		idx = 1
	}
	if cur := s.slideLocked(idx); cur != nil {
		if st.SlideNotes != "" {
			cur.Notes = st.SlideNotes
		}
		if st.SlideTitle != "" {
			cur.Title = st.SlideTitle
		}
	}
	state := s.stateLocked()
	deckMsg := s.deckMsgLocked()
	stateJSON, _ := json.Marshal(state)
	deckJSON, _ := json.Marshal(deckMsg)
	sendState := string(stateJSON) != s.lastState
	sendDeck := string(deckJSON) != s.deckSig
	s.lastState, s.deckSig = string(stateJSON), string(deckJSON)
	onChange := s.onChange
	s.mu.Unlock()

	if s.send != nil {
		if sendDeck {
			s.send(deckMsg)
		}
		if sendState {
			s.send(state)
		}
	}
	if (sendState || sendDeck) && onChange != nil {
		onChange()
	}
}

// ReloadDeck forces an immediate refresh of slide titles and notes from PowerPoint.
func (s *Sync) ReloadDeck() {
	s.mu.Lock()
	s.deckKey = ""
	s.deckChecked = time.Time{}
	s.mu.Unlock()
	s.kick()
}

// refreshDeck (re)loads titles and notes when the deck changes or the file is saved.
func (s *Sync) refreshDeck(ctx context.Context, st presenter.Status) {
	if st.DeckPath == "" && st.DeckName == "" {
		s.mu.Lock()
		s.deck, s.deckKey = nil, ""
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	recent := time.Since(s.deckChecked) < 3*time.Second && strings.HasPrefix(s.deckKey, st.DeckPath+"|")
	s.mu.Unlock()
	if recent {
		return
	}
	var mtime int64
	local := false
	if fi, err := os.Stat(st.DeckPath); err == nil && !fi.IsDir() {
		local, mtime = true, fi.ModTime().UnixNano()
	}
	key := st.DeckPath + "|" + itoa64(mtime) + "|" + itoa64(int64(st.Total))
	s.mu.Lock()
	s.deckChecked = time.Now()
	same := key == s.deckKey
	hadKey := s.deckKey != ""
	s.mu.Unlock()
	if same {
		return
	}

	var slides []pptx.Slide
	var err error
	if local && hadKey {
		slides, err = pptx.Read(st.DeckPath)
		if err != nil || (st.Total > 0 && len(slides) != st.Total) {
			s.log.Info("file differs from the open deck or read error - reading notes from PowerPoint", "err", err)
			if appSlides, err2 := s.ctl.AppSlides(ctx); err2 == nil && len(appSlides) > 0 {
				slides = appSlides
				err = nil
			}
		}
	} else {
		slides, err = s.ctl.AppSlides(ctx)
		if (err != nil || len(slides) == 0) && local {
			slides, err = pptx.Read(st.DeckPath)
		}
	}
	if err != nil {
		s.log.Warn("could not read speaker notes", "deck", st.DeckName, "err", err)
	} else {
		s.log.Info("deck loaded", "deck", st.DeckName, "slides", len(slides), "source", map[bool]string{true: "file", false: "app"}[local])
	}
	s.mu.Lock()
	s.deck, s.deckKey = slides, key
	s.mu.Unlock()
}

func (s *Sync) slideLocked(n int) *pptx.Slide {
	for i := range s.deck {
		if s.deck[i].Index == n {
			return &s.deck[i]
		}
	}
	return nil
}

func (s *Sync) stateLocked() map[string]any {
	st := s.st
	idx := st.Slide
	if idx == 0 && st.Total > 0 {
		idx = 1
	}
	total := st.Total
	if total == 0 {
		total = len(s.deck)
	}
	m := map[string]any{
		"type":         "STATE_UPDATE",
		"isPresenting": st.Presenting,
		"currentSlide": idx,
		"totalSlides":  total,
		"title":        "",
		"notes":        "",
		// extensions
		"appRunning":   st.AppRunning,
		"presentation": st.DeckName,
		"ended":        st.Ended,
		"blackout":     st.Blackout,
		"hidden":       false,
		"nextSlide":    nil,
		"nextTitle":    nil,
		"controller":   s.ctl.Name(),
		"error":        strings.TrimPrefix(s.lastErr, "!"),
		"decks":        st.Decks,
		"hostVersion":  s.version,
	}
	if !s.showStarted.IsZero() {
		m["showStartedAt"] = s.showStarted.UnixMilli()
	}
	if cur := s.slideLocked(idx); cur != nil {
		m["title"], m["notes"], m["hidden"] = cur.Title, cur.Notes, cur.Hidden
	}
	if st.SlideTitle != "" {
		m["title"] = st.SlideTitle
	}
	if st.SlideNotes != "" {
		m["notes"] = st.SlideNotes
	}
	if idx < total {
		m["nextSlide"] = idx + 1
		title := ""
		if nxt := s.slideLocked(idx + 1); nxt != nil {
			title = nxt.Title
		}
		m["nextTitle"] = title
	}
	return m
}

func (s *Sync) deckMsgLocked() map[string]any {
	outline := make([]map[string]any, 0, len(s.deck))
	for _, sl := range s.deck {
		outline = append(outline, map[string]any{"n": sl.Index, "title": sl.Title, "hidden": sl.Hidden, "hasNotes": sl.Notes != ""})
	}
	return map[string]any{"type": "DECK", "presentation": s.st.DeckName, "slides": outline}
}

// CurrentState is embedded in AUTH_OK.
func (s *Sync) CurrentState() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.stateLocked()
	delete(m, "type")
	return m
}

// PushDeck sends the deck outline right after (re)authentication.
func (s *Sync) PushDeck() {
	s.mu.Lock()
	msg := s.deckMsgLocked()
	s.mu.Unlock()
	if s.send != nil {
		s.send(msg)
	}
}

// Status returns the last controller status (for the tray).
func (s *Sync) Status() presenter.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st
}

// Handle executes an authenticated phone command.
func (s *Sync) Handle(ctx context.Context, cmd session.Command) {
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var err error
	switch cmd.Action {
	case "START_SHOW":
		err = s.ctl.Start(cctx, cmd.Slide)
	case "END_SHOW":
		err = s.ctl.End(cctx)
	case "NEXT":
		err = s.ctl.Next(cctx)
	case "PREV":
		err = s.ctl.Prev(cctx)
	case "GOTO":
		err = s.ctl.Goto(cctx, cmd.Slide)
	case "BLACK":
		err = s.ctl.ToggleBlack(cctx)
	case "SWITCH_DECK":
		err = s.ctl.SwitchDeck(cctx, cmd.Deck, true)
	case "REFRESH":
		s.ReloadDeck()
		return
	default:
		s.log.Warn("unknown action", "action", cmd.Action)
		return
	}
	s.mu.Lock()
	if err != nil {
		s.lastErr = "!" + err.Error()
	} else {
		s.lastErr = ""
	}
	s.mu.Unlock()
	if err != nil {
		s.log.Warn("command failed", "action", cmd.Action, "err", err)
		if s.send != nil {
			s.send(map[string]any{"type": "ERROR", "message": err.Error()})
		}
	}
	s.poll(ctx) // push the result immediately
}

func itoa64(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
