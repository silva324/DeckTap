package presenter

import "testing"

func TestNormalizeMacPath(t *testing.T) {
	yes := func(string) bool { return true }
	no := func(string) bool { return false }
	cases := []struct {
		in, want string
		ex       func(string) bool
	}{
		{"/Users/me/a.pptx", "/Users/me/a.pptx", no},
		{"https://x.sharepoint.com/a.pptx", "https://x.sharepoint.com/a.pptx", no},
		{"Macintosh HD:Users:me:Talks:Deck.pptx", "/Users/me/Talks/Deck.pptx", yes},
		{"USB:Talks:Deck.pptx", "/Volumes/USB/Talks/Deck.pptx", no},
		{"Deck.pptx", "Deck.pptx", no},
	}
	for _, c := range cases {
		if got := NormalizeMacPath(c.in, c.ex); got != c.want {
			t.Errorf("%q -> %q, want %q", c.in, got, c.want)
		}
	}
	if CleanText("a\rb\vc\r\n") != "a\nb\nc" {
		t.Fatal("CleanText")
	}
}
