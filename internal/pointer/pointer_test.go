package pointer

import "testing"

func TestPointerLifecycle(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("expected non-nil pointer Controller")
	}

	// Verify Update calls do not panic or crash
	p.Update(false, 0.5, 0.5, "laser")
	p.Update(false, 0.5, 0.5, "spotlight")
	p.Close()
}
