package pptx

import (
	"strings"
	"testing"
)

func TestReadDemo(t *testing.T) {
	slides, err := Read("testdata/demo.pptx")
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 5 {
		t.Fatalf("got %d slides", len(slides))
	}
	if slides[0].Title != "Welcome to SlideCue" {
		t.Fatalf("title %q", slides[0].Title)
	}
	if !strings.Contains(slides[0].Notes, "Swipe left") || !strings.Contains(slides[0].Notes, "\n") {
		t.Fatalf("notes %q", slides[0].Notes)
	}
	if !slides[3].Hidden || slides[2].Hidden {
		t.Fatal("hidden flag")
	}
	if slides[4].Index != 5 {
		t.Fatal("index")
	}
}

func TestReadGarbage(t *testing.T) {
	if _, err := ReadBytes([]byte("not a zip")); err == nil {
		t.Fatal("expected error")
	}
}
