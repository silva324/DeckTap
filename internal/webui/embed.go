// Package webui embeds the phone client and serves it with strict security headers.
package webui

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed static
var files embed.FS

// Handler serves the static phone client. Only GET/HEAD of known files is allowed.
func Handler() http.Handler {
	root, _ := fs.Sub(files, "static")
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		data, err := fs.ReadFile(root, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; worker-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; "+
			"connect-src 'self' wss: ws:; manifest-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(self), microphone=(), geolocation=(), payment=()")
		h.Set("Cache-Control", "no-store") // the app is tiny; never serve a stale (or proxy-cached) client
		if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
			h.Set("Content-Type", ct)
		}
		if r.Method == http.MethodHead {
			return
		}
		w.Write(data)
	})
}
