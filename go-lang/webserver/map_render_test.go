package webserver

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// startGameInBrowser loads the app, starts a game and waits for the map.
func startGameInBrowser(t *testing.T, baseURL string) playwright.Page {
	t.Helper()

	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		Viewport: &playwright.Size{Width: 1920, Height: 1080},
	})
	if err != nil {
		t.Fatalf("creating page: %v", err)
	}
	page.On("pageerror", func(err error) { t.Errorf("uncaught browser error: %v", err) })

	if _, err := page.Goto(baseURL, playwright.PageGotoOptions{
		Timeout: playwright.Float(15000),
	}); err != nil {
		t.Fatalf("navigating to %s: %v", baseURL, err)
	}

	if err := page.Fill("#gdfPath", "../../aaa.gdf"); err != nil {
		t.Fatalf("filling board path: %v", err)
	}
	if err := page.Click("button[type=submit]"); err != nil {
		t.Fatalf("submitting setup form: %v", err)
	}
	if _, err := page.WaitForSelector("#gameMap", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(15000),
	}); err != nil {
		t.Fatalf("map never rendered: %v", err)
	}

	// Dismiss the phase guidance dialog so it does not sit over the map.
	dismiss := page.Locator("button:has-text('Got it!')")
	if n, _ := dismiss.Count(); n > 0 {
		if err := dismiss.First().Click(playwright.LocatorClickOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			t.Fatalf("could not dismiss the phase dialog -- is something covering it? %v", err)
		}
	}
	time.Sleep(400 * time.Millisecond)
	return page
}

// TestMap_ClickAnchorSelectsTerritory is the regression test the previous
// implementation never had, and the reason the map is now trustworthy.
//
// For every territory it converts that territory's own label anchor into
// viewport pixels and asks the browser what sits at that point. If geometry and
// hit regions ever drift apart -- the failure that made the old coordinate
// overlay useless -- this reports exactly which territory hits which, instead of
// looking fine and behaving wrongly.
func TestMap_ClickAnchorSelectsTerritory(t *testing.T) {
	skipIfNotBrowserTest(t)

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	raw, err := page.Evaluate(`() => {
		const svg = document.getElementById('gameMap');
		const ctm = svg.getScreenCTM();
		const out = [];
		for (const name of Object.keys(MAP.byName)) {
			const geo = MAP.byName[name];
			const pt = svg.createSVGPoint();
			pt.x = geo.labelX; pt.y = geo.labelY;
			const screen = pt.matrixTransform(ctm);
			const hit = document.elementFromPoint(screen.x, screen.y);
			out.push({
				name: name,
				hit: hit ? (hit.getAttribute('data-territory') || hit.tagName) : 'nothing'
			});
		}
		return out;
	}`)
	if err != nil {
		t.Fatalf("probing anchors: %v", err)
	}

	rows, ok := raw.([]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("expected a list of probe results, got %T", raw)
	}

	var wrong []string
	for _, row := range rows {
		entry := row.(map[string]any)
		name, _ := entry["name"].(string)
		hit, _ := entry["hit"].(string)
		if name != hit {
			wrong = append(wrong, fmt.Sprintf("clicking %q hits %q", name, hit))
		}
	}
	sort.Strings(wrong)

	t.Logf("probed %d territories", len(rows))
	if len(wrong) > 0 {
		t.Errorf("%d of %d territory anchors select the wrong region:", len(wrong), len(rows))
		for _, line := range wrong {
			t.Errorf("  %s", line)
		}
	}
}

// TestMap_SelectingTerritoryUpdatesApp checks the click actually drives the app,
// not merely that the right element is under the cursor.
func TestMap_SelectingTerritoryUpdatesApp(t *testing.T) {
	skipIfNotBrowserTest(t)

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	for _, name := range []string{"Germany", "Russia", "Japan", "Brazil"} {
		if _, err := page.Evaluate(`(name) => {
			const svg = document.getElementById('gameMap');
			const ctm = svg.getScreenCTM();
			const geo = MAP.byName[name];
			const pt = svg.createSVGPoint();
			pt.x = geo.labelX; pt.y = geo.labelY;
			const s = pt.matrixTransform(ctm);
			document.elementFromPoint(s.x, s.y).dispatchEvent(
				new MouseEvent('click', {bubbles: true}));
		}`, name); err != nil {
			t.Fatalf("clicking %s: %v", name, err)
		}

		time.Sleep(250 * time.Millisecond)
		got, err := page.Evaluate(`() => vueApp ? vueApp.selectedTerritory : null`)
		if err != nil {
			t.Fatalf("reading selection: %v", err)
		}
		if got != name {
			t.Errorf("clicked %q but the app selected %v", name, got)
		}
	}
}

// TestMap_EveryTerritoryHasNonZeroArea uses the browser's own geometry engine,
// so it catches malformed path data that the Go-side validator would accept.
func TestMap_EveryTerritoryHasNonZeroArea(t *testing.T) {
	skipIfNotBrowserTest(t)

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	raw, err := page.Evaluate(`() => {
		const bad = [];
		let count = 0;
		for (const path of document.querySelectorAll('#landLayer path, #seaLayer path')) {
			count++;
			const box = path.getBBox();
			if (!(box.width > 0.5 && box.height > 0.5)) {
				bad.push(path.getAttribute('data-territory') + ' ' + box.width + 'x' + box.height);
			}
		}
		return {count: count, bad: bad};
	}`)
	if err != nil {
		t.Fatalf("measuring territories: %v", err)
	}

	result, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected an object from the probe, got %T", raw)
	}
	count := toInt(result["count"])
	if count == 0 {
		t.Fatal("no territory paths rendered at all")
	}
	if bad := toStrings(result["bad"]); len(bad) > 0 {
		t.Errorf("%d territories have a degenerate bounding box: %v", len(bad), bad)
	}
	t.Logf("%d territory paths rendered", count)
}

// TestMap_NoTwoLandPolygonsOverlap asks the real hit-tester whether more than
// one land region claims the same point, which the Go validator's sampling
// could in principle miss.
func TestMap_NoTwoLandPolygonsOverlap(t *testing.T) {
	skipIfNotBrowserTest(t)

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	raw, err := page.Evaluate(`() => {
		const svg = document.getElementById('gameMap');
		const ctm = svg.getScreenCTM();
		const clashes = [];
		for (const name of Object.keys(MAP.byName)) {
			const geo = MAP.byName[name];
			if (geo.isSea) continue;
			const pt = svg.createSVGPoint();
			pt.x = geo.labelX; pt.y = geo.labelY;
			const s = pt.matrixTransform(ctm);
			const land = document.elementsFromPoint(s.x, s.y)
				.filter(el => el.closest && el.closest('#landLayer'));
			if (land.length > 1) {
				clashes.push(name + ' -> ' + land.map(
					el => el.getAttribute('data-territory')).join(', '));
			}
		}
		return clashes;
	}`)
	if err != nil {
		t.Fatalf("checking overlaps: %v", err)
	}

	if clashes := toStrings(raw); len(clashes) > 0 {
		t.Errorf("%d land anchors are covered by more than one region: %v",
			len(clashes), clashes)
	}
}

// TestMap_OwnershipColoursFollowGameState guards the binding between game state
// and CSS classes. A renamed class would leave the map looking plausible while
// showing the wrong owner.
func TestMap_OwnershipColoursFollowGameState(t *testing.T) {
	skipIfNotBrowserTest(t)

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	raw, err := page.Evaluate(`() => {
		const wrong = [];
		let checked = 0;
		for (const terr of vueApp.territories) {
			const path = document.querySelector(
				'[data-territory="' + CSS.escape(terr.name) + '"]');
			if (!path) { wrong.push(terr.name + ': no path'); continue; }
			checked++;
			const expected = 'owner-' + terr.owner.toLowerCase().replace(/\s+/g, '-');
			if (!path.classList.contains(expected)) {
				wrong.push(terr.name + ': owned by ' + terr.owner +
					' but classes are ' + path.getAttribute('class'));
			}
		}
		return {checked: checked, wrong: wrong};
	}`)
	if err != nil {
		t.Fatalf("checking ownership classes: %v", err)
	}

	result, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected an object from the probe, got %T", raw)
	}
	if wrong := toStrings(result["wrong"]); len(wrong) > 0 {
		t.Errorf("%d territories carry the wrong owner class: %v", len(wrong), wrong)
	}
	t.Logf("checked %v territories", result["checked"])
}

// TestMap_SidebarAndMapShareSelection pins the structural win of the rewrite:
// the sidebar button and the map path call one selectTerritory, so the two can
// never disagree about what is selected.
func TestMap_SidebarAndMapShareSelection(t *testing.T) {
	skipIfNotBrowserTest(t)

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	if _, err := page.Evaluate(`() => vueApp.selectTerritory('Germany')`); err != nil {
		t.Fatalf("selecting from script: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	raw, err := page.Evaluate(`() => {
		const path = document.querySelector('[data-territory="Germany"]');
		return {
			selected: vueApp.selectedTerritory,
			pathSelected: path ? path.classList.contains('selected') : false
		};
	}`)
	if err != nil {
		t.Fatalf("reading selection state: %v", err)
	}

	result, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected an object from the probe, got %T", raw)
	}
	if result["selected"] != "Germany" {
		t.Errorf("app selection is %v, want Germany", result["selected"])
	}
	if result["pathSelected"] != true {
		t.Error("the map path for Germany is not marked selected")
	}
}

// toStrings normalises whatever Playwright hands back for a JS array. An empty
// array can arrive as nil rather than an empty slice, so a bare type assertion
// panics exactly when the test should be passing.
func toStrings(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

// toInt normalises a JS number. Playwright hands whole numbers back as int
// rather than float64, so asserting on one type alone silently reads zero.
func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}
