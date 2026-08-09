package webserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The player's browser must never be able to answer a page load from a copy of
// the CSS or JS it fetched during an earlier game.
func TestStatic_AssetsAreNeverServedFromCache(t *testing.T) {
	handler := staticHandler()

	for _, path := range []string{"/", "/css/style.css", "/js/app.js"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status %d, want 200", path, rec.Code)
			continue
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Errorf("GET %s: Cache-Control %q, want it to contain no-store", path, cc)
		}
	}
}

// A cache that ignores the headers still cannot win: the URLs change per load.
func TestStatic_IndexStampsItsAssetURLs(t *testing.T) {
	handler := staticHandler()

	get := func() string {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		return rec.Body.String()
	}

	first, second := get(), get()

	stamped := regexp.MustCompile(`(?:href|src)="(/(?:css|js)/[^"]+)"`)
	refs := stamped.FindAllStringSubmatch(first, -1)
	if len(refs) < 3 {
		t.Fatalf("expected the page to pull in at least 3 local assets, found %d", len(refs))
	}
	for _, ref := range refs {
		if !strings.Contains(ref[1], "?v=") {
			t.Errorf("asset %q was served without a version stamp", ref[1])
		}
	}

	if first == second {
		t.Error("two page loads produced identical asset URLs; a stale cached copy would still be reachable")
	}
}

// A conditional request must not be answered with 304 against a cache we have
// told the browser not to keep.
func TestStatic_ConditionalRequestStillGetsTheBytes(t *testing.T) {
	handler := staticHandler()

	req := httptest.NewRequest("GET", "/css/style.css", nil)
	req.Header.Set("If-Modified-Since", "Mon, 02 Jan 2040 15:04:05 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 with the real stylesheet", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "--sea:") {
		t.Error("response was not the stylesheet")
	}
}

// The frontend travels inside the binary, so where the server was started from
// cannot change what a player sees. The old code guessed between two relative
// directories and served nothing at all when neither matched.
func TestStatic_ServedFromAnyWorkingDirectory(t *testing.T) {
	was, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(was)

	handler := staticHandler()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/css/style.css", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d from an unrelated working directory, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "--sea:") {
		t.Error("the stylesheet served from an unrelated directory was not ours")
	}
}

// The map paints its own dark theme. Declaring that keeps a browser or an
// extension from inverting it and turning the ocean white -- the failure that
// looked for all the world like a caching bug.
func TestStatic_PageDeclaresItsOwnDarkTheme(t *testing.T) {
	handler := staticHandler()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	page := rec.Body.String()
	for _, want := range []string{`name="color-scheme"`, `name="darkreader-lock"`} {
		if !strings.Contains(page, want) {
			t.Errorf("page does not carry <meta %s>", want)
		}
	}
}
