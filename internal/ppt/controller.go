package ppt

import "context"

// Action represents presentation control actions.
type Action string

const (
	ActionNextSlide   Action = "NEXT_SLIDE"
	ActionPrevSlide   Action = "PREV_SLIDE"
	ActionGotoSlide   Action = "GOTO_SLIDE"
	ActionStartShow   Action = "START_SHOW"
	ActionEndShow     Action = "END_SHOW"
	ActionBlackScreen Action = "BLACK_SCREEN"
	ActionWhiteScreen Action = "WHITE_SCREEN"
)

// PresentationState holds real-time telemetry from Microsoft PowerPoint.
type PresentationState struct {
	Available    bool     `json:"available"`     // PowerPoint app is running with active presentation
	Running      bool     `json:"running"`       // Slideshow is actively in presentation mode
	CurrentSlide int      `json:"current_slide"` // Current 1-based slide index
	TotalSlides  int      `json:"total_slides"`  // Total number of slides
	Title        string   `json:"title"`         // Current slide title
	Notes        string   `json:"notes"`         // Speaker notes for current slide
	NextTitle    string   `json:"next_title"`    // Title of upcoming slide
	NextNotes    string   `json:"next_notes"`    // Notes of upcoming slide (speaker preview)
	FileName     string   `json:"file_name"`     // Presentation filename
	BlackScreen  bool     `json:"black_screen"`  // Blank screen toggle state
	WhiteScreen  bool     `json:"white_screen"`  // White screen toggle state
	SlideTitles  []string `json:"slide_titles"`  // Quick index of all slide titles
}

// PowerPointController defines the cross-platform presentation control contract.
type PowerPointController interface {
	GetState(ctx context.Context) (PresentationState, error)
	ExecuteAction(ctx context.Context, action Action, targetSlide int) (PresentationState, error)
	Name() string
}
