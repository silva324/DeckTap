# DeckTap PowerPoint bridge (Windows PowerShell 5.1, COM). One JSON request per line on
# stdin, one JSON response per line on stdout. Started and owned by slidecue.exe.
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding $false

function Get-App {
    try { return [Runtime.InteropServices.Marshal]::GetActiveObject('PowerPoint.Application') } catch { return $null }
}
function Get-Win($app) {
    if ($app -and $app.SlideShowWindows.Count -gt 0) { return $app.SlideShowWindows.Item(1) }
    return $null
}
function Get-Pres($app) {
    $w = Get-Win $app
    if ($w) { return $w.Presentation }
    if ($app -and $app.Presentations.Count -gt 0) {
        try { return $app.ActivePresentation } catch { return $app.Presentations.Item(1) }
    }
    return $null
}
function Need-View {
    $w = Get-Win (Get-App)
    if (-not $w) { throw 'Slide show is not running' }
    return $w.View
}
function Get-Status {
    $app = Get-App
    if (-not $app) { return @{ app = $false } }
    $r = @{ app = $true; name = ''; path = ''; total = 0; showing = $false; idx = 0; black = $false; done = $false; decks = @() }
    $p = Get-Pres $app
    if ($p) { $r.name = $p.Name; $r.path = $p.FullName; $r.total = $p.Slides.Count }
    $w = Get-Win $app
    if ($w) {
        $r.showing = $true
        $v = $w.View
        $st = $v.State
        $r.black = ($st -eq 3)
        $r.done = ($st -eq 5)
        if ($r.done) { $r.idx = $r.total }
        else { try { $r.idx = $v.Slide.SlideIndex } catch { $r.idx = $v.CurrentShowPosition } }
    } else {
        try {
            if ($app.ActiveWindow -and $app.ActiveWindow.Selection -and $app.ActiveWindow.Selection.SlideRange) {
                $r.idx = $app.ActiveWindow.Selection.SlideRange.Item(1).SlideIndex
            }
        } catch {}
    }
    $notes = ""
    $title = ""
    if ($p -and $r.idx -gt 0 -and $r.idx -le $r.total) {
        try {
            $s = $p.Slides.Item($r.idx)
            try { if ($s.Shapes.HasTitle) { $title = $s.Shapes.Title.TextFrame.TextRange.Text.Trim() } } catch {}
            try { $notes = $s.NotesPage.Shapes.Placeholders.Item(2).TextFrame.TextRange.Text } catch {
                try { $notes = $s.NotesPage.Shapes.Item(2).TextFrame.TextRange.Text } catch {}
            }
        } catch {}
    }
    $r.notes = $notes
    $r.title = $title
    $decks = @()
    if ($app -and $app.Presentations) {
        try {
            foreach ($pr in $app.Presentations) {
                $isAct = ($p -and $pr.Name -eq $p.Name)
                $isShw = ($w -and $w.Presentation -and $pr.Name -eq $w.Presentation.Name)
                $decks += @{ name = $pr.Name; total = $pr.Slides.Count; active = $isAct; showing = $isShw }
            }
        } catch {}
    }
    $r.decks = $decks
    return $r
}
function Get-Slides {
    $p = Get-Pres (Get-App)
    if (-not $p) { throw 'No presentation is open in PowerPoint' }
    $list = New-Object System.Collections.ArrayList
    foreach ($s in $p.Slides) {
        $t = ''; $n = ''
        try { if ($s.Shapes.HasTitle) { $t = $s.Shapes.Title.TextFrame.TextRange.Text.Trim() } } catch {}
        if (-not $t) {
            try {
                $bestTop = 999999.0
                foreach ($ph in $s.Shapes.Placeholders) {
                    if ($ph.HasTextFrame -and $ph.TextFrame.HasText) {
                        $txt = $ph.TextFrame.TextRange.Text.Trim()
                        if ($txt -and $ph.Top -lt 450 -and $ph.Top -lt $bestTop) {
                            $t = $txt
                            $bestTop = $ph.Top
                        }
                    }
                }
            } catch {}
        }
        try { $n = $s.NotesPage.Shapes.Placeholders.Item(2).TextFrame.TextRange.Text } catch {}
        [void]$list.Add(@{ n = $s.SlideIndex; title = $t; notes = $n; hidden = ($s.SlideShowTransition.Hidden -eq -1) })
    }
    return ,$list.ToArray()
}
function Start-Show($from) {
    $app = Get-App
    if (-not $app) { throw 'PowerPoint is not running' }
    $w = Get-Win $app
    if (-not $w) {
        $p = Get-Pres $app
        if (-not $p) { throw 'No presentation is open in PowerPoint' }
        try { $p.SlideShowSettings.ShowWithPresenter = $true } catch {}
        $w = $p.SlideShowSettings.Run()
    }
    if ($from -gt 1) { $w.View.GotoSlide([int]$from) }
}
function Resume-IfBlack($v) { if ($v.State -eq 3) { $v.State = 1 } }

while ($true) {
    $line = [Console]::In.ReadLine()
    if ($null -eq $line) { break }
    try {
        $req = $line | ConvertFrom-Json
        $data = $null
        switch ($req.cmd) {
            'status' { $data = Get-Status }
            'slides' { $data = Get-Slides }
            'start'  { Start-Show $req.n }
            'next'   { $v = Need-View; Resume-IfBlack $v; $v.Next() }
            'prev'   { $v = Need-View; Resume-IfBlack $v; $v.Previous() }
            'goto'   { $v = Need-View; $v.GotoSlide([int]$req.n) }
            'end'    { $v = Need-View; $v.Exit() }
            'black'  { $v = Need-View; if ($v.State -eq 3) { $v.State = 1 } else { $v.State = 3 } }
            { $_ -like 'switch:*' } {
                $target = $_.Substring(7)
                $app = Get-App
                if (-not $app) { throw 'PowerPoint is not running' }
                $pr = $null
                try { $pr = $app.Presentations.Item($target) } catch {}
                if (-not $pr) { throw "Presentation '$target' not found" }
                $pr.Windows.Item(1).Activate()
                if ($req.n -eq 1) {
                    $sw = Get-Win $app
                    if ($sw) { $sw.View.Exit() }
                    $pr.SlideShowSettings.Run()
                }
            }
            default  { throw "unknown command $($req.cmd)" }
        }
        $resp = @{ id = $req.id; ok = $true; data = $data }
    } catch {
        $resp = @{ id = $req.id; ok = $false; error = $_.Exception.Message }
    }
    [Console]::Out.WriteLine((ConvertTo-Json -InputObject $resp -Compress -Depth 6))
    [Console]::Out.Flush()
}
