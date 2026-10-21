package ppt

import (
	"context"
	"testing"
)

func TestMockController(t *testing.T) {
	ctx := context.Background()
	m := NewMockController()

	state, err := m.GetState(ctx)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}

	if !state.Available || !state.Running {
		t.Errorf("expected available and running, got available=%v, running=%v", state.Available, state.Running)
	}
	if state.CurrentSlide != 1 {
		t.Errorf("expected current slide 1, got %d", state.CurrentSlide)
	}
	if state.TotalSlides != 8 {
		t.Errorf("expected total slides 8, got %d", state.TotalSlides)
	}
	if state.Notes == "" {
		t.Errorf("expected non-empty speaker notes")
	}
	if state.NextTitle == "" {
		t.Errorf("expected non-empty next slide title")
	}

	// 1. Advance slide
	state, err = m.ExecuteAction(ctx, ActionNextSlide, 0)
	if err != nil {
		t.Fatalf("ExecuteAction NEXT_SLIDE failed: %v", err)
	}
	if state.CurrentSlide != 2 {
		t.Errorf("expected slide 2, got %d", state.CurrentSlide)
	}

	// 2. Go back
	state, err = m.ExecuteAction(ctx, ActionPrevSlide, 0)
	if err != nil {
		t.Fatalf("ExecuteAction PREV_SLIDE failed: %v", err)
	}
	if state.CurrentSlide != 1 {
		t.Errorf("expected slide 1, got %d", state.CurrentSlide)
	}

	// 3. Lower boundary check (cannot go below 1)
	state, err = m.ExecuteAction(ctx, ActionPrevSlide, 0)
	if state.CurrentSlide != 1 {
		t.Errorf("expected boundary at slide 1, got %d", state.CurrentSlide)
	}

	// 4. Jump to slide 5
	state, err = m.ExecuteAction(ctx, ActionGotoSlide, 5)
	if state.CurrentSlide != 5 {
		t.Errorf("expected slide 5, got %d", state.CurrentSlide)
	}

	// 5. Jump out of bounds (100) -> ignored
	state, err = m.ExecuteAction(ctx, ActionGotoSlide, 100)
	if state.CurrentSlide != 5 {
		t.Errorf("expected slide to remain 5, got %d", state.CurrentSlide)
	}

	// 6. Test Blank screen toggle
	state, err = m.ExecuteAction(ctx, ActionBlackScreen, 0)
	if !state.BlackScreen {
		t.Errorf("expected BlackScreen=true, got %v", state.BlackScreen)
	}
	state, err = m.ExecuteAction(ctx, ActionBlackScreen, 0)
	if state.BlackScreen {
		t.Errorf("expected BlackScreen=false, got %v", state.BlackScreen)
	}

	// 7. Test White screen toggle
	state, err = m.ExecuteAction(ctx, ActionWhiteScreen, 0)
	if !state.WhiteScreen {
		t.Errorf("expected WhiteScreen=true, got %v", state.WhiteScreen)
	}

	// 8. Test End Show & Start Show
	state, err = m.ExecuteAction(ctx, ActionEndShow, 0)
	if state.Running {
		t.Errorf("expected Running=false after EndShow")
	}
	state, err = m.ExecuteAction(ctx, ActionStartShow, 0)
	if !state.Running {
		t.Errorf("expected Running=true after StartShow")
	}
}

func TestAdaptiveControllerFallback(t *testing.T) {
	ctx := context.Background()
	mock := NewMockController()

	// A dummy controller that simulates PowerPoint being closed/unavailable
	dummyClosed := &MockController{available: false}

	adaptive := NewAdaptiveController(dummyClosed, mock, nil)

	state, err := adaptive.GetState(ctx)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}
	if !state.Available {
		t.Errorf("expected adaptive fallback to supply available state")
	}

	state, err = adaptive.ExecuteAction(ctx, ActionNextSlide, 0)
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if state.CurrentSlide != 2 {
		t.Errorf("expected slide 2 from fallback, got %d", state.CurrentSlide)
	}
}
