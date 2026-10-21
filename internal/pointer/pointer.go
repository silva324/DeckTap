package pointer

// Controller manages the on-screen laser pointer and spotlight overlay.
type Controller interface {
	// Update updates the pointer position and state.
	// x and y are normalized screen coordinates between 0.0 and 1.0.
	// mode is "laser" or "spotlight".
	Update(active bool, x, y float64, mode string)
	Close()
}

// New creates a platform-appropriate pointer controller.
func New() Controller {
	return newPlatform()
}
