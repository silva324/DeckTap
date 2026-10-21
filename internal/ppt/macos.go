package ppt

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const delimiter = "|||PPTX_DELIM|||"

// MacOSController implements PowerPointController using macOS AppleScript (osascript).
type MacOSController struct{}

func NewMacOSController() *MacOSController {
	return &MacOSController{}
}

func (m *MacOSController) Name() string {
	return "macOS Native AppleScript (Microsoft PowerPoint)"
}

func (m *MacOSController) GetState(ctx context.Context) (PresentationState, error) {
	script := fmt.Sprintf(`
set delim to "%s"
try
	tell application "Microsoft PowerPoint"
		if not (exists presentation 1) then
			return "true" & delim & "false" & delim & "0" & delim & "0" & delim & "No Presentation Open"
		end if

		set activePres to active presentation
		set presName to name of activePres
		set totalSlides to count of slides of activePres

		set isRunning to false
		set curSlide to 1

		if (count of slide show windows) > 0 then
			set isRunning to true
			set curSlide to slide index of slide of slide show view of slide show window 1
		else
			try
				set curSlide to slide index of slide of view of document window 1
			on error
				set curSlide to 1
			end try
		end if

		set slideTitle to "Slide " & (curSlide as string)
		try
			set titleTxt to content of text range of text frame of title shape of slide curSlide of activePres
			if titleTxt is not "" then
				set slideTitle to titleTxt
			end if
		end try

		set slideNotes to ""
		try
			set theNotesPage to notes page of slide curSlide of activePres
			repeat with shp in shapes of theNotesPage
				if has text frame of shp then
					set shpTxt to content of text range of text frame of shp
					if shpTxt is not "" and shpTxt is not (curSlide as string) then
						set slideNotes to shpTxt
						exit repeat
					end if
				end if
			end repeat
		end try

		set nextSlideTitle to ""
		set nextSlideNotes to ""
		if curSlide < totalSlides then
			set nxtIdx to curSlide + 1
			set nextSlideTitle to "Slide " & (nxtIdx as string)
			try
				set nxtTitleTxt to content of text range of text frame of title shape of slide nxtIdx of activePres
				if nxtTitleTxt is not "" then
					set nextSlideTitle to nxtTitleTxt
				end if
			end try
		end if

		return "true" & delim & (isRunning as string) & delim & (curSlide as string) & delim & (totalSlides as string) & delim & slideTitle & delim & slideNotes & delim & nextSlideTitle & delim & nextSlideNotes & delim & presName
	end tell
on error errMsg
	return "false" & delim & "false" & delim & "0" & delim & "0" & delim & errMsg
end try
`, delimiter)

	ctxTimeout, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, "osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return PresentationState{Available: false}, fmt.Errorf("osascript exec failed: %w (output: %s)", err, string(output))
	}

	parts := strings.Split(strings.TrimSpace(string(output)), delimiter)
	if len(parts) < 2 {
		return PresentationState{Available: false}, nil
	}

	available := parts[0] == "true"
	if !available {
		return PresentationState{Available: false}, nil
	}

	running := parts[1] == "true"
	curSlide, _ := strconv.Atoi(parts[2])
	totalSlides, _ := strconv.Atoi(parts[3])

	title := ""
	if len(parts) > 4 {
		title = parts[4]
	}
	notes := ""
	if len(parts) > 5 {
		notes = parts[5]
	}
	nextTitle := ""
	if len(parts) > 6 {
		nextTitle = parts[6]
	}
	nextNotes := ""
	if len(parts) > 7 {
		nextNotes = parts[7]
	}
	fileName := ""
	if len(parts) > 8 {
		fileName = parts[8]
	}

	return PresentationState{
		Available:    available,
		Running:      running,
		CurrentSlide: curSlide,
		TotalSlides:  totalSlides,
		Title:        title,
		Notes:        notes,
		NextTitle:    nextTitle,
		NextNotes:    nextNotes,
		FileName:     fileName,
	}, nil
}

func (m *MacOSController) ExecuteAction(ctx context.Context, action Action, targetSlide int) (PresentationState, error) {
	var script string
	switch action {
	case ActionNextSlide:
		script = `
tell application "Microsoft PowerPoint"
	if (count of slide show windows) > 0 then
		next slide of slide show view of slide show window 1
	else if (count of presentations) > 0 then
		run slide show of slide show settings of active presentation
	end if
end tell`
	case ActionPrevSlide:
		script = `
tell application "Microsoft PowerPoint"
	if (count of slide show windows) > 0 then
		previous slide of slide show view of slide show window 1
	end if
end tell`
	case ActionGotoSlide:
		script = fmt.Sprintf(`
tell application "Microsoft PowerPoint"
	if (count of slide show windows) > 0 then
		go to slide (slide show view of slide show window 1) number %d
	end if
end tell`, targetSlide)
	case ActionStartShow:
		script = `
tell application "Microsoft PowerPoint"
	if (count of presentations) > 0 then
		run slide show of slide show settings of active presentation
	end if
end tell`
	case ActionEndShow:
		script = `
tell application "Microsoft PowerPoint"
	if (count of slide show windows) > 0 then
		exit slide show (slide show view of slide show window 1)
	end if
end tell`
	case ActionBlackScreen:
		script = `
tell application "System Events"
	keystroke "b"
end tell`
	case ActionWhiteScreen:
		script = `
tell application "System Events"
	keystroke "w"
end tell`
	default:
		return m.GetState(ctx)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, "osascript", "-e", script)
	_ = cmd.Run()

	return m.GetState(ctx)
}
