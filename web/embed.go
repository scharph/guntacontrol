// Package web embeds the compiled Angular UI (built into web/dist/web/browser) so the
// server ships it inside the binary — no -webroot flag or external files needed.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist/web/browser
var dist embed.FS

// FS returns the UI files rooted at the app shell (index.html).
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist/web/browser")
	if err != nil {
		panic(err)
	}
	return sub
}
