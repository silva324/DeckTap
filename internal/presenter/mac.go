//go:build darwin

package presenter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"pptx-remote/internal/pptx"
)

// Mac drives Microsoft PowerPoint for Mac through AppleScript (osascript).
//
// Only well-established PowerPoint AppleScript terms are compiled directly. Terms whose
// availability varies across Office versions (slide state) are evaluated with
// `run script`, so a missing term degrades one feature instead of breaking polling.
//
// First use triggers the macOS prompt "DeckTap wants to control Microsoft PowerPoint"
// (Automation permission, System Settings › Privacy & Security › Automation).
type Mac struct{}

func NewPlatform() (Controller, error) { return &Mac{}, nil }

func (m *Mac) Name() string { return "powerpoint-mac" }
func (m *Mac) Close()       {}

const fieldSep = "\x1f"
const recordSep = "\x1e"

func installed() bool {
	for _, p := range []string{
		"/Applications/Microsoft PowerPoint.app",
		os.Getenv("HOME") + "/Applications/Microsoft PowerPoint.app",
	} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// osascript runs an AppleScript read from stdin; args are passed to `on run argv`
// (never interpolated into the source).
func osascript(ctx context.Context, script string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", append([]string{"-"}, args...)...)
	cmd.Stdin = strings.NewReader(script)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if strings.Contains(msg, "-1743") {
			return "", fmt.Errorf("macOS blocked automation: allow DeckTap to control Microsoft PowerPoint in System Settings › Privacy & Security › Automation")
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("osascript: %s", msg)
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

const statusScript = `
set fs to character id 31
if application "Microsoft PowerPoint" is not running then return "0"
tell application "Microsoft PowerPoint"
	set pname to ""
	set ppath to ""
	set total to 0
	set showing to "0"
	set idx to 0
	try
		set p to active presentation
		set pname to name of p
		set ppath to full name of p
		set total to count of slides of p
	end try
	try
		if (count of slide show windows) > 0 then
			set showing to "1"
			set idx to slide index of slide of slide show view of slide show window 1
		end if
	end try
	set deckStr to ""
	try
		repeat with i from 1 to count of presentations
			set pr to presentation i
			set dName to name of pr
			set dTotal to count of slides of pr
			set deckStr to deckStr & dName & ":" & dTotal & ";"
		end repeat
	end try
end tell
set sstate to ""
if showing is "1" then
	try
		set sstate to run script "tell application \"Microsoft PowerPoint\" to return (slide state of slide show view of slide show window 1) as string"
	end try
end if
return "1" & fs & pname & fs & ppath & fs & total & fs & showing & fs & idx & fs & sstate & fs & deckStr
`

func (m *Mac) Status(ctx context.Context) (Status, error) {
	if !installed() {
		return Status{}, nil
	}
	out, err := osascript(ctx, statusScript)
	if err != nil {
		return Status{}, err
	}
	f := strings.Split(out, fieldSep)
	if len(f) < 7 || f[0] != "1" {
		return Status{}, nil
	}
	st := Status{AppRunning: true, DeckName: f[1], DeckPath: NormalizeMacPath(f[2], nil)}
	st.Total, _ = strconv.Atoi(f[3])
	st.Presenting = f[4] == "1"
	st.Slide, _ = strconv.Atoi(f[5])
	state := strings.ToLower(f[6])
	st.Blackout = strings.Contains(state, "black")
	st.Ended = strings.Contains(state, "done")
	if st.Ended && st.Slide == 0 {
		st.Slide = st.Total
	}

	if len(f) >= 8 && f[7] != "" {
		for _, raw := range strings.Split(f[7], ";") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			parts := strings.SplitN(raw, ":", 2)
			dTotal := 0
			if len(parts) == 2 {
				dTotal, _ = strconv.Atoi(parts[1])
			}
			dName := parts[0]
			st.Decks = append(st.Decks, DeckInfo{
				Name:    dName,
				Total:   dTotal,
				Active:  dName == st.DeckName,
				Showing: dName == st.DeckName && st.Presenting,
			})
		}
	}
	return st, nil
}

const notesScript = `
set fieldSeparator to character id 31
set recordSeparator to character id 30
tell application "Microsoft PowerPoint"
	set curPres to active presentation
	set slideCount to count of slides of curPres
	set resultOutput to ""
	repeat with sIdx from 1 to slideCount
		set curSlide to slide sIdx of curPres
		set slideTitle to ""
		try
			set slideTitle to content of text range of text frame of place holder 1 of curSlide
		end try
		set noteText to ""
		try
			set noteText to content of text range of text frame of place holder 2 of notes page of curSlide
		end try
		if noteText is "" then
			try
				repeat with shp in shapes of notes page of curSlide
					if has text frame of shp then
						set candidate to content of text range of text frame of shp
						if candidate is not "" and candidate is not (sIdx as string) then
							set noteText to candidate
							exit repeat
						end if
					end if
				end repeat
			end try
		end if
		set resultOutput to resultOutput & sIdx & fieldSeparator & slideTitle & fieldSeparator & noteText & recordSeparator
	end repeat
	return resultOutput
end tell
`

func (m *Mac) AppSlides(ctx context.Context) ([]pptx.Slide, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := osascript(ctx, notesScript)
	if err != nil {
		return nil, err
	}
	var slides []pptx.Slide
	for _, rec := range strings.Split(out, recordSep) {
		parts := strings.Split(rec, fieldSep)
		if len(parts) < 2 {
			continue
		}
		n, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		title := ""
		notes := ""
		if len(parts) >= 3 {
			title = CleanText(parts[1])
			notes = CleanText(parts[2])
		} else {
			notes = CleanText(parts[1])
		}
		slides = append(slides, pptx.Slide{Index: n, Title: title, Notes: notes})
	}
	return slides, nil
}

func (m *Mac) requireShow(ctx context.Context) error {
	st, err := m.Status(ctx)
	if err != nil {
		return err
	}
	if !st.AppRunning {
		return ErrNotRunning
	}
	if !st.Presenting {
		return ErrNoShow
	}
	return nil
}

func (m *Mac) Start(ctx context.Context, from int) error {
	st, err := m.Status(ctx)
	if err != nil {
		return err
	}
	if !st.AppRunning {
		return ErrNotRunning
	}
	if st.DeckName == "" {
		return ErrNoDeck
	}
	if !st.Presenting {
		if _, err := osascript(ctx, `tell application "Microsoft PowerPoint"
	activate
	run slide show slide show settings of active presentation
end tell`); err != nil {
			return err
		}
		if from > 1 {
			time.Sleep(400 * time.Millisecond) // the show window needs a moment
			return m.Goto(ctx, from)
		}
	}
	return nil
}

func (m *Mac) End(ctx context.Context) error {
	if err := m.requireShow(ctx); err != nil {
		return err
	}
	_, err := osascript(ctx, `tell application "Microsoft PowerPoint" to exit slide show slide show view of slide show window 1`)
	return err
}

func (m *Mac) Next(ctx context.Context) error {
	_, err := osascript(ctx, `tell application "Microsoft PowerPoint"
	if (count of slide show windows) is 0 then error "Slide show is not running"
	go to next slide slide show view of slide show window 1
end tell`)
	return err
}

func (m *Mac) Prev(ctx context.Context) error {
	_, err := osascript(ctx, `tell application "Microsoft PowerPoint"
	if (count of slide show windows) is 0 then error "Slide show is not running"
	go to previous slide slide show view of slide show window 1
end tell`)
	return err
}

func (m *Mac) Goto(ctx context.Context, n int) error {
	_, err := osascript(ctx, `on run argv
	set n to (item 1 of argv) as integer
	tell application "Microsoft PowerPoint"
		if (count of slide show windows) is 0 then error "Slide show is not running"
		go to slide slide show view of slide show window 1 number n
	end tell
end run`, strconv.Itoa(n))
	return err
}

// ToggleBlack prefers the slide-state API and falls back to the "B" key, which needs
// the Accessibility permission for DeckTap.
func (m *Mac) ToggleBlack(ctx context.Context) error {
	if err := m.requireShow(ctx); err != nil {
		return err
	}
	_, err := osascript(ctx, `tell application "Microsoft PowerPoint"
	set v to slide show view of slide show window 1
	if ((slide state of v) as string) contains "black" then
		run script "tell application \"Microsoft PowerPoint\" to set slide state of slide show view of slide show window 1 to slide show state running"
	else
		run script "tell application \"Microsoft PowerPoint\" to set slide state of slide show view of slide show window 1 to slide show state black screen"
	end if
end tell`)
	if err == nil {
		return nil
	}
	_, err2 := osascript(ctx, `tell application "Microsoft PowerPoint" to activate
delay 0.1
tell application "System Events" to keystroke "b"`)
	if err2 != nil {
		return fmt.Errorf("black screen unavailable (%v); allow Accessibility for DeckTap to enable the B-key fallback", err)
	}
	return nil
}

func (m *Mac) SwitchDeck(ctx context.Context, name string, startShow bool) error {
	st, err := m.Status(ctx)
	if err != nil {
		return err
	}
	if !st.AppRunning {
		return ErrNotRunning
	}
	showFlag := "0"
	if startShow {
		showFlag = "1"
	}
	const switchScript = `on run argv
	set targetName to (item 1 of argv) as string
	set startShow to (item 2 of argv) as string
	tell application "Microsoft PowerPoint"
		set pr to presentation targetName
		set w to document window 1 of pr
		activate
		activate w
		if startShow is "1" then
			if (count of slide show windows) > 0 then
				exit slide show slide show view of slide show window 1
			end if
			run slide show slide show settings of pr
		end if
	end tell
end run`
	_, err = osascript(ctx, switchScript, name, showFlag)
	return err
}

func (m *Mac) Diagnose(ctx context.Context) []string {
	var out []string
	add := func(ok bool, msg string) {
		if ok {
			out = append(out, "OK   "+msg)
		} else {
			out = append(out, "FAIL "+msg)
		}
	}
	add(installed(), "Microsoft PowerPoint installed in /Applications")
	st, err := m.Status(ctx)
	add(err == nil, fmt.Sprintf("AppleScript status query (err=%v)", err))
	if err == nil {
		add(st.AppRunning, "PowerPoint running")
		add(st.DeckName != "", fmt.Sprintf("active presentation: %q (%d slides) path=%q", st.DeckName, st.Total, st.DeckPath))
		if st.Presenting {
			add(st.Slide > 0, fmt.Sprintf("slide show running, current slide %d, black=%v ended=%v", st.Slide, st.Blackout, st.Ended))
		} else {
			out = append(out, "INFO start a slide show to test navigation")
		}
	}
	if st.DeckName != "" {
		slides, err := m.AppSlides(ctx)
		withNotes := 0
		for _, s := range slides {
			if s.Notes != "" {
				withNotes++
			}
		}
		add(err == nil && len(slides) > 0, fmt.Sprintf("notes via AppleScript: %d slides, %d with notes (err=%v)", len(slides), withNotes, err))
	}
	return out
}
