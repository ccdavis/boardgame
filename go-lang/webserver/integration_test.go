package webserver

import (
	"boardgame/game"
	"boardgame/models"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Helper function to create a simple test game
func createIntegrationTestGame() *models.Game {
	g := models.NewGame()

	// Create players
	germany := g.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"
	germany.IPCs = 40
	germany.Capital = "Berlin"

	ussr := g.GetOrCreatePlayer("USSR")
	ussr.Side = "Allies"
	ussr.IPCs = 30
	ussr.Capital = "Moscow"

	g.PlayerOrder = []string{"Germany", "USSR"}

	// Create territories
	berlin := &models.Territory{
		Name:          "Berlin",
		Owner:         germany,
		Terrain:       models.Land,
		Production:    10,
		IsVictoryCity: true,
		Pieces:        []int{},
		ConnectedTo:   []*models.Territory{},
	}

	poland := &models.Territory{
		Name:          "Poland",
		Owner:         germany,
		Terrain:       models.Land,
		Production:    2,
		IsVictoryCity: false,
		Pieces:        []int{},
		ConnectedTo:   []*models.Territory{},
	}

	moscow := &models.Territory{
		Name:          "Moscow",
		Owner:         ussr,
		Terrain:       models.Land,
		Production:    8,
		IsVictoryCity: true,
		Pieces:        []int{},
		ConnectedTo:   []*models.Territory{},
	}

	// Connect territories
	berlin.ConnectedTo = []*models.Territory{poland}
	poland.ConnectedTo = []*models.Territory{berlin, moscow}
	moscow.ConnectedTo = []*models.Territory{poland}

	g.Board["Berlin"] = berlin
	g.Board["Poland"] = poland
	g.Board["Moscow"] = moscow

	germany.Territories = []*models.Territory{berlin, poland}
	ussr.Territories = []*models.Territory{moscow}

	// Add some unit templates
	infantry := &models.Piece{
		Name:     "infantry",
		Attack:   1,
		Defend:   2,
		Movement: 1,
		Cost:     3,
		Terrain:  models.Land,
	}

	g.GlobalPieceTemplates["infantry"] = infantry

	return g
}

func TestServerIntegration_GetGameState(t *testing.T) {
	// Create test game
	g := createIntegrationTestGame()
	controller := game.NewGameController(g)
	controller.StartGame()

	// Create session manager and session
	sm := NewSessionManager()
	session, err := sm.CreateSession(controller, "Germany")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create server
	server := &Server{
		sessionManager: sm,
		port:           8080,
	}

	// Create request
	req := httptest.NewRequest("GET", "/api/game/"+session.ID, nil)
	w := httptest.NewRecorder()

	// Handle request
	server.handleGameRoutes(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var state GameStateDTO
	if err := json.NewDecoder(w.Body).Decode(&state); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if state.Turn != 1 {
		t.Errorf("Expected turn 1, got %d", state.Turn)
	}

	if state.HumanPlayer != "Germany" {
		t.Errorf("Expected human player Germany, got %s", state.HumanPlayer)
	}

	if !state.IsHumanTurn {
		t.Error("Expected it to be human's turn")
	}
}

func TestServerIntegration_PurchaseUnit(t *testing.T) {
	// Create test game
	g := createIntegrationTestGame()
	controller := game.NewGameController(g)
	controller.StartGame()

	// Create session
	sm := NewSessionManager()
	session, err := sm.CreateSession(controller, "Germany")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create server
	server := &Server{
		sessionManager: sm,
		port:           8080,
	}

	// Create purchase request
	reqBody := map[string]interface{}{
		"unitType": "infantry",
		"quantity": 5,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/game/"+session.ID+"/action/purchase", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Handle request
	server.handleGameRoutes(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify IPCs were deducted
	player := g.Players["Germany"]
	expectedIPCs := 40 - (5 * 3) // Started with 40, bought 5 infantry at 3 each
	if player.IPCs != expectedIPCs {
		t.Errorf("Expected %d IPCs, got %d", expectedIPCs, player.IPCs)
	}

	// Verify units were added to purchased list
	purchased := g.PurchasedUnits["Germany"]
	if len(purchased) != 5 {
		t.Errorf("Expected 5 purchased units, got %d", len(purchased))
	}
}

func TestServerIntegration_GetTerritories(t *testing.T) {
	// Create test game
	g := createIntegrationTestGame()
	controller := game.NewGameController(g)

	// Create session
	sm := NewSessionManager()
	session, err := sm.CreateSession(controller, "Germany")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create server
	server := &Server{
		sessionManager: sm,
		port:           8080,
	}

	// Create request
	req := httptest.NewRequest("GET", "/api/game/"+session.ID+"/territories", nil)
	w := httptest.NewRecorder()

	// Handle request
	server.handleGameRoutes(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response map[string][]TerritoryDTO
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	territories := response["territories"]
	if len(territories) != 3 {
		t.Errorf("Expected 3 territories, got %d", len(territories))
	}

	// Check that Berlin is in the list
	found := false
	for _, terr := range territories {
		if terr.Name == "Berlin" {
			found = true
			if !terr.IsVictoryCity {
				t.Error("Berlin should be a victory city")
			}
			if terr.Owner != "Germany" {
				t.Errorf("Berlin should be owned by Germany, got %s", terr.Owner)
			}
		}
	}

	if !found {
		t.Error("Berlin not found in territories list")
	}
}

func TestServerIntegration_AdvancePhase(t *testing.T) {
	// Create test game
	g := createIntegrationTestGame()
	controller := game.NewGameController(g)
	controller.StartGame()

	// Create session
	sm := NewSessionManager()
	session, err := sm.CreateSession(controller, "Germany")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create server
	server := &Server{
		sessionManager: sm,
		port:           8080,
	}

	// Verify starting phase
	if g.CurrentPhase != models.PurchasePhase {
		t.Fatalf("Expected Purchase phase, got %v", g.CurrentPhase)
	}

	// Advance phase
	req := httptest.NewRequest("POST", "/api/game/"+session.ID+"/action/advance-phase", nil)
	w := httptest.NewRecorder()

	server.handleGameRoutes(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify phase changed
	if g.CurrentPhase != models.CombatMovePhase {
		t.Errorf("Expected Combat Move phase, got %v", g.CurrentPhase)
	}
}
