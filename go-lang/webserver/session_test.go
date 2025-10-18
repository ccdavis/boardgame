package webserver

import (
	"boardgame/game"
	"boardgame/models"
	"testing"
	"time"
)

func createTestGame() *models.Game {
	g := models.NewGame()

	// Create players
	germany := g.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"
	germany.IPCs = 40

	ussr := g.GetOrCreatePlayer("USSR")
	ussr.Side = "Allies"
	ussr.IPCs = 30

	g.PlayerOrder = []string{"Germany", "USSR"}

	// Create a simple board
	berlin := &models.Territory{
		Name:       "Berlin",
		Owner:      germany,
		Terrain:    models.Land,
		Production: 10,
		Pieces:     []int{},
		ConnectedTo: []*models.Territory{},
	}

	g.Board["Berlin"] = berlin
	germany.Territories = append(germany.Territories, berlin)

	return g
}

func TestSessionManager_CreateSession(t *testing.T) {
	sm := NewSessionManager()
	g := createTestGame()
	controller := game.NewGameController(g)

	session, err := sm.CreateSession(controller, "Germany")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if session.ID == "" {
		t.Error("Session ID is empty")
	}

	if session.HumanPlayer != "Germany" {
		t.Errorf("Expected human player Germany, got %s", session.HumanPlayer)
	}

	if session.Controller != controller {
		t.Error("Controller not set correctly")
	}
}

func TestSessionManager_GetSession(t *testing.T) {
	sm := NewSessionManager()
	g := createTestGame()
	controller := game.NewGameController(g)

	session, err := sm.CreateSession(controller, "Germany")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Retrieve the session
	retrieved, err := sm.GetSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, retrieved.ID)
	}

	// Try to get non-existent session
	_, err = sm.GetSession("non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent session")
	}
}

func TestSessionManager_DeleteSession(t *testing.T) {
	sm := NewSessionManager()
	g := createTestGame()
	controller := game.NewGameController(g)

	session, err := sm.CreateSession(controller, "Germany")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Delete the session
	err = sm.DeleteSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Try to get deleted session
	_, err = sm.GetSession(session.ID)
	if err == nil {
		t.Error("Expected error for deleted session")
	}
}

func TestGameSession_IsExpired(t *testing.T) {
	g := createTestGame()
	controller := game.NewGameController(g)
	session := NewGameSession(controller, "Germany")

	// New session should not be expired
	if session.IsExpired() {
		t.Error("New session should not be expired")
	}

	// Manually set last accessed to 25 hours ago
	session.LastAccessedAt = time.Now().Add(-25 * time.Hour)

	if !session.IsExpired() {
		t.Error("Session should be expired after 25 hours")
	}
}

func TestGameSession_Touch(t *testing.T) {
	g := createTestGame()
	controller := game.NewGameController(g)
	session := NewGameSession(controller, "Germany")

	oldTime := session.LastAccessedAt

	// Wait a bit and touch
	time.Sleep(10 * time.Millisecond)
	session.Touch()

	if !session.LastAccessedAt.After(oldTime) {
		t.Error("Touch should update LastAccessedAt")
	}
}

func TestGameSession_IsHumanTurn(t *testing.T) {
	g := createTestGame()
	controller := game.NewGameController(g)
	controller.StartGame()

	session := NewGameSession(controller, "Germany")

	// First turn should be Germany
	if !session.IsHumanTurn() {
		t.Error("Should be human turn for Germany")
	}

	// Advance to next player
	for g.CurrentPhase != models.CollectIncomePhase {
		controller.AdvancePhase()
	}
	controller.AdvancePhase() // This should advance to next player

	// Now it's USSR's turn
	if session.IsHumanTurn() {
		t.Error("Should not be human turn for USSR")
	}
}
