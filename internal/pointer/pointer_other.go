//go:build !darwin || !cgo

package pointer

type stubController struct{}

func newPlatform() Controller {
	return &stubController{}
}

func (s *stubController) Update(active bool, x, y float64, mode string) {}
func (s *stubController) Close()                                        {}
