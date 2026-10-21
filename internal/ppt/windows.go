package ppt

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// WindowsController implements PowerPointController using Windows PowerShell COM automation.
type WindowsController struct{}

func NewWindowsController() *WindowsController {
	return &WindowsController{}
}

func (w *WindowsController) Name() string {
	return "Windows Native COM Automation (PowerPoint.Application)"
}

func (w *WindowsController) GetState(ctx context.Context) (PresentationState, error) {
	delim := "|||PPTX_DELIM|||"
	psScript := fmt.Sprintf(`
$delim = "%s"
try {
    $ppt = [System.Runtime.InteropServices.Marshal]::GetActiveObject("PowerPoint.Application")
    if ($null -eq $ppt) {
        Write-Output ("false" + $delim + "false")
        exit
    }
    if ($ppt.Presentations.Count -eq 0) {
        Write-Output ("true" + $delim + "false" + $delim + "0" + $delim + "0" + $delim + "No Presentation Open")
        exit
    }

    $pres = $ppt.ActivePresentation
    $presName = $pres.Name
    $total = $pres.Slides.Count

    $isRunning = "false"
    $curSlide = 1

    if ($ppt.SlideShowWindows.Count -gt 0) {
        $isRunning = "true"
        $curSlide = $ppt.SlideShowWindow.View.CurrentShowPosition
    } else {
        try {
            $curSlide = $ppt.ActiveWindow.View.Slide.SlideIndex
        } catch {
            $curSlide = 1
        }
    }

    $curSlideObj = $pres.Slides.Item($curSlide)
    $title = "Slide " + $curSlide
    try {
        if ($curSlideObj.Shapes.HasTitle) {
            $title = $curSlideObj.Shapes.Title.TextFrame.TextRange.Text
        }
    } catch {}

    $notes = ""
    try {
        if ($curSlideObj.NotesPage.Shapes.Count -ge 2) {
            $notes = $curSlideObj.NotesPage.Shapes.Item(2).TextFrame.TextRange.Text
        }
    } catch {}

    $nextTitle = ""
    $nextNotes = ""
    if ($curSlide -lt $total) {
        $nxt = $curSlide + 1
        $nextTitle = "Slide " + $nxt
        $nxtSlideObj = $pres.Slides.Item($nxt)
        try {
            if ($nxtSlideObj.Shapes.HasTitle) {
                $nextTitle = $nxtSlideObj.Shapes.Title.TextFrame.TextRange.Text
            }
        } catch {}
        try {
            if ($nxtSlideObj.NotesPage.Shapes.Count -ge 2) {
                $nextNotes = $nxtSlideObj.NotesPage.Shapes.Item(2).TextFrame.TextRange.Text
            }
        } catch {}
    }

    Write-Output ("true" + $delim + $isRunning + $delim + $curSlide + $delim + $total + $delim + $title + $delim + $notes + $delim + $nextTitle + $delim + $nextNotes + $delim + $presName)
} catch {
    Write-Output ("false" + $delim + "false" + $delim + "0" + $delim + "0" + $delim + $_.Exception.Message)
}
`, delim)

	ctxTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, "powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return PresentationState{Available: false}, fmt.Errorf("powershell exec failed: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), delim)
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

func (w *WindowsController) ExecuteAction(ctx context.Context, action Action, targetSlide int) (PresentationState, error) {
	var psCmd string
	switch action {
	case ActionNextSlide:
		psCmd = `$ppt = [System.Runtime.InteropServices.Marshal]::GetActiveObject("PowerPoint.Application"); if ($ppt.SlideShowWindows.Count -gt 0) { $ppt.SlideShowWindow.View.Next() } else { $ppt.ActivePresentation.SlideShowSettings.Run() }`
	case ActionPrevSlide:
		psCmd = `$ppt = [System.Runtime.InteropServices.Marshal]::GetActiveObject("PowerPoint.Application"); if ($ppt.SlideShowWindows.Count -gt 0) { $ppt.SlideShowWindow.View.Previous() }`
	case ActionGotoSlide:
		psCmd = fmt.Sprintf(`$ppt = [System.Runtime.InteropServices.Marshal]::GetActiveObject("PowerPoint.Application"); if ($ppt.SlideShowWindows.Count -gt 0) { $ppt.SlideShowWindow.View.GotoSlide(%d) }`, targetSlide)
	case ActionStartShow:
		psCmd = `$ppt = [System.Runtime.InteropServices.Marshal]::GetActiveObject("PowerPoint.Application"); $ppt.ActivePresentation.SlideShowSettings.Run()`
	case ActionEndShow:
		psCmd = `$ppt = [System.Runtime.InteropServices.Marshal]::GetActiveObject("PowerPoint.Application"); if ($ppt.SlideShowWindows.Count -gt 0) { $ppt.SlideShowWindow.View.Exit() }`
	case ActionBlackScreen:
		psCmd = `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait("b")`
	case ActionWhiteScreen:
		psCmd = `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait("w")`
	default:
		return w.GetState(ctx)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, "powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	_ = cmd.Run()

	return w.GetState(ctx)
}
