package webserver

import (
	"embed"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// The frontend is compiled into the binary. There is then exactly one copy of
// the CSS, JS and HTML in play, it cannot be missing, and it cannot vary with
// the directory the server happens to be started from -- the old code guessed
// between ./webserver/static and ./static and silently served nothing when
// neither matched.
//
// Editing still works the way it always did: `go run ./cmd/webserver` rebuilds
// from source, so a changed stylesheet is in the next run.
//
//go:embed static
var embeddedStatic embed.FS

// Asset caching is switched off entirely, and the page's own asset URLs are
// stamped with a fresh token on every load.
//
// This is a local game server: refetching ~100KB of CSS and JS costs nothing,
// while a stale copy costs a player their afternoon. `Cache-Control: no-cache`
// was not enough on its own -- it only asks the browser to revalidate, and a
// browser holding a heuristically-cached copy from before that header existed
// never asks. The version stamp closes the hole from the other side:
// /css/style.css?v=<token> is a URL the cache has never seen, so there is
// nothing to serve stale.
//
// Anything under /css/ or /js/ referenced by index.html gets the stamp, so new
// assets are covered automatically.
var assetRefs = regexp.MustCompile(`(href|src)="(/(?:css|js)/[^"?]+)"`)

// staticAssets is the embedded frontend rooted at the static directory, so
// "/css/style.css" is the path both in the URL and in the filesystem.
func staticAssets() fs.FS {
	sub, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		// Unreachable: the embed directive above guarantees the directory
		// exists, and a typo there is a compile error, not a runtime one.
		panic("embedded static assets are missing: " + err.Error())
	}
	return sub
}

// staticHandler serves the frontend from the binary, fresh on every load.
func staticHandler() http.Handler {
	assets := staticAssets()
	files := http.FileServer(http.FS(assets))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		noStore(w)

		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			if serveStampedIndex(w, assets) {
				return
			}
		}

		// A conditional request can only come from a cache we have told not to
		// exist. Dropping the headers keeps the file server from answering 304
		// Not Modified against it, so the response is always the real bytes.
		r.Header.Del("If-Modified-Since")
		r.Header.Del("If-None-Match")
		files.ServeHTTP(w, r)
	})
}

// serveStampedIndex writes index.html with a per-load version stamp on every
// stylesheet and script it pulls in. Reports whether it served anything.
func serveStampedIndex(w http.ResponseWriter, assets fs.FS) bool {
	page, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return false
	}

	// Per load, not per server start: a token the browser has never seen
	// cannot resolve to a cached response, whenever that cache was filled.
	stamp := []byte(`$1="$2?v=` + strconv.FormatInt(time.Now().UnixNano(), 36) + `"`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(assetRefs.ReplaceAll(page, stamp))
	return true
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}
