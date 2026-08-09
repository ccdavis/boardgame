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
		// Clicking an actionable territory legitimately opens a dialog (the
		// game opens in Purchase, so the player's own factory brings up its
		// production menu). An open modal owns the top layer and would swallow
		// the next probe, so close everything before clicking.
		if _, err := page.Evaluate(`(name) => {
			document.querySelectorAll('dialog[open]').forEach(d => d.close());
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

// TestMap_DialogsFollowTheChosenTheme measures a real dialog in a real
// browser, in both themes. A <dialog> takes its background from the user
// agent unless told otherwise, which is how every purchase and battle report
// used to arrive as a white page over a dark board.
func TestMap_DialogsFollowTheChosenTheme(t *testing.T) {
	skipIfNotBrowserTest(t)

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	// Open a dialog that exists in every phase, and read what it is painted.
	probe := `(theme) => {
		document.documentElement.dataset.theme = theme;
		const dlg = document.getElementById('noticeModal');
		if (!dlg.open) dlg.showModal();
		const lum = (css) => {
			const [r, g, b] = css.match(/[\d.]+/g).slice(0, 3).map(Number);
			const chan = (c) => {
				c /= 255;
				return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
			};
			return 0.2126 * chan(r) + 0.7152 * chan(g) + 0.0722 * chan(b);
		};
		const style = getComputedStyle(dlg);
		const body = getComputedStyle(document.body);
		const out = {
			dialogBg: lum(style.backgroundColor),
			dialogText: lum(style.color),
			bodyBg: lum(body.backgroundColor),
			sea: lum(getComputedStyle(document.querySelector('path.kind-sea')).fill)
		};
		dlg.close();
		return out;
	}`

	read := func(theme string) map[string]any {
		t.Helper()
		raw, err := page.Evaluate(probe, theme)
		if err != nil {
			t.Fatalf("probing the %s theme: %v", theme, err)
		}
		result, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("expected an object from the probe, got %T", raw)
		}
		return result
	}
	// A luminance that lands exactly on 0 or 1 crosses the bridge as an int.
	lum := func(m map[string]any, key string) float64 {
		switch v := m[key].(type) {
		case float64:
			return v
		case int:
			return float64(v)
		default:
			t.Fatalf("%s came back as %T", key, m[key])
			return 0
		}
	}

	dark := read("dark")
	if bg := lum(dark, "dialogBg"); bg > 0.25 {
		t.Errorf("dark theme: dialog background luminance %.2f, wanted a dark surface", bg)
	}
	if fg := lum(dark, "dialogText"); fg < 0.5 {
		t.Errorf("dark theme: dialog text luminance %.2f is not light enough to read on it", fg)
	}
	if bg := lum(dark, "bodyBg"); bg > 0.25 {
		t.Errorf("dark theme: page background luminance %.2f, wanted a dark page", bg)
	}

	light := read("light")
	if bg := lum(light, "dialogBg"); bg < 0.5 {
		t.Errorf("light theme: dialog background luminance %.2f, wanted a light surface", bg)
	}
	if fg := lum(light, "dialogText"); fg > 0.35 {
		t.Errorf("light theme: dialog text luminance %.2f is not dark enough to read on it", fg)
	}

	// The board is not decoration: the ocean stays navy whichever theme is on,
	// or the map would mean something different depending on a preference.
	if darkSea, lightSea := lum(dark, "sea"), lum(light, "sea"); darkSea != lightSea {
		t.Errorf("the ocean changed with the theme (%.3f vs %.3f); the map must not", darkSea, lightSea)
	}
}

// TestMap_ThemeToggleIsRememberedAcrossLoads drives the setup-screen control
// the way a player does and checks the choice survives a reload.
func TestMap_ThemeToggleIsRememberedAcrossLoads(t *testing.T) {
	skipIfNotBrowserTest(t)

	_, baseURL := startTestServer(t)
	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		Viewport: &playwright.Size{Width: 1280, Height: 800},
	})
	if err != nil {
		t.Fatalf("creating page: %v", err)
	}
	defer page.Close()

	if _, err := page.Goto(baseURL, playwright.PageGotoOptions{
		Timeout: playwright.Float(15000),
	}); err != nil {
		t.Fatalf("navigating: %v", err)
	}

	themeOf := func() string {
		raw, err := page.Evaluate(`() => document.documentElement.dataset.theme || ''`)
		if err != nil {
			t.Fatalf("reading the theme: %v", err)
		}
		return raw.(string)
	}

	if got := themeOf(); got != "dark" {
		t.Errorf("a first visit should open dark, got %q", got)
	}

	if err := page.Click("#themeLight"); err != nil {
		t.Fatalf("clicking Light: %v", err)
	}
	if got := themeOf(); got != "light" {
		t.Fatalf("clicking Light left the theme %q", got)
	}

	if _, err := page.Reload(); err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if _, err := page.WaitForSelector("#themeLight", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	}); err != nil {
		t.Fatalf("setup screen never came back: %v", err)
	}
	if got := themeOf(); got != "light" {
		t.Errorf("the chosen theme did not survive a reload, got %q", got)
	}

	// And back again, so the toggle is a toggle rather than a one-way door.
	if err := page.Click("#themeDark"); err != nil {
		t.Fatalf("clicking Dark: %v", err)
	}
	if got := themeOf(); got != "dark" {
		t.Errorf("clicking Dark left the theme %q", got)
	}
}

// TestMap_OceanRendersDark pins the water as actually painted in a browser,
// not as written in the stylesheet. Every sea zone must come out dark enough
// for the light zone borders, italic sea names and fleet badges drawn on top
// of it to read -- the arrangement that stops the map looking like a page of
// white paper with countries on it.
func TestMap_OceanRendersDark(t *testing.T) {
	skipIfNotBrowserTest(t)

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	raw, err := page.Evaluate(`() => {
		// Relative luminance, the WCAG definition.
		const lum = (css) => {
			const [r, g, b] = css.match(/[\d.]+/g).slice(0, 3).map(Number);
			const chan = (c) => {
				c /= 255;
				return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
			};
			return 0.2126 * chan(r) + 0.7152 * chan(g) + 0.0722 * chan(b);
		};

		const pale = [];
		const seas = document.querySelectorAll('path.kind-sea');
		for (const path of seas) {
			const fill = getComputedStyle(path).fill;
			if (lum(fill) > 0.25) pale.push(path.dataset.territory + ': ' + fill);
		}
		const backdrop = getComputedStyle(document.querySelector('rect.ocean-bg')).fill;
		return {
			seas: seas.length,
			pale: pale,
			backdropPale: lum(backdrop) > 0.25 ? backdrop : '',
			label: getComputedStyle(document.querySelector('text.terr-label.sea')).fill,
			labelDark: lum(getComputedStyle(document.querySelector('text.terr-label.sea')).fill) < 0.25
		};
	}`)
	if err != nil {
		t.Fatalf("measuring the ocean: %v", err)
	}

	result, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected an object from the probe, got %T", raw)
	}
	switch seas := result["seas"].(type) {
	case int:
		if seas < 1 {
			t.Fatal("no sea zones rendered at all")
		}
	case float64:
		if seas < 1 {
			t.Fatal("no sea zones rendered at all")
		}
	default:
		t.Fatalf("sea-zone count came back as %T", result["seas"])
	}
	if pale := toStrings(result["pale"]); len(pale) > 0 {
		t.Errorf("%d sea zones render pale, not dark blue: %v", len(pale), pale)
	}
	if backdrop, _ := result["backdropPale"].(string); backdrop != "" {
		t.Errorf("the backdrop behind the map renders pale (%s)", backdrop)
	}
	if dark, _ := result["labelDark"].(bool); dark {
		t.Errorf("sea names are dark (%v); they sit on dark water and must be light",
			result["label"])
	}
	t.Logf("%v sea zones render dark, sea names %v", result["seas"], result["label"])
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

// TestMap_ViewportZoomsAndPans verifies the view controls actually move the
// rendered map. The regression it guards: this page is an in-DOM Vue template,
// so ':viewBox' was lowercased by the HTML parser into an attribute SVG
// ignores -- every control mutated state faithfully while the picture never
// changed, and the previous tests, run on a viewport wide enough to show the
// unscaled geometry 1:1, could not tell the difference.
func TestMap_ViewportZoomsAndPans(t *testing.T) {
	skipIfNotBrowserTest(t)

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	readView := func() (string, float64) {
		t.Helper()
		raw, err := page.Evaluate(`() => {
			const svg = document.getElementById('gameMap');
			return { attr: svg.getAttribute('viewBox'), w: svg.viewBox.baseVal.width };
		}`)
		if err != nil {
			t.Fatalf("reading viewBox: %v", err)
		}
		entry := raw.(map[string]any)
		attr, _ := entry["attr"].(string)
		var w float64
		switch v := entry["w"].(type) {
		case float64:
			w = v
		case int:
			w = float64(v)
		}
		return attr, w
	}

	// The real, case-sensitive viewBox attribute must exist and be effective.
	attr, fullWidth := readView()
	if attr == "" {
		t.Fatal("the SVG has no viewBox attribute -- the binding is writing a dead attribute again")
	}
	if fullWidth <= 0 {
		t.Fatalf("viewBox.baseVal.width = %v; the browser is ignoring the viewBox", fullWidth)
	}

	// Zooming in must narrow the visible span of the map.
	if err := page.Locator(".map-toolbar button[title='Zoom in']").Click(); err != nil {
		t.Fatalf("clicking zoom in: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	_, zoomedWidth := readView()
	if zoomedWidth >= fullWidth {
		t.Errorf("zoom in left the visible span at %v (was %v); the control changes nothing on screen",
			zoomedWidth, fullWidth)
	}

	// Dragging must shift the viewport.
	box, err := page.Locator("#gameMap").BoundingBox()
	if err != nil {
		t.Fatalf("map bounding box: %v", err)
	}
	cx, cy := box.X+box.Width/2, box.Y+box.Height/2
	beforeDrag, _ := readView()
	page.Mouse().Move(cx, cy)
	page.Mouse().Down()
	page.Mouse().Move(cx-120, cy-60, playwright.MouseMoveOptions{Steps: playwright.Int(8)})
	page.Mouse().Up()
	time.Sleep(200 * time.Millisecond)
	afterDrag, _ := readView()
	if afterDrag == beforeDrag {
		t.Errorf("dragging the map left the viewport at %q; pan does nothing", beforeDrag)
	}

	// Fit restores the whole world.
	if err := page.Locator(".map-toolbar button[title='Fit the whole map']").Click(); err != nil {
		t.Fatalf("clicking fit: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, w := readView(); w != fullWidth {
		t.Errorf("Fit shows a span of %v, want the full %v", w, fullWidth)
	}
}

// TestMap_WholeWorldReachableInANarrowWindow plays the report that found the
// dead viewBox: pick Japan in a laptop-sized window. Without a working
// viewBox the geometry rendered 1:1, everything east of the container's edge
// -- Japan included -- was clipped into unreachability, and its owner could
// not select a single one of their own territories.
func TestMap_WholeWorldReachableInANarrowWindow(t *testing.T) {
	skipIfNotBrowserTest(t)

	_, baseURL := startTestServer(t)

	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		Viewport: &playwright.Size{Width: 1280, Height: 800},
	})
	if err != nil {
		t.Fatalf("creating page: %v", err)
	}
	defer page.Close()

	if _, err := page.Goto(baseURL, playwright.PageGotoOptions{
		Timeout: playwright.Float(15000),
	}); err != nil {
		t.Fatalf("navigating: %v", err)
	}
	if err := page.Fill("#gdfPath", "../../aaa.gdf"); err != nil {
		t.Fatalf("filling board path: %v", err)
	}
	if _, err := page.Locator("#playerName").SelectOption(playwright.SelectOptionValues{
		Values: &[]string{"Japan"},
	}); err != nil {
		t.Fatalf("selecting Japan: %v", err)
	}
	if err := page.Click("button[type=submit]"); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	if _, err := page.WaitForSelector("#gameMap", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(15000),
	}); err != nil {
		t.Fatalf("map never rendered: %v", err)
	}
	dismiss := page.Locator("button:has-text('Got it!')")
	if n, _ := dismiss.Count(); n > 0 {
		dismiss.First().Click(playwright.LocatorClickOptions{Timeout: playwright.Float(5000)})
	}
	time.Sleep(300 * time.Millisecond)

	// A plain click -- no force, no scripted dispatch. If Japan is clipped
	// out of the container this times out exactly the way a mouse fails.
	if err := page.Locator(`path[data-territory="Japan"]`).Click(playwright.LocatorClickOptions{
		Timeout: playwright.Float(5000),
	}); err != nil {
		t.Fatalf("Japan cannot be clicked in a 1280px window: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	got, err := page.Evaluate(`() => vueApp ? vueApp.selectedTerritory : null`)
	if err != nil {
		t.Fatalf("reading selection: %v", err)
	}
	if got != "Japan" {
		t.Errorf("clicked Japan but the app selected %v", got)
	}
}
