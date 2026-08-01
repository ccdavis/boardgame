package webserver

// End-to-end walk of the map-driven interaction: purchase from a clicked
// factory, pick units and a destination on the map, fight the resulting
// battle on the battle screen. Set UX_SHOT_DIR to also capture a screenshot
// of each stage.

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

func TestE2E_MapDrivenFlow(t *testing.T) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		t.Skip("Skipping E2E test (set RUN_BROWSER_TESTS=1 to run)")
	}

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	dir := os.Getenv("UX_SHOT_DIR")
	shot := func(name string) {
		t.Helper()
		time.Sleep(400 * time.Millisecond)
		if dir == "" {
			return
		}
		if _, err := page.Screenshot(playwright.PageScreenshotOptions{
			Path: playwright.String(fmt.Sprintf("%s/%s.png", dir, name)),
		}); err != nil {
			t.Fatalf("screenshot %s: %v", name, err)
		}
	}
	eval := func(js string, args ...any) {
		t.Helper()
		if _, err := page.Evaluate(js, args...); err != nil {
			t.Fatalf("eval failed: %v\n%s", err, js)
		}
	}

	closeGuidance(page)
	shot("1-purchase-highlights")

	// Click the German factory: production menu.
	eval(`() => document.querySelector('path[data-territory="Germany"]')
		.dispatchEvent(new MouseEvent('click', {bubbles: true}))`)
	shot("2-purchase-menu")
	eval(`() => document.querySelectorAll('dialog[open]').forEach(d => d.close())`)

	// On to Combat Move.
	clickAction(t, page, "Done Purchasing")
	expectPhase(t, page, "Combat Move")
	closeGuidance(page)
	shot("3-combatmove-highlights")

	// Click Eastern Europe: unit picker.
	eval(`() => document.querySelector('path[data-territory="Eastern Europe"]')
		.dispatchEvent(new MouseEvent('click', {bubbles: true}))`)
	if err := page.Locator("#unitPickerModal").WaitFor(playwright.LocatorWaitForOptions{
		State: playwright.WaitForSelectorStateVisible, Timeout: playwright.Float(5000),
	}); err != nil {
		t.Fatalf("unit picker never opened: %v", err)
	}
	shot("4-unit-picker")

	// OK: destination targeting.
	if err := page.Locator("#unitPickerModal button:has-text('OK')").Click(); err != nil {
		t.Fatalf("confirming picker: %v", err)
	}
	time.Sleep(800 * time.Millisecond)
	shot("5-pick-destination")

	// A bad click flashes red.
	eval(`() => document.querySelector('path[data-territory="Brazil"]')
		.dispatchEvent(new MouseEvent('click', {bubbles: true}))`)
	time.Sleep(150 * time.Millisecond)
	shot("6-bad-click-flash")

	// Commit the attack into Karelia.
	eval(`() => document.querySelector('path[data-territory="Karelia"]')
		.dispatchEvent(new MouseEvent('click', {bubbles: true}))`)
	time.Sleep(800 * time.Millisecond)
	if text, _ := page.Locator(".action-bar").TextContent(); !containsMovesPlanned(text) {
		t.Fatalf("destination click planned no moves; action bar shows %q", text)
	}
	shot("7-move-arrow")

	// Fight it.
	clickAction(t, page, "Execute Moves")
	expectPhase(t, page, "Conduct Combat")
	closeGuidance(page)
	shot("8-battle-marker")

	eval(`() => document.querySelector('path[data-territory="Karelia"]')
		.dispatchEvent(new MouseEvent('click', {bubbles: true}))`)
	if err := page.Locator("#battleModal").WaitFor(playwright.LocatorWaitForOptions{
		State: playwright.WaitForSelectorStateVisible, Timeout: playwright.Float(5000),
	}); err != nil {
		t.Fatalf("battle screen never opened: %v", err)
	}
	shot("9-battle-screen")

	if err := page.Locator("#battleModal button:has-text('Fight it out')").Click(); err != nil {
		t.Fatalf("fighting battle: %v", err)
	}
	time.Sleep(1200 * time.Millisecond)
	if text, _ := page.Locator("#battleModal .battle-result").TextContent(); !strings.Contains(text, "round") {
		t.Fatalf("battle produced no result summary; dialog shows %q", text)
	}
	shot("10-battle-result")
}

func containsMovesPlanned(text string) bool {
	return strings.Contains(text, "moves planned") || strings.Contains(text, "1 move planned")
}
