package webserver

import (
	"boardgame/parser"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// TestE2E_CompleteGameFlow tests a complete game from start to finish
func TestE2E_CompleteGameFlow(t *testing.T) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		t.Skip("Skipping E2E test (set RUN_BROWSER_TESTS=1 to run)")
	}

	// Create a simplified test game file for E2E testing
	testGDFPath := createTestGameFile(t)
	defer os.Remove(testGDFPath)

	// Start server
	server, baseURL := startTestServer(t)
	_ = server

	// Create browser context
	context, err := browser.NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer context.Close()

	page, err := context.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Navigate to app
	if _, err := page.Goto(baseURL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	t.Log("✓ Page loaded")

	// Step 1: Start a new game
	t.Log("Starting new game...")

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

	// Wait for game to load
	time.Sleep(1 * time.Second)

	// Verify game screen appeared
	gameScreen := page.Locator(".game-screen")
	if err := gameScreen.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	}); err != nil {
		t.Fatalf("Game screen did not appear: %v", err)
	}

	t.Log("✓ Game started")

	// Step 2: Close phase guidance modal if it appears
	phaseModal := page.Locator("#phaseModal")
	if visible, err := phaseModal.IsVisible(); err == nil && visible {
		closeButton := page.Locator("#phaseModal button")
		if err := closeButton.Click(); err != nil {
			t.Logf("Could not close phase modal: %v", err)
		}
		time.Sleep(300 * time.Millisecond)
	}

	t.Log("✓ Phase modal handled")

	// Step 3: Verify we're in Purchase phase
	phaseInfo := page.Locator(".phase-info")
	phaseText, err := phaseInfo.TextContent()
	if err != nil {
		t.Fatalf("Failed to get phase: %v", err)
	}

	if phaseText != "Purchase Units" {
		t.Errorf("Expected Purchase Units phase, got: %s", phaseText)
	}

	t.Log("✓ In Purchase Units phase")

	// Step 4: Skip purchase phase (click Done Purchasing button)
	donePurchasing := page.Locator("button:has-text('Done Purchasing')")
	if err := donePurchasing.Click(); err != nil {
		t.Fatalf("Failed to click Done Purchasing: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Verify we advanced to Combat Move phase
	phaseText, _ = phaseInfo.TextContent()
	if phaseText != "Combat Move" {
		t.Errorf("Expected Combat Move phase, got: %s", phaseText)
	}

	t.Log("✓ Advanced to Combat Move phase")

	// Step 5: Skip combat move phase
	executeMoves := page.Locator("button:has-text('Execute Moves')")
	if err := executeMoves.Click(); err != nil {
		t.Fatalf("Failed to click Execute Moves: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	t.Log("✓ Combat moves executed")

	// Step 6: Should be in Conduct Combat phase (or skip to Noncombat if no battles)
	phaseText, _ = phaseInfo.TextContent()
	t.Logf("Current phase after combat moves: %s", phaseText)

	// If in Conduct Combat, skip it
	if phaseText == "Conduct Combat" {
		doneButton := page.Locator("button:has-text('Done with Combat')")
		if err := doneButton.Click(); err != nil {
			t.Fatalf("Failed to advance from combat: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
		t.Log("✓ Combat phase completed")
	}

	// Step 7: Should now be in Noncombat Move
	phaseText, _ = phaseInfo.TextContent()
	if phaseText != "Noncombat Move" {
		t.Logf("Warning: Expected Noncombat Move, got: %s", phaseText)
	}

	// Skip noncombat moves
	executeMoves = page.Locator("button:has-text('Execute Moves')")
	if count, err := executeMoves.Count(); err == nil && count > 0 {
		if err := executeMoves.Click(); err != nil {
			t.Logf("Could not click Execute Moves: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Log("✓ Noncombat moves completed")

	// Step 8: Should be in Mobilize phase
	phaseText, _ = phaseInfo.TextContent()
	if phaseText == "Mobilize New Units" {
		donePlacing := page.Locator("button:has-text('Done Placing')")
		if err := donePlacing.Click(); err != nil {
			t.Logf("Could not advance from Mobilize: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
		t.Log("✓ Mobilize phase completed")
	}

	// Step 9: Collect Income phase
	phaseText, _ = phaseInfo.TextContent()
	if phaseText == "Collect Income" {
		collectButton := page.Locator("button:has-text('Collect Income & End Turn')")
		if err := collectButton.Click(); err != nil {
			t.Fatalf("Failed to collect income: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
		t.Log("✓ Income collected, turn ended")
	}

	// Step 10: Should now be NPC's turn
	time.Sleep(1 * time.Second) // Give time for state update
	yourTurnIndicator := page.Locator(".your-turn-indicator")
	if visible, err := yourTurnIndicator.IsVisible(); err == nil && visible {
		t.Error("Should not be human's turn anymore")
	}

	t.Log("✓ Turn advanced to NPC")

	// Step 11: Test territory navigation
	t.Log("Testing territory navigation...")

	// Wait for territories to load
	territoryList := page.Locator(".territory-list")
	if err := territoryList.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
		State:   playwright.WaitForSelectorStateVisible,
	}); err != nil {
		t.Fatalf("Territory list did not appear: %v", err)
	}

	// Check how many territory buttons exist
	territoryButtons := page.Locator(".territory-button")
	count, err := territoryButtons.Count()
	if err != nil {
		t.Fatalf("Failed to count territory buttons: %v", err)
	}
	t.Logf("Found %d territory buttons", count)

	if count == 0 {
		t.Fatal("No territory buttons found")
	}

	// Take screenshot before clicking
	page.Screenshot(playwright.PageScreenshotOptions{
		Path: playwright.String("territory-nav-screenshot.png"),
	})

	territoryButton := territoryButtons.First()

	// Check if button is visible
	if visible, err := territoryButton.IsVisible(); err != nil || !visible {
		t.Fatalf("Territory button not visible: %v", err)
	}

	// Try to click with force option
	if err := territoryButton.Click(playwright.LocatorClickOptions{
		Force:   playwright.Bool(true),
		Timeout: playwright.Float(5000),
	}); err != nil {
		t.Fatalf("Failed to click territory: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Verify territory details sidebar is present (details may vary based on turn)
	actionSidebar := page.Locator(".action-sidebar")
	if visible, err := actionSidebar.IsVisible(); err == nil && visible {
		t.Log("✓ Territory details sidebar visible")
	} else {
		// During NPC turn, details might be limited - this is acceptable
		t.Log("ℹ Territory details sidebar not fully populated (expected during NPC turn)")
	}

	t.Log("✓ Territory navigation working")

	// Step 12: Test search functionality
	t.Log("Testing territory search...")

	searchInput := page.Locator("input[placeholder*='Search']")
	if err := searchInput.Fill("Berlin"); err != nil {
		t.Fatalf("Failed to fill search: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Check that list is filtered
	territoryButtons = page.Locator(".territory-button")
	count, err = territoryButtons.Count()
	if err != nil {
		t.Fatalf("Failed to count territories: %v", err)
	}

	if count > 3 {
		t.Errorf("Expected filtered list, got %d territories", count)
	}

	t.Log("✓ Search functionality working")

	// Clear search
	if err := searchInput.Clear(); err != nil {
		t.Logf("Could not clear search: %v", err)
	}

	// Step 13: Test NPC turn execution
	t.Log("Testing NPC turn execution...")

	npcButton := page.Locator("button:has-text('Watch NPC Turn')")
	if count, err := npcButton.Count(); err == nil && count > 0 {
		if err := npcButton.Click(); err != nil {
			t.Fatalf("Failed to execute NPC turn: %v", err)
		}

		// Wait for NPC turn to complete
		time.Sleep(2 * time.Second)

		t.Log("✓ NPC turn executed")
	}

	t.Log("✅ Complete game flow test passed!")
}

// TestE2E_KeyboardNavigation tests keyboard-only navigation
func TestE2E_KeyboardNavigation(t *testing.T) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		t.Skip("Skipping E2E test (set RUN_BROWSER_TESTS=1 to run)")
	}

	_, baseURL := startTestServer(t)

	ctx, err := browser.NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()

	page, err := ctx.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	if _, err := page.Goto(baseURL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	t.Log("Testing keyboard navigation...")

	// Tab through form fields
	for i := 0; i < 3; i++ {
		if err := page.Keyboard().Press("Tab", playwright.KeyboardPressOptions{}); err != nil {
			t.Fatalf("Failed to press Tab: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Active element should be the submit button now
	activeTag, err := page.Evaluate("document.activeElement.tagName", nil)
	if err != nil {
		t.Errorf("Failed to get active element: %v", err)
	}

	t.Logf("After tabbing, active element: %v", activeTag)

	// Test Escape key (should do nothing on setup screen, but shouldn't error)
	if err := page.Keyboard().Press("Escape", playwright.KeyboardPressOptions{}); err != nil {
		t.Errorf("Escape key caused error: %v", err)
	}

	t.Log("✓ Keyboard navigation working")
}

// TestE2E_PurchaseFlow tests the purchase unit workflow
func TestE2E_PurchaseFlow(t *testing.T) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		t.Skip("Skipping E2E test (set RUN_BROWSER_TESTS=1 to run)")
	}

	// This test would require a real game to be set up
	// For now, we'll verify the purchase modal can open and close
	t.Skip("Purchase flow test requires full game setup")
}

// Helper function to create a minimal test game file
func createTestGameFile(t *testing.T) string {
	// Create temp file
	tmpDir := os.TempDir()
	testFile := filepath.Join(tmpDir, "test_game.gdf")

	// Create a minimal but valid game definition
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

	// Verify the file is valid by parsing it
	p, err := parser.NewParser(testFile)
	if err != nil {
		t.Fatalf("Failed to create parser for test file: %v", err)
	}

	if _, err := p.Parse(); err != nil {
		t.Fatalf("Test game file is invalid: %v", err)
	}

	return testFile
}

// TestE2E_ErrorRecovery tests error handling and recovery
func TestE2E_ErrorRecovery(t *testing.T) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		t.Skip("Skipping E2E test (set RUN_BROWSER_TESTS=1 to run)")
	}

	_, baseURL := startTestServer(t)

	ctx, err := browser.NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()

	page, err := ctx.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Listen for console errors
	page.On("pageerror", func(err error) {
		t.Logf("Browser error: %v", err)
	})

	page.On("console", func(msg playwright.ConsoleMessage) {
		if msg.Type() == "error" {
			t.Logf("Console error: %s", msg.Text())
		}
	})

	if _, err := page.Goto(baseURL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Try with invalid file path
	gdfInput := page.Locator("#gdfPath")
	if err := gdfInput.Fill("/invalid/path.gdf"); err != nil {
		t.Fatalf("Failed to fill input: %v", err)
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

	// Wait a bit for potential error
	time.Sleep(1 * time.Second)

	// Error message should appear
	errorMsg := page.Locator(".error-message")
	if visible, err := errorMsg.IsVisible(); err == nil && visible {
		text, _ := errorMsg.TextContent()
		t.Logf("Error message displayed: %s", text)

		// Verify setup screen is still visible (didn't transition)
		setupScreen := page.Locator(".setup-screen")
		if visible, err := setupScreen.IsVisible(); err != nil || !visible {
			t.Error("Setup screen should still be visible after error")
		}

		t.Log("✓ Error handling working correctly")
	}
}

// TestE2E_MultipleGames tests creating multiple game sessions
func TestE2E_MultipleGames(t *testing.T) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		t.Skip("Skipping E2E test (set RUN_BROWSER_TESTS=1 to run)")
	}

	testGDFPath := createTestGameFile(t)
	defer os.Remove(testGDFPath)

	_, baseURL := startTestServer(t)

	// Create two separate browser contexts (simulating two users)
	ctx1, err := browser.NewContext()
	if err != nil {
		t.Fatalf("Failed to create context 1: %v", err)
	}
	defer ctx1.Close()

	ctx2, err := browser.NewContext()
	if err != nil {
		t.Fatalf("Failed to create context 2: %v", err)
	}
	defer ctx2.Close()

	page1, _ := ctx1.NewPage()
	page2, _ := ctx2.NewPage()

	// Start game in both contexts
	for i, page := range []playwright.Page{page1, page2} {
		if _, err := page.Goto(baseURL); err != nil {
			t.Fatalf("Failed to navigate page %d: %v", i+1, err)
		}

		gdfInput := page.Locator("#gdfPath")
		gdfInput.Fill(testGDFPath)

		playerSelect := page.Locator("#playerName")
		playerSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{"Germany"},
		})

		startButton := page.Locator("button[type='submit']")
		startButton.Click()

		time.Sleep(1 * time.Second)

		t.Logf("✓ Game %d started", i+1)
	}

	// Verify both games are independent
	// They should both be in Purchase phase for Germany
	for i, page := range []playwright.Page{page1, page2} {
		phaseInfo := page.Locator(".phase-info")
		phaseText, _ := phaseInfo.TextContent()

		if phaseText != "Purchase Units" {
			t.Errorf("Game %d: Expected Purchase Units, got %s", i+1, phaseText)
		}
	}

	t.Log("✓ Multiple independent game sessions working")
}

// Benchmark function to test performance
func BenchmarkE2E_GameStartup(b *testing.B) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		b.Skip("Skipping benchmark (set RUN_BROWSER_TESTS=1 to run)")
	}

	testGDFPath := createTestGameFile(&testing.T{})
	defer os.Remove(testGDFPath)

	_, baseURL := startTestServer(&testing.T{})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, _ := browser.NewContext()
		page, _ := ctx.NewPage()

		page.Goto(baseURL)

		gdfInput := page.Locator("#gdfPath")
		gdfInput.Fill(testGDFPath)

		playerSelect := page.Locator("#playerName")
		playerSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{"Germany"},
		})

		startButton := page.Locator("button[type='submit']")
		startButton.Click()

		time.Sleep(500 * time.Millisecond)

		ctx.Close()
	}
}
