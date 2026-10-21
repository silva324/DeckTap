//go:build !darwin && !windows

package presenter

import "errors"

// NewPlatform: PowerPoint control is only available on macOS and Windows. Use --demo.
func NewPlatform() (Controller, error) {
	return nil, errors.New("PowerPoint control requires macOS or Windows (use --demo FILE.pptx to try the app)")
}
