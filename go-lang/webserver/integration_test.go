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

// The state response must carry the victory verdict: the browser only ever
// learns anything through this poll, so a game that ends silently on the
// server never ends on screen.
func TestServerIntegration_StateReportsVictory(t *testing.T) {
	g := createIntegrationTestGame()
	controller := game.NewGameController(g)
	controller.StartGame()

	// A sustained victory already on the books: Axis held their threshold
	// across two consecutive round boundaries.
	g.VictoryCitiesEnabled = true
	g.VictoryHoldSide = "Axis"
	g.VictoryHoldRounds = 2

	sm := NewSessionManager()
	session, err := sm.CreateSession(controller, "Germany")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	server := &Server{sessionManager: sm, port: 8080}

	req := httptest.NewRequest("GET", "/api/game/"+session.ID, nil)
	w := httptest.NewRecorder()
	server.handleGameRoutes(w, req)

	var state GameStateDTO
	if err := json.NewDecoder(w.Body).Decode(&state); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !state.GameOver {
		t.Error("state does not report the game as over")
	}
	if state.Winner != "Axis" {
		t.Errorf("state names %q as winner, want Axis", state.Winner)
	}
}

// During the Mobilize phase, available-actions must say not only what is
// waiting to be placed but where it may legally go -- that list is what the
// placement interface offers the player.
func TestServerIntegration_MobilizeActionsIncludeTargets(t *testing.T) {
	g := createIntegrationTestGame()

	// Berlin gets an industrial complex; Poland stays bare. Only Berlin may
	// receive the pending infantry.
	factory := &models.Piece{Name: "factory", Cost: 32, Terrain: models.Land}
	g.GlobalPieceTemplates["factory"] = factory
	if err := g.PlacePieces("Berlin", "factory", 1); err != nil {
		t.Fatalf("placing factory: %v", err)
	}

	controller := game.NewGameController(g)
	controller.StartGame()
	g.CurrentPhase = models.MobilizePhase
	g.PurchasedUnits["Germany"] = []*models.PendingUnit{
		{Type: "infantry", Cost: 3},
		{Type: "infantry", Cost: 3},
	}

	sm := NewSessionManager()
	session, err := sm.CreateSession(controller, "Germany")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	server := &Server{sessionManager: sm, port: 8080}

	req := httptest.NewRequest("GET", "/api/game/"+session.ID+"/available-actions", nil)
	w := httptest.NewRecorder()
	server.handleGameRoutes(w, req)

	var response struct {
		Actions struct {
			PurchasedUnits []PurchasedUnitDTO `json:"purchasedUnits"`
		} `json:"actions"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	groups := response.Actions.PurchasedUnits
	if len(groups) != 1 {
		t.Fatalf("got %d pending groups, want 1: %+v", len(groups), groups)
	}
	if groups[0].Type != "infantry" || groups[0].Quantity != 2 {
		t.Errorf("pending group = %+v, want 2 infantry", groups[0])
	}
	if len(groups[0].Targets) != 1 || groups[0].Targets[0] != "Berlin" {
		t.Errorf("targets = %v, want [Berlin]", groups[0].Targets)
	}
}
