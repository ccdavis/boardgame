package ui

import (
	"boardgame/game"
	"boardgame/models"
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// TestTerminalBasicCommands tests that basic commands work
func TestTerminalBasicCommands(t *testing.T) {
	// Create a minimal game
	g := &models.Game{
		Players:              make(map[string]*models.Player),
		PlayerOrder:          []string{"USSR"},
		Board:                make(map[string]*models.Territory),
		Pieces:               make(map[int]*models.Piece),
		GlobalPieceTemplates: make(map[string]*models.Piece),
		PurchasedUnits:       make(map[string][]*models.PendingUnit),
	}

	// Add a player
	player := &models.Player{
		Name:           "USSR",
		IPCs:           50,
		NPC:            false,
		Territories:    []*models.Territory{},
		PieceTemplates: make(map[string]*models.Piece),
	}
	g.Players["USSR"] = player

	// Add a territory
	territory := &models.Territory{
		Name:       "Moscow",
		Terrain:    models.Land,
		Production: 8,
		Owner:      player,
		Pieces:     []int{},
	}
	g.Board["Moscow"] = territory
	player.Territories = append(player.Territories, territory)

	// Add unit templates
	infantry := &models.Piece{
		Name:     "infantry",
		Terrain:  models.Land,
		Movement: 1,
		Attack:   1,
		Defend:   2,
		Cost:     3,
	}
	g.GlobalPieceTemplates["infantry"] = infantry

	// Create controller
	controller := game.NewGameController(g)
	err := controller.StartGame()
	if err != nil {
		t.Fatalf("Failed to start game: %v", err)
	}

	// Test commands
	tests := []struct {
		name         string
		command      string
		expectError  bool
		checkPhase   *models.Phase
		checkMessage string
	}{
		{
			name:        "help command",
			command:     "help",
			expectError: false,
		},
		{
			name:        "status command",
			command:     "status",
			expectError: false,
		},
		{
			name:        "income command",
			command:     "income",
			expectError: false,
		},
		{
			name:        "board command",
			command:     "board Moscow",
			expectError: false,
		},
		{
			name:        "buy infantry",
			command:     "buy infantry 2",
			expectError: false,
		},
		{
			name:       "advance to combat move phase",
			command:    "done",
			checkPhase: ptrPhase(models.CombatMovePhase),
		},
		{
			name:       "advance to conduct combat phase",
			command:    "done",
			checkPhase: ptrPhase(models.ConductCombatPhase),
		},
		{
			name:       "advance to noncombat move phase",
			command:    "done",
			checkPhase: ptrPhase(models.NoncombatMovePhase),
		},
		{
			name:       "advance to mobilize phase",
			command:    "done",
			checkPhase: ptrPhase(models.MobilizePhase),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terminal := newTestTerminal(controller)
			err := terminal.ProcessCommand(tt.command)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.checkPhase != nil {
				if controller.Game.CurrentPhase != *tt.checkPhase {
					t.Errorf("Expected phase %v, got %v", *tt.checkPhase, controller.Game.CurrentPhase)
				}
			}
		})
	}
}

// TestTerminalPurchaseAndMobilize tests the full purchase -> mobilize flow
func TestTerminalPurchaseAndMobilize(t *testing.T) {
	// Create a minimal game
	g := &models.Game{
		Players:              make(map[string]*models.Player),
		PlayerOrder:          []string{"Germany"},
		Board:                make(map[string]*models.Territory),
		Pieces:               make(map[int]*models.Piece),
		GlobalPieceTemplates: make(map[string]*models.Piece),
		PurchasedUnits:       make(map[string][]*models.PendingUnit),
	}

	// Add a player
	player := &models.Player{
		Name:           "Germany",
		IPCs:           30,
		NPC:            false,
		Territories:    []*models.Territory{},
		PieceTemplates: make(map[string]*models.Piece),
	}
	g.Players["Germany"] = player

	// Add a territory
	berlin := &models.Territory{
		Name:       "Berlin",
		Terrain:    models.Land,
		Production: 10,
		Owner:      player,
		Pieces:     []int{},
	}
	g.Board["Berlin"] = berlin
	player.Territories = append(player.Territories, berlin)

	// Berlin builds units, so it needs an industrial complex.
	g.AddPieceTemplate("factory", models.Land, 0, 0, 0, 32)
	if err := g.PlacePieces("Berlin", "factory", 1); err != nil {
		panic(err)
	}

	// Add unit template
	tank := &models.Piece{
		Name:     "armor",
		Terrain:  models.Land,
		Movement: 2,
		Attack:   3,
		Defend:   3,
		Cost:     5,
	}
	g.GlobalPieceTemplates["armor"] = tank
	g.NextPieceID = 1

	// Create controller and terminal
	controller := game.NewGameController(g)
	err := controller.StartGame()
	if err != nil {
		t.Fatalf("Failed to start game: %v", err)
	}

	terminal := newTestTerminal(controller)

	// Purchase phase: buy 2 tanks (10 IPCs)
	err = terminal.ProcessCommand("buy armor 2")
	if err != nil {
		t.Fatalf("Failed to buy armor: %v", err)
	}

	if player.IPCs != 20 {
		t.Errorf("Expected 20 IPCs remaining, got %d", player.IPCs)
	}

	if len(g.PurchasedUnits["Germany"]) != 2 {
		t.Errorf("Expected 2 purchased units, got %d", len(g.PurchasedUnits["Germany"]))
	}

	// Advance through phases to mobilize
	terminal.ProcessCommand("done") // -> Combat Move
	terminal.ProcessCommand("done") // -> Conduct Combat
	terminal.ProcessCommand("done") // -> Noncombat Move
	terminal.ProcessCommand("done") // -> Mobilize

	if controller.Game.CurrentPhase != models.MobilizePhase {
		t.Fatalf("Expected Mobilize phase, got %v", controller.Game.CurrentPhase)
	}

	// Place the units
	err = terminal.ProcessCommand("place armor Berlin 2")
	if err != nil {
		t.Fatalf("Failed to place armor: %v", err)
	}

	// Verify units are on the board. Three, not two: Berlin's industrial
	// complex is a piece as well as the two units just placed.
	if len(berlin.Pieces) != 3 {
		t.Errorf("Expected 3 pieces in Berlin (2 units plus the factory), got %d",
			len(berlin.Pieces))
	}

	// Verify purchased units list is empty
	if len(g.PurchasedUnits["Germany"]) != 0 {
		t.Errorf("Expected 0 purchased units remaining, got %d", len(g.PurchasedUnits["Germany"]))
	}
}

// TestTerminalIncomeCollection tests that income is collected properly
func TestTerminalIncomeCollection(t *testing.T) {
	// Create a minimal game
	g := &models.Game{
		Players:              make(map[string]*models.Player),
		PlayerOrder:          []string{"UK"},
		Board:                make(map[string]*models.Territory),
		Pieces:               make(map[int]*models.Piece),
		GlobalPieceTemplates: make(map[string]*models.Piece),
		PurchasedUnits:       make(map[string][]*models.PendingUnit),
	}

	// Add a player
	player := &models.Player{
		Name:           "UK",
		IPCs:           10,
		NPC:            false,
		Territories:    []*models.Territory{},
		PieceTemplates: make(map[string]*models.Piece),
	}
	g.Players["UK"] = player

	// Add territories with production
	london := &models.Territory{
		Name:       "London",
		Terrain:    models.Land,
		Production: 8,
		Owner:      player,
		Pieces:     []int{},
	}
	g.Board["London"] = london
	player.Territories = append(player.Territories, london)

	india := &models.Territory{
		Name:       "India",
		Terrain:    models.Land,
		Production: 3,
		Owner:      player,
		Pieces:     []int{},
	}
	g.Board["India"] = india
	player.Territories = append(player.Territories, india)

	// Create controller
	controller := game.NewGameController(g)
	err := controller.StartGame()
	if err != nil {
		t.Fatalf("Failed to start game: %v", err)
	}

	terminal := newTestTerminal(controller)

	// Advance through all phases to collect income
	terminal.ProcessCommand("done") // -> Combat Move
	terminal.ProcessCommand("done") // -> Conduct Combat
	terminal.ProcessCommand("done") // -> Noncombat Move
	terminal.ProcessCommand("done") // -> Mobilize
	terminal.ProcessCommand("done") // -> Collect Income
	terminal.ProcessCommand("done") // -> Collect income & advance turn

	// Income should be collected: 10 + 8 (London) + 3 (India) = 21
	expectedIPCs := 10 + 8 + 3
	if player.IPCs != expectedIPCs {
		t.Errorf("Expected %d IPCs after income collection, got %d", expectedIPCs, player.IPCs)
	}

	// Should be back to Purchase phase for next turn
	if controller.Game.CurrentPhase != models.PurchasePhase {
		t.Errorf("Expected Purchase phase after turn end, got %v", controller.Game.CurrentPhase)
	}
}

// TestTerminalQuitCommand tests the quit command
func TestTerminalQuitCommand(t *testing.T) {
	g := &models.Game{
		Players:              make(map[string]*models.Player),
		PlayerOrder:          []string{"USA"},
		Board:                make(map[string]*models.Territory),
		Pieces:               make(map[int]*models.Piece),
		GlobalPieceTemplates: make(map[string]*models.Piece),
		PurchasedUnits:       make(map[string][]*models.PendingUnit),
	}

	player := &models.Player{
		Name:           "USA",
		IPCs:           50,
		NPC:            false,
		Territories:    []*models.Territory{},
		PieceTemplates: make(map[string]*models.Piece),
	}
	g.Players["USA"] = player

	controller := game.NewGameController(g)
	controller.StartGame()

	terminal := newTestTerminal(controller)
	terminal.Running = true

	err := terminal.ProcessCommand("quit")
	if err != nil {
		t.Errorf("Quit command returned error: %v", err)
	}

	if terminal.Running {
		t.Error("Terminal should not be running after quit command")
	}
}

// TestTerminalWithSimulatedInput tests the full Run() loop with simulated input
func TestTerminalWithSimulatedInput(t *testing.T) {
	// Create a minimal game
	g := &models.Game{
		Players:              make(map[string]*models.Player),
		PlayerOrder:          []string{"Japan"},
		Board:                make(map[string]*models.Territory),
		Pieces:               make(map[int]*models.Piece),
		GlobalPieceTemplates: make(map[string]*models.Piece),
		PurchasedUnits:       make(map[string][]*models.PendingUnit),
	}

	player := &models.Player{
		Name:           "Japan",
		IPCs:           40,
		NPC:            false,
		Territories:    []*models.Territory{},
		PieceTemplates: make(map[string]*models.Piece),
	}
	g.Players["Japan"] = player

	tokyo := &models.Territory{
		Name:       "Tokyo",
		Terrain:    models.Land,
		Production: 8,
		Owner:      player,
		Pieces:     []int{},
	}
	g.Board["Tokyo"] = tokyo
	player.Territories = append(player.Territories, tokyo)

	// Create controller
	controller := game.NewGameController(g)

	// Simulate user input: select nation (1), status, help, then quit
	input := "1\nstatus\nhelp\nquit\n"
	reader := bufio.NewReader(strings.NewReader(input))

	terminal := &Terminal{
		Controller: controller,
		Reader:     reader,
		Running:    true,
	}

	// Start and run terminal (should quit after commands)
	err := controller.StartGame()
	if err != nil {
		t.Fatalf("Failed to start game: %v", err)
	}

	// Capture output to suppress it during test
	var buf bytes.Buffer
	// Note: We can't easily capture stdout in this test without more infrastructure,
	// so we'll just verify the commands execute without error

	err = terminal.Run()
	if err != nil {
		t.Errorf("Terminal.Run() returned error: %v", err)
	}

	if terminal.Running {
		t.Error("Terminal should have stopped after quit command")
	}

	// Verify we can read from buffer (even if empty)
	_ = buf.String()
}

// Helper function to create phase pointer
func ptrPhase(p models.Phase) *models.Phase {
	return &p
}
