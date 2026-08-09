package webserver

// End-to-end walk of the naval interaction the player found impenetrable:
// putting troops aboard a transport, reading what a ship is carrying from the
// sea-zone picker, sending that cargo ashore, and being asked before an
// aircraft commits itself to a carrier deck.
//
// Set UX_SHOT_DIR to also capture a screenshot of each stage.

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// clickTerritory clicks a territory's path on the map, the way the player does.
func clickTerritory(t *testing.T, page playwright.Page, name string) {
	t.Helper()
	_, err := page.Evaluate(`(name) => {
		const path = document.querySelector('path[data-territory="' + name + '"]');
		if (!path) throw new Error('no territory path for ' + name);
		path.dispatchEvent(new MouseEvent('click', {bubbles: true}));
	}`, name)
	if err != nil {
		t.Fatalf("clicking %s: %v", name, err)
	}
	time.Sleep(600 * time.Millisecond)
}

func waitForDialog(t *testing.T, page playwright.Page, selector, what string) {
	t.Helper()
	if err := page.Locator(selector).WaitFor(playwright.LocatorWaitForOptions{
		State: playwright.WaitForSelectorStateVisible, Timeout: playwright.Float(5000),
	}); err != nil {
		t.Fatalf("%s never appeared: %v", what, err)
	}
}

func TestE2E_LoadCarryAndPutAshore(t *testing.T) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		t.Skip("Skipping E2E test (set RUN_BROWSER_TESTS=1 to run)")
	}

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	dir := os.Getenv("UX_SHOT_DIR")
	shot := func(name string) {
		t.Helper()
		time.Sleep(300 * time.Millisecond)
		if dir == "" {
			return
		}
		if _, err := page.Screenshot(playwright.PageScreenshotOptions{
			Path: playwright.String(fmt.Sprintf("%s/%s.png", dir, name)),
		}); err != nil {
			t.Fatalf("screenshot %s: %v", name, err)
		}
	}

	closeGuidance(page)
	clickAction(t, page, "Done Purchasing")
	expectPhase(t, page, "Combat Move")
	closeGuidance(page)

	// --- Put troops aboard -------------------------------------------------
	// Germany stands on the Baltic, where its transport is.
	clickTerritory(t, page, "Germany")
	waitForDialog(t, page, "#unitPickerModal", "the unit picker")

	// Take two infantry and nothing else: the whole garrison would not fit,
	// and the point here is the boarding, not the refusal.
	if _, err := page.Evaluate(`() => {
		const app = window.vueApp;
		for (const g of app.unitPicker.groups) g.take = (g.type === 'infantry') ? 2 : 0;
	}`); err != nil {
		t.Fatalf("trimming the picker: %v", err)
	}
	if err := page.Locator("#unitPickerModal button:has-text('OK')").Click(); err != nil {
		t.Fatalf("confirming picker: %v", err)
	}
	time.Sleep(800 * time.Millisecond)
	shot("n1-boarding-destinations")

	clickTerritory(t, page, "Baltic Sea")
	loaded, err := page.Evaluate(`async () => {
		const r = await window.vueApp.api.getTerritoryDetails('Baltic Sea');
		return r.units.filter(u => u.aboard).length;
	}`)
	if err != nil {
		t.Fatalf("reading the Baltic: %v", err)
	}
	if n, ok := loaded.(int); !ok || n != 2 {
		t.Fatalf("expected 2 infantry aboard after boarding, got %v", loaded)
	}

	// --- The sea-zone picker names the cargo -------------------------------
	clickTerritory(t, page, "Baltic Sea")
	waitForDialog(t, page, "#unitPickerModal", "the sea-zone picker")
	shot("n2-sea-zone-picker")

	body, err := page.Locator("#unitPickerModal").TextContent()
	if err != nil {
		t.Fatalf("reading the picker: %v", err)
	}
	if !strings.Contains(body, "carrying 2 infantry") {
		t.Errorf("the loaded transport does not say what it carries:\n%s", body)
	}
	if !strings.Contains(body, "Put ashore") {
		t.Errorf("no way to land the cargo from the ship's own row:\n%s", body)
	}

	// Cargo must not appear as movable units of its own -- that was the old
	// picker's central confusion.
	rows, err := page.Evaluate(`() => window.vueApp.unitPicker.groups.map(g => g.key)`)
	if err != nil {
		t.Fatalf("reading picker rows: %v", err)
	}
	if fmt.Sprint(rows) == "" {
		t.Fatal("the sea-zone picker is empty")
	}
	infantryRow, err := page.Evaluate(
		`() => window.vueApp.unitPicker.groups.some(g => g.type === 'infantry')`)
	if err != nil {
		t.Fatalf("checking for cargo rows: %v", err)
	}
	if infantryRow == true {
		t.Error("cargo was offered as a unit to move; it sails with its ship")
	}

	// --- Put it ashore -----------------------------------------------------
	if err := page.Locator("#unitPickerModal button:has-text('Put ashore')").First().Click(); err != nil {
		t.Fatalf("clicking Put ashore: %v", err)
	}
	time.Sleep(900 * time.Millisecond)
	shot("n3-unload-destinations")

	banner, err := page.Locator(".action-bar").TextContent()
	if err != nil {
		t.Fatalf("reading the action bar: %v", err)
	}
	if !strings.Contains(banner, "2 infantry from the transport") {
		t.Errorf("the banner does not say what is being landed: %q", banner)
	}

	// Karelia is Soviet ground, reachable from the Baltic: an assault landing.
	dests, err := page.Evaluate(`() => Object.entries(window.vueApp.ui.eligibleDest)
		.map(([name, d]) => name + (d.isAttack ? ':attack' : ':friendly'))`)
	if err != nil {
		t.Fatalf("reading destinations: %v", err)
	}
	list := fmt.Sprint(dests)
	if !strings.Contains(list, "Karelia:attack") {
		t.Errorf("an opposed landing was not offered during combat move: %v", list)
	}

	clickTerritory(t, page, "Karelia")
	time.Sleep(900 * time.Millisecond)
	shot("n4-landing-booked")

	planned, err := page.Evaluate(
		`() => window.vueApp.plannedMoves.filter(m => m.landing && m.to === 'Karelia').length`)
	if err != nil {
		t.Fatalf("reading planned moves: %v", err)
	}
	if n, ok := planned.(int); !ok || n != 2 {
		t.Fatalf("expected 2 booked landings in Karelia, got %v", planned)
	}
}

// The reported bug, end to end: flying an aircraft at a carrier must ask before
// it commits, and cancelling must leave the player still choosing.
func TestE2E_CarrierLandingAsksFirst(t *testing.T) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		t.Skip("Skipping E2E test (set RUN_BROWSER_TESTS=1 to run)")
	}

	_, baseURL := startTestServer(t)
	page := startGameInBrowser(t, baseURL)
	defer page.Close()

	closeGuidance(page)
	clickAction(t, page, "Done Purchasing")
	expectPhase(t, page, "Combat Move")
	closeGuidance(page)

	// Germany fields no carrier on this board, so the deck is supplied here.
	// Everything else is real: a real fighter, a real sea zone, and a real
	// move booked through the server if the player says yes.
	if _, err := page.Evaluate(`async () => {
		const app = window.vueApp;
		const here = await app.api.getTerritoryDetails('Germany');
		const fighter = here.units.find(u => u.name === 'fighter' && u.owner === 'Germany');
		if (!fighter) throw new Error('no German fighter to fly');
		app.ui.mode = 'pickDest';
		app.ui.source = 'Germany';
		app.ui.picked = [fighter.id];
		app.ui.pickedLabel = '1 fighter';
		app.ui.eligibleDest = {
			'Baltic Sea': {
				name: 'Baltic Sea', distance: 1, owner: 'Neutral',
				isCarrier: true, note: 'carrier in Baltic Sea — 2 deck spaces free'
			}
		};
	}`); err != nil {
		t.Fatalf("setting up destination mode: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	clickTerritory(t, page, "Baltic Sea")
	waitForDialog(t, page, "#confirmModal", "the carrier confirmation")

	text, err := page.Locator("#confirmModal").TextContent()
	if err != nil {
		t.Fatalf("reading the confirmation: %v", err)
	}
	for _, want := range []string{"2 deck spaces free", "lost"} {
		if !strings.Contains(text, want) {
			t.Errorf("the confirmation does not mention %q:\n%s", want, text)
		}
	}

	// Cancel: nothing is booked, and the player is still picking a destination.
	if err := page.Locator("#confirmModal button:has-text('Cancel')").Click(); err != nil {
		t.Fatalf("cancelling: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	mode, err := page.Evaluate(`() => window.vueApp.ui.mode`)
	if err != nil {
		t.Fatalf("reading interaction mode: %v", err)
	}
	if mode != "pickDest" {
		t.Errorf("cancelling the landing threw away the selection; mode is %v", mode)
	}
	if n, _ := page.Evaluate(`() => window.vueApp.plannedMoves.length`); n != 0 {
		t.Errorf("a cancelled landing was booked anyway: %v moves planned", n)
	}

	// Say yes this time: the move is booked and targeting ends.
	clickTerritory(t, page, "Baltic Sea")
	waitForDialog(t, page, "#confirmModal", "the carrier confirmation")
	if err := page.Locator("#confirmModal button:has-text('Land on carrier')").Click(); err != nil {
		t.Fatalf("confirming the landing: %v", err)
	}
	time.Sleep(1000 * time.Millisecond)

	booked, err := page.Evaluate(
		`() => window.vueApp.plannedMoves.filter(m => m.to === 'Baltic Sea').length`)
	if err != nil {
		t.Fatalf("reading planned moves: %v", err)
	}
	if n, ok := booked.(int); !ok || n != 1 {
		t.Fatalf("confirming the landing booked nothing; planned moves to the Baltic: %v", booked)
	}
	if mode, _ := page.Evaluate(`() => window.vueApp.ui.mode`); mode != "idle" {
		t.Errorf("targeting did not end after the landing was booked; mode is %v", mode)
	}
}
