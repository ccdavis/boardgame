package webserver

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// TestBrowser_MapClickable tests that the map has clickable territory regions
func TestBrowser_MapClickable(t *testing.T) {
	skipIfNotBrowserTest(t)

	testGDFPath := createMinimalTestGame(t)
	defer os.Remove(testGDFPath)

	_, baseURL := startTestServer(t)

	page, err := browser.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// Navigate and start game
	if _, err := page.Goto(baseURL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Fill in form and start game
	gdfInput := page.Locator("#gdfPath")
	if err := gdfInput.Fill(testGDFPath); err != nil {
		t.Fatalf("Failed to fill GDF path: %v", err)
	}

	playerSelect := page.Locator("#playerName")
	if _, err := playerSelect.SelectOption(playwright.SelectOptionValues{
		Values: &[]string{"Germany"},
	}); err != nil {
		t.Fatalf("Failed to select player: %v", err)
	}

	startButton := page.Locator("button[type='submit']")
	if err := startButton.Click(); err != nil {
		t.Fatalf("Failed to click start: %v", err)
	}

	// Listen for console messages
	page.On("console", func(msg playwright.ConsoleMessage) {
		t.Logf("Browser console [%s]: %s", msg.Type(), msg.Text())
	})
	page.On("pageerror", func(err error) {
		t.Logf("Browser error: %v", err)
	})

	// Wait for game to load
	time.Sleep(2 * time.Second)

	// Check that TERRITORY_COORDS is loaded
	coordsLoaded, err := page.Evaluate("typeof window.TERRITORY_COORDS !== 'undefined'", nil)
	if err != nil {
		t.Fatalf("Failed to check TERRITORY_COORDS: %v", err)
	}
	if !coordsLoaded.(bool) {
		t.Error("TERRITORY_COORDS not loaded - check if territory-coords.js is being served")
	} else {
		// Check how many territories are in TERRITORY_COORDS
		coordsCount, _ := page.Evaluate("Object.keys(window.TERRITORY_COORDS).length", nil)
		t.Logf("✓ TERRITORY_COORDS loaded successfully with %v territories", coordsCount)
	}

	// Check that SVG overlay exists
	overlay := page.Locator("#mapOverlay")
	if visible, err := overlay.IsVisible(); err != nil || !visible {
		t.Fatal("Map overlay SVG not visible")
	}
	t.Log("✓ Map overlay SVG is visible")

	// Check if vue app is available
	vueAppExists, _ := page.Evaluate("typeof window.vueApp !== 'undefined'", nil)
	t.Logf("Vue app exists: %v", vueAppExists)

	// Manually call initializeMapOverlay to ensure it runs
	result, err := page.Evaluate("() => { if (window.vueApp) { window.vueApp.initializeMapOverlay(); return 'called'; } else { return 'vueApp not found'; } }", nil)
	if err != nil {
		t.Logf("Error calling initializeMapOverlay: %v", err)
	} else {
		t.Logf("Manual initializeMapOverlay call: %v", result)
	}

	time.Sleep(500 * time.Millisecond)

	// Check that territory circles were created
	circles := page.Locator("#mapOverlay circle[data-territory]")
	count, err := circles.Count()
	if err != nil {
		t.Fatalf("Failed to count territory circles: %v", err)
	}

	t.Logf("Found %d clickable territory regions on map", count)

	if count == 0 {
		// Get more diagnostic info
		overlayHTML, _ := overlay.InnerHTML()
		t.Logf("Map overlay HTML: %s", overlayHTML)

		// Check console for errors
		t.Fatal("No clickable territory regions found on map")
	}

	// Try to click on a territory region (first available one)
	firstCircle := circles.First()
	territoryName, err := firstCircle.GetAttribute("data-territory")
	if err != nil {
		t.Fatalf("Failed to get territory name: %v", err)
	}

	t.Logf("Attempting to click on territory: %s", territoryName)

	// Get circle attributes for debugging
	cx, _ := firstCircle.GetAttribute("cx")
	cy, _ := firstCircle.GetAttribute("cy")
	r, _ := firstCircle.GetAttribute("r")
	t.Logf("Circle attributes: cx=%s, cy=%s, r=%s", cx, cy, r)

	// Click the circle
	if err := firstCircle.Click(playwright.LocatorClickOptions{
		Force: playwright.Bool(true),
	}); err != nil {
		t.Fatalf("Failed to click territory circle: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Verify that the territory was selected in the action sidebar
	actionSidebar := page.Locator(".action-sidebar h2")
	sidebarText, err := actionSidebar.TextContent()
	if err != nil {
		t.Fatalf("Failed to get sidebar text: %v", err)
	}

	if sidebarText != territoryName {
		t.Errorf("Territory not selected correctly. Expected '%s', got '%s'", territoryName, sidebarText)
	} else {
		t.Logf("✓ Successfully clicked on territory '%s' and it was selected", territoryName)
	}
}

// Helper function to create a minimal test game file for browser tests
func createMinimalTestGame(t *testing.T) string {
	tmpDir := os.TempDir()
	testFile := filepath.Join(tmpDir, "browser_test_game.gdf")

	content := `Players
  Germany, USSR;

Turn 1;

Territories
  Berlin :land, Germany, 10;
  Moscow :land, USSR, 8;
  Poland :land, Germany, 2;

Map
  Berlin: Poland;
  Moscow: Poland;
  Poland: Berlin, Moscow;

Units
  infantry: land, 1 movement, 1 attack, 2 defend, 3 cost;

Containers

Placement
  Berlin: 2 infantry;
  Moscow: 2 infantry;
  Poland: 1 infantry;
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	return testFile
}
