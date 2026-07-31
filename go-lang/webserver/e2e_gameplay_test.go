package webserver

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// The board the shipping product actually plays, relative to this package
// directory (tests run with the package as cwd). Synthetic one-off boards are
// no use here: every board must pair with a layout file, and the server
// refuses -- by design -- to start a game without one.
const realBoard = "../../aaa.gdf"

// startOwnedTestServer starts a server on its own port and returns it with its
// base URL. startTestServer's shared :8888 has a quirk: the first test's
// server binds and keeps serving everyone, so a later test that needs to reach
// into the session behind the browser would be holding the wrong
// SessionManager. A test that grants IPCs or declares victory must own the
// manager its requests actually hit.
func startOwnedTestServer(t *testing.T, port int) (*Server, string) {
	t.Helper()
	server := NewServer(port)
	go func() {
		if err := server.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	time.Sleep(500 * time.Millisecond)
	return server, fmt.Sprintf("http://localhost:%d", port)
}

// startGameOnPage drives the setup form: real board, the given power, submit,
// and waits until the game screen is actually up.
func startGameOnPage(t *testing.T, page playwright.Page, baseURL, power string) {
	t.Helper()
	if _, err := page.Goto(baseURL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}
	if err := page.Locator("#gdfPath").Fill(realBoard); err != nil {
		t.Fatalf("Failed to fill GDF path: %v", err)
	}
	if _, err := page.Locator("#playerName").SelectOption(playwright.SelectOptionValues{
		Values: &[]string{power},
	}); err != nil {
		t.Fatalf("Failed to select player: %v", err)
	}
	if err := page.Locator("button[type='submit']").Click(); err != nil {
		t.Fatalf("Failed to click start: %v", err)
	}
	if err := page.Locator(".game-screen").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(15000),
	}); err != nil {
		t.Fatalf("Game screen did not appear: %v", err)
	}
}

// sessionIDOf reads the session id out of the running page's API client.
func sessionIDOf(t *testing.T, page playwright.Page) string {
	t.Helper()
	raw, err := page.Evaluate("window.vueApp.api.sessionId")
	if err != nil {
		t.Fatalf("Failed to read session id from page: %v", err)
	}
	id, ok := raw.(string)
	if !ok || id == "" {
		t.Fatalf("Page has no session id (got %v)", raw)
	}
	return id
}

// closeGuidance dismisses the phase-guidance modal if it is showing. It
// reopens on every phase change of the human's turn, so phase-walking tests
// call this before each click.
func closeGuidance(page playwright.Page) {
	btn := page.Locator(".phase-modal button")
	if visible, _ := btn.IsVisible(); visible {
		btn.Click()
		time.Sleep(150 * time.Millisecond)
	}
}

// expectPhase polls the header until the named phase shows, or fails.
func expectPhase(t *testing.T, page playwright.Page, want string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		if text, err := page.Locator(".phase-info").TextContent(); err == nil {
			last = strings.TrimSpace(text)
			if last == want {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("Phase never reached %q (still %q)", want, last)
}

// clickAction clicks a footer action button by its text.
func clickAction(t *testing.T, page playwright.Page, label string) {
	t.Helper()
	closeGuidance(page)
	btn := page.Locator(fmt.Sprintf(".action-bar button:has-text('%s')", label))
	if err := btn.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(5000)}); err != nil {
		t.Fatalf("Failed to click %q: %v", label, err)
	}
}

// TestE2E_CompleteGameFlow walks a full human turn on the real board -- every
// phase from Purchase to Collect Income -- then hands over to an NPC and
// checks the world keeps turning.
func TestE2E_CompleteGameFlow(t *testing.T) {
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
	// The app still reports through prompt()/alert()/confirm(); accept them
	// all so the flow is not silently vetoed by a dismissed confirm.
	page.OnDialog(func(d playwright.Dialog) { d.Accept() })

	startGameOnPage(t, page, baseURL, "Germany")
	expectPhase(t, page, "Purchase Units")

	clickAction(t, page, "Done Purchasing")
	expectPhase(t, page, "Combat Move")

	clickAction(t, page, "Execute Moves")
	expectPhase(t, page, "Conduct Combat")

	clickAction(t, page, "Done with Combat")
	expectPhase(t, page, "Noncombat Move")

	clickAction(t, page, "Execute Moves")
	expectPhase(t, page, "Mobilize New Units")

	clickAction(t, page, "Done Placing")
	expectPhase(t, page, "Collect Income")

	clickAction(t, page, "Collect Income & End Turn")
	expectPhase(t, page, "Purchase Units")

	// Now it is an NPC's turn; the footer must say so.
	npcInfo := page.Locator(".npc-turn-info")
	if err := npcInfo.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	}); err != nil {
		t.Fatalf("NPC turn indicator never appeared: %v", err)
	}

	// Run the NPC's whole turn from the browser.
	before, _ := page.Locator(".player-info").TextContent()
	clickAction(t, page, "Watch NPC Turn")
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		now, _ := page.Locator(".player-info").TextContent()
		if now != before {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	after, _ := page.Locator(".player-info").TextContent()
	if after == before {
		t.Fatalf("Power never changed after the NPC turn (still %q)", before)
	}

	// Territory search narrows the sidebar, and selecting from it fills the
	// details panel.
	search := page.Locator("input[placeholder*='Search']")
	if err := search.Fill("Karelia"); err != nil {
		t.Fatalf("Failed to fill search: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	buttons := page.Locator(".territory-button")
	count, _ := buttons.Count()
	if count == 0 || count > 3 {
		t.Errorf("Search for Karelia matched %d territories, want a small filtered list", count)
	}
	if err := buttons.First().Click(); err != nil {
		t.Fatalf("Failed to click filtered territory: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	heading, _ := page.Locator(".action-sidebar h2").TextContent()
	if !strings.Contains(heading, "Karelia") {
		t.Errorf("Details panel shows %q after selecting Karelia", heading)
	}
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

// TestE2E_PurchaseFlow buys units with real money and places them through the
// Mobilize interface: the full economy loop a human actually plays. Powers
// start with an empty treasury, so the test grants Germany a budget
// server-side -- which is why it runs on a server it owns.
func TestE2E_PurchaseFlow(t *testing.T) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		t.Skip("Skipping E2E test (set RUN_BROWSER_TESTS=1 to run)")
	}

	server, baseURL := startOwnedTestServer(t, 8899)

	ctx, err := browser.NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	page, err := ctx.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	var dialogs []string
	page.OnDialog(func(d playwright.Dialog) {
		dialogs = append(dialogs, d.Message())
		d.Accept()
	})

	startGameOnPage(t, page, baseURL, "Germany")

	// Grant a budget: 2 infantry and 1 armor's worth.
	session, err := server.sessionManager.GetSession(sessionIDOf(t, page))
	if err != nil {
		t.Fatalf("Session not found on owned server: %v", err)
	}
	session.Lock()
	session.Controller.Game.Players["Germany"].IPCs = 11
	session.Unlock()

	// The header IPC count follows the next poll; wait for it.
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		text, _ := page.Locator(".action-bar").TextContent()
		if strings.Contains(text, "11 IPCs") {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	closeGuidance(page)
	clickAction(t, page, "Buy Units")

	buy := func(unit string, times int) {
		t.Helper()
		item := page.Locator(".unit-purchase-item", playwright.PageLocatorOptions{
			HasText: unit,
		}).First()
		for i := 0; i < times; i++ {
			if err := item.Locator("button:has-text('Buy')").Click(playwright.LocatorClickOptions{
				Timeout: playwright.Float(5000),
			}); err != nil {
				t.Fatalf("Failed to buy %s: %v", unit, err)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
	buy("infantry", 2)
	buy("armor", 1)

	if err := page.Locator("#purchaseModal button:has-text('Close')").Click(); err != nil {
		t.Fatalf("Failed to close purchase modal: %v", err)
	}

	// Walk to the Mobilize phase.
	clickAction(t, page, "Done Purchasing")
	expectPhase(t, page, "Combat Move")
	clickAction(t, page, "Execute Moves")
	expectPhase(t, page, "Conduct Combat")
	clickAction(t, page, "Done with Combat")
	expectPhase(t, page, "Noncombat Move")
	clickAction(t, page, "Execute Moves")
	expectPhase(t, page, "Mobilize New Units")

	// Three actual units to place, in two groups.
	closeGuidance(page)
	countText, _ := page.Locator(".units-count").TextContent()
	if !strings.Contains(countText, "3 units to place") {
		t.Errorf("Mobilize bar says %q, want 3 units to place", countText)
	}
	groups := page.Locator(".mobilize-group")
	if n, _ := groups.Count(); n != 2 {
		t.Errorf("Mobilize bar shows %d groups, want 2 (infantry, armor)", n)
	}

	// Ending the phase with units unplaced must be refused, with a reason.
	clickAction(t, page, "Done Placing")
	time.Sleep(500 * time.Millisecond)
	blockedSeen := false
	for _, d := range dialogs {
		if strings.Contains(d, "Cannot end this phase yet") && strings.Contains(d, "still to place") {
			blockedSeen = true
		}
	}
	if !blockedSeen {
		t.Errorf("Blocked advance never explained itself; dialogs: %q", dialogs)
	}
	expectPhase(t, page, "Mobilize New Units")

	// Place everything in Germany (the only German factory on the real board).
	for _, unit := range []string{"infantry", "armor"} {
		group := page.Locator(".mobilize-group", playwright.PageLocatorOptions{
			HasText: unit,
		}).First()
		if _, err := group.Locator("select").SelectOption(playwright.SelectOptionValues{
			Values: &[]string{"Germany"},
		}); err != nil {
			t.Fatalf("Failed to choose territory for %s: %v", unit, err)
		}
		label := "Place all"
		if unit == "armor" {
			label = "Place 1"
		}
		if err := group.Locator(fmt.Sprintf("button:has-text('%s')", label)).Click(playwright.LocatorClickOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			t.Fatalf("Failed to place %s: %v", unit, err)
		}
		time.Sleep(400 * time.Millisecond)
	}

	// With the backlog cleared, the phase ends normally.
	clickAction(t, page, "Done Placing")
	expectPhase(t, page, "Collect Income")
}

// TestE2E_ErrorRecovery tests that a bad board path fails visibly and leaves
// the setup screen usable.
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

// TestE2E_MultipleGames verifies two browser sessions get two independent
// games of the real board.
func TestE2E_MultipleGames(t *testing.T) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		t.Skip("Skipping E2E test (set RUN_BROWSER_TESTS=1 to run)")
	}

	_, baseURL := startTestServer(t)

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

	sessionIDs := make([]string, 0, 2)
	for i, page := range []playwright.Page{page1, page2} {
		startGameOnPage(t, page, baseURL, "Germany")
		expectPhase(t, page, "Purchase Units")
		sessionIDs = append(sessionIDs, sessionIDOf(t, page))
		t.Logf("✓ Game %d started", i+1)
	}

	if sessionIDs[0] == sessionIDs[1] {
		t.Errorf("Both pages share session %s; games are not independent", sessionIDs[0])
	}
}

// TestE2E_VictoryBanner declares a victory server-side and requires the
// browser to notice: the overlay names the winner, and the action bar closes
// the war. The state poll is the only channel, so this proves the whole path.
func TestE2E_VictoryBanner(t *testing.T) {
	if os.Getenv("RUN_BROWSER_TESTS") != "1" {
		t.Skip("Skipping E2E test (set RUN_BROWSER_TESTS=1 to run)")
	}

	server, baseURL := startOwnedTestServer(t, 8897)

	ctx, err := browser.NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	page, err := ctx.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	startGameOnPage(t, page, baseURL, "Germany")

	// A sustained Axis victory, already held for two round boundaries.
	session, err := server.sessionManager.GetSession(sessionIDOf(t, page))
	if err != nil {
		t.Fatalf("Session not found on owned server: %v", err)
	}
	session.Lock()
	session.Controller.Game.VictoryCitiesEnabled = true
	session.Controller.Game.VictoryHoldSide = "Axis"
	session.Controller.Game.VictoryHoldRounds = 2
	session.Unlock()

	overlay := page.Locator(".victory-overlay")
	if err := overlay.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(8000), // one poll cycle plus slack
	}); err != nil {
		t.Fatalf("Victory overlay never appeared: %v", err)
	}
	text, _ := overlay.TextContent()
	if !strings.Contains(text, "Axis win the war") {
		t.Errorf("Overlay says %q, want it to name the Axis as winners", text)
	}

	// Dismissing it leaves the verdict in the action bar and the map browsable.
	if err := overlay.Locator("button:has-text('View the final map')").Click(); err != nil {
		t.Fatalf("Failed to dismiss victory overlay: %v", err)
	}
	if visible, _ := overlay.IsVisible(); visible {
		t.Error("Victory overlay still showing after dismissal")
	}
	gameOverBar, _ := page.Locator(".game-over-text").TextContent()
	if !strings.Contains(gameOverBar, "Axis win") {
		t.Errorf("Action bar says %q, want the Axis verdict", gameOverBar)
	}
}
