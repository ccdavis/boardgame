package ui

import (
	"boardgame/game"
	"boardgame/models"
	"testing"
)

// TestModeToggle tests switching between tutorial and expert modes
func TestModeToggle(t *testing.T) {
	g := createMinimalGame("Germany")
	controller := game.NewGameController(g)
	controller.StartGame()

	// This test exercises the mode toggle itself, so it needs a terminal in the
	// real default mode rather than the ExpertMode one the other tests use.
	terminal := NewTerminal(controller)

	// Should start in tutorial mode
	if terminal.Mode != TutorialMode {
		t.Error("Terminal should start in tutorial mode")
	}

	// Toggle to expert
	terminal.toggleMode()
	if terminal.Mode != ExpertMode {
		t.Error("Should be in expert mode after toggle")
	}

	// Toggle back
	terminal.toggleMode()
	if terminal.Mode != TutorialMode {
		t.Error("Should be back in tutorial mode")
	}
}

// TestMovementCommands tests the movement command flow
func TestMovementCommands(t *testing.T) {
	g := createGameWithConnectedTerritories()
	controller := game.NewGameController(g)
	controller.StartGame()

	terminal := newTestTerminal(controller)

	// Advance to combat move phase
	controller.Game.CurrentPhase = models.CombatMovePhase

	// Plan a move
	err := terminal.ProcessCommand("move 1 Berlin Poland")
	if err != nil {
		t.Fatalf("Failed to plan move: %v", err)
	}

	// Check move was added
	moves := controller.GetPlannedMoves()
	if len(moves) != 1 {
		t.Errorf("Expected 1 planned move, got %d", len(moves))
	}

	// Show moves
	err = terminal.ProcessCommand("show")
	if err != nil {
		t.Errorf("Show command failed: %v", err)
	}

	// Cancel move
	err = terminal.ProcessCommand("cancel 1")
	if err != nil {
		t.Fatalf("Failed to cancel move: %v", err)
	}

	// Check move was removed
	moves = controller.GetPlannedMoves()
	if len(moves) != 0 {
		t.Errorf("Expected 0 planned moves after cancel, got %d", len(moves))
	}
}

// TestCombatCommands tests the combat command flow
func TestCombatCommands(t *testing.T) {
	g := createGameWithBattle()
	controller := game.NewGameController(g)
	controller.StartGame()

	terminal := newTestTerminal(controller)

	// Set up phase
	controller.Game.CurrentPhase = models.ConductCombatPhase

	// Create a battle
	attacker := g.Players["Germany"]
	defender := g.Players["USSR"]
	battle := game.NewBattle("Poland", game.LandBattle, attacker.Name, defender.Name)

	infantry := g.GlobalPieceTemplates["infantry"]
	battle.Attackers = []*models.Piece{infantry, infantry}
	battle.Defenders = []*models.Piece{infantry}
	controller.PendingBattles["Poland"] = battle

	// List battles
	err := terminal.ProcessCommand("battles")
	if err != nil {
		t.Errorf("Battles command failed: %v", err)
	}

	// View battle
	err = terminal.ProcessCommand("view Poland")
	if err != nil {
		t.Errorf("View command failed: %v", err)
	}

	// Resolve battle
	err = terminal.ProcessCommand("resolve Poland")
	if err != nil {
		t.Errorf("Resolve command failed: %v", err)
	}

	// Battle should be removed after resolution
	if _, exists := controller.PendingBattles["Poland"]; exists {
		t.Error("Battle should be removed after resolution")
	}
}

// TestAutoCombat tests auto-resolving multiple battles
func TestAutoCombat(t *testing.T) {
	g := createGameWithBattle()
	controller := game.NewGameController(g)
	controller.StartGame()

	terminal := newTestTerminal(controller)
	controller.Game.CurrentPhase = models.ConductCombatPhase

	// Create multiple battles
	attacker := g.Players["Germany"]
	infantry := g.GlobalPieceTemplates["infantry"]

	battle1 := game.NewBattle("Poland", game.LandBattle, attacker.Name, "USSR")
	battle1.Attackers = []*models.Piece{infantry, infantry}
	battle1.Defenders = []*models.Piece{infantry}
	controller.PendingBattles["Poland"] = battle1

	battle2 := game.NewBattle("France", game.LandBattle, attacker.Name, "UK")
	battle2.Attackers = []*models.Piece{infantry, infantry, infantry}
	battle2.Defenders = []*models.Piece{infantry}
	controller.PendingBattles["France"] = battle2

	// Auto-resolve all
	err := terminal.ProcessCommand("auto")
	if err != nil {
		t.Errorf("Auto command failed: %v", err)
	}

	// All battles should be resolved
	if len(controller.PendingBattles) != 0 {
		t.Errorf("Expected all battles resolved, got %d remaining", len(controller.PendingBattles))
	}
}

// TestCompleteGameFlow tests a full turn through all phases
func TestCompleteGameFlow(t *testing.T) {
	g := createGameWithConnectedTerritories()
	controller := game.NewGameController(g)
	controller.StartGame()

	terminal := newTestTerminal(controller)
	player := g.Players["Germany"]

	// Purchase phase - buy units
	player.IPCs = 20
	err := terminal.ProcessCommand("buy infantry 3")
	if err != nil {
		t.Fatalf("Buy command failed: %v", err)
	}

	if len(g.PurchasedUnits["Germany"]) != 3 {
		t.Errorf("Expected 3 purchased units, got %d", len(g.PurchasedUnits["Germany"]))
	}

	// Advance to combat move
	err = terminal.advancePhase()
	if err != nil {
		t.Fatalf("Failed to advance phase: %v", err)
	}

	if controller.Game.CurrentPhase != models.CombatMovePhase {
		t.Errorf("Expected CombatMovePhase, got %v", controller.Game.CurrentPhase)
	}

	// Plan a move
	err = terminal.ProcessCommand("move 1 Berlin Poland")
	if err != nil {
		t.Fatalf("Move command failed: %v", err)
	}

	// Advance to combat (executes moves)
	err = terminal.advancePhase()
	if err != nil {
		t.Fatalf("Failed to advance from combat move: %v", err)
	}

	if controller.Game.CurrentPhase != models.ConductCombatPhase {
		t.Errorf("Expected ConductCombatPhase, got %v", controller.Game.CurrentPhase)
	}

	// Auto-resolve battles (if any)
	if len(controller.PendingBattles) > 0 {
		terminal.ProcessCommand("auto")
	}

	// Advance through remaining phases
	terminal.advancePhase() // -> Noncombat
	terminal.advancePhase() // -> Mobilize

	if controller.Game.CurrentPhase != models.MobilizePhase {
		t.Errorf("Expected MobilizePhase, got %v", controller.Game.CurrentPhase)
	}

	// Place units
	err = terminal.ProcessCommand("place infantry Berlin 3")
	if err != nil {
		t.Fatalf("Place command failed: %v", err)
	}

	terminal.advancePhase() // -> Collect Income
	terminal.advancePhase() // -> Next player's turn

	if controller.Game.CurrentPhase != models.PurchasePhase {
		t.Errorf("Expected back to PurchasePhase, got %v", controller.Game.CurrentPhase)
	}
}

// Helper functions

func createMinimalGame(playerName string) *models.Game {
	g := &models.Game{
		Players:              make(map[string]*models.Player),
		PlayerOrder:          []string{playerName},
		Board:                make(map[string]*models.Territory),
		Pieces:               make(map[int]*models.Piece),
		GlobalPieceTemplates: make(map[string]*models.Piece),
		PurchasedUnits:       make(map[string][]*models.PendingUnit),
	}

	player := &models.Player{
		Name:           playerName,
		IPCs:           50,
		NPC:            false,
		Territories:    []*models.Territory{},
		PieceTemplates: make(map[string]*models.Piece),
	}
	g.Players[playerName] = player

	territory := &models.Territory{
		Name:       "Berlin",
		Terrain:    models.Land,
		Production: 10,
		Owner:      player,
		Pieces:     []int{},
	}
	g.Board["Berlin"] = territory
	player.Territories = append(player.Territories, territory)

	// Berlin builds units, so it needs an industrial complex.
	g.AddPieceTemplate("factory", models.Land, 0, 0, 0, 32)
	if err := g.PlacePieces("Berlin", "factory", 1); err != nil {
		panic(err)
	}

	return g
}

func createGameWithConnectedTerritories() *models.Game {
	g := createMinimalGame("Germany")
	player := g.Players["Germany"]

	// Add connected territory
	poland := &models.Territory{
		Name:       "Poland",
		Terrain:    models.Land,
		Production: 2,
		Owner:      player,
		Pieces:     []int{},
	}
	g.Board["Poland"] = poland

	berlin := g.Board["Berlin"]
	berlin.ConnectedTo = []*models.Territory{poland}
	poland.ConnectedTo = []*models.Territory{berlin}

	// Add a piece
	infantry := &models.Piece{
		Name:     "infantry",
		Terrain:  models.Land,
		Movement: 1,
		Attack:   1,
		Defend:   2,
		Cost:     3,
	}
	g.GlobalPieceTemplates["infantry"] = infantry
	g.Pieces[1] = infantry
	berlin.Pieces = append(berlin.Pieces, 1)

	return g
}

func createGameWithBattle() *models.Game {
	g := &models.Game{
		Players:              make(map[string]*models.Player),
		PlayerOrder:          []string{"Germany", "USSR", "UK"},
		Board:                make(map[string]*models.Territory),
		Pieces:               make(map[int]*models.Piece),
		GlobalPieceTemplates: make(map[string]*models.Piece),
		PurchasedUnits:       make(map[string][]*models.PendingUnit),
	}

	germany := &models.Player{
		Name:           "Germany",
		IPCs:           50,
		NPC:            false,
		Territories:    []*models.Territory{},
		PieceTemplates: make(map[string]*models.Piece),
	}
	ussr := &models.Player{
		Name:           "USSR",
		IPCs:           30,
		NPC:            false,
		Territories:    []*models.Territory{},
		PieceTemplates: make(map[string]*models.Piece),
	}
	uk := &models.Player{
		Name:           "UK",
		IPCs:           40,
		NPC:            false,
		Territories:    []*models.Territory{},
		PieceTemplates: make(map[string]*models.Piece),
	}

	g.Players["Germany"] = germany
	g.Players["USSR"] = ussr
	g.Players["UK"] = uk

	// Add territories
	berlin := &models.Territory{
		Name:       "Berlin",
		Terrain:    models.Land,
		Production: 10,
		Owner:      germany,
		Pieces:     []int{},
	}
	poland := &models.Territory{
		Name:       "Poland",
		Terrain:    models.Land,
		Production: 2,
		Owner:      ussr,
		Pieces:     []int{},
	}
	france := &models.Territory{
		Name:       "France",
		Terrain:    models.Land,
		Production: 6,
		Owner:      uk,
		Pieces:     []int{},
	}

	g.Board["Berlin"] = berlin
	g.Board["Poland"] = poland
	g.Board["France"] = france

	germany.Territories = []*models.Territory{berlin}
	ussr.Territories = []*models.Territory{poland}
	uk.Territories = []*models.Territory{france}

	// Add piece template
	infantry := &models.Piece{
		Name:     "infantry",
		Terrain:  models.Land,
		Movement: 1,
		Attack:   1,
		Defend:   2,
		Cost:     3,
	}
	g.GlobalPieceTemplates["infantry"] = infantry

	return g
}
