package ppt

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"
)

// AdaptiveController tries the native OS PowerPoint controller.
// To prevent blocking WebSocket requests and state sync loops, it rate-limits
// probes to native PowerPoint (at most once every 3s if unavailable) and immediately
// falls back to the in-memory MockController.
type AdaptiveController struct {
	mu              sync.Mutex
	native          PowerPointController
	mock            *MockController
	logger          *log.Logger
	lastNativeProbe time.Time
	nativeAvailable bool
}

func NewAdaptiveController(native PowerPointController, mock *MockController, logger *log.Logger) *AdaptiveController {
	return &AdaptiveController{
		native: native,
		mock:   mock,
		logger: logger,
	}
}

func (a *AdaptiveController) Name() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.nativeAvailable {
		return a.native.Name()
	}
	return "Adaptive (Mock Presentation Active - PowerPoint Closed/Unresponsive)"
}

func (a *AdaptiveController) GetState(ctx context.Context) (PresentationState, error) {
	a.mu.Lock()
	now := time.Now()
	shouldProbe := a.nativeAvailable || now.Sub(a.lastNativeProbe) >= 3*time.Second
	if shouldProbe {
		a.lastNativeProbe = now
	}
	a.mu.Unlock()

	if shouldProbe {
		probeCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		state, err := a.native.GetState(probeCtx)
		cancel()

		a.mu.Lock()
		if err == nil && state.Available {
			if !a.nativeAvailable && a.logger != nil {
				a.logger.Printf("[PPT] Microsoft PowerPoint connected: %s", state.FileName)
			}
			a.nativeAvailable = true
			a.mu.Unlock()
			return state, nil
		}
		a.nativeAvailable = false
		a.mu.Unlock()
	}

	return a.mock.GetState(ctx)
}

func (a *AdaptiveController) ExecuteAction(ctx context.Context, action Action, targetSlide int) (PresentationState, error) {
	a.mu.Lock()
	isNative := a.nativeAvailable
	a.mu.Unlock()

	if isNative {
		actionCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		state, err := a.native.ExecuteAction(actionCtx, action, targetSlide)
		cancel()
		if err == nil && state.Available {
			return state, nil
		}
	}
	return a.mock.ExecuteAction(ctx, action, targetSlide)
}

// DetectController selects the best PowerPoint controller for the current operating system.
func DetectController(forceMock bool, logger *log.Logger) PowerPointController {
	if logger == nil {
		logger = log.Default()
	}

	mock := NewMockController()
	if forceMock {
		logger.Printf("[PPT] Using Mock presentation engine (--mock-ppt specified)")
		return mock
	}

	var native PowerPointController
	switch runtime.GOOS {
	case "darwin":
		native = NewMacOSController()
	case "windows":
		native = NewWindowsController()
	default:
		logger.Printf("[PPT] OS %s has no native PowerPoint integration, using Mock engine", runtime.GOOS)
		return mock
	}

	// Non-blocking initial probe
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	state, err := native.GetState(ctx)
	if err == nil && state.Available {
		logger.Printf("[PPT] Microsoft PowerPoint detected (%s). Active: %s (Slide %d/%d)",
			native.Name(), state.FileName, state.CurrentSlide, state.TotalSlides)
	} else {
		logger.Printf("[PPT] Microsoft PowerPoint not currently running or open. Adaptive fallback enabled.")
	}

	return NewAdaptiveController(native, mock, logger)
}
