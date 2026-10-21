package presenter

import (
	"os"
	"strings"
)

// NormalizeMacPath converts what PowerPoint for Mac reports as "full name" into a POSIX
// path. Depending on the Office version this is already POSIX, an HFS path
// ("Macintosh HD:Users:me:Talks:Deck.pptx") or a URL for cloud documents.
func NormalizeMacPath(p string, exists func(string) bool) string {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "://") {
		return p
	}
	i := strings.Index(p, ":")
	if i <= 0 {
		return p
	}
	if exists == nil {
		exists = func(s string) bool { _, err := os.Stat(s); return err == nil }
	}
	volume, rest := p[:i], strings.ReplaceAll(p[i+1:], ":", "/")
	if root := "/" + rest; exists(root) { // startup disk
		return root
	}
	return "/Volumes/" + volume + "/" + rest
}
