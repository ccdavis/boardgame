package game

import (
	"boardgame/models"
	"testing"
)

// TestAlliesFunctionBasic tests the basic areAllies function
func TestAlliesFunctionBasic(t *testing.T) {
	germany := &models.Player{Name: "Germany", Side: "Axis"}
	japan := &models.Player{Name: "Japan", Side: "Axis"}
	ussr := &models.Player{Name: "USSR", Side: "Allies"}
	uk := &models.Player{Name: "UK", Side: "Allies"}
	neutral := &models.Player{Name: "Neutral", Side: ""}

	// Axis powers are allies
	if !areAllies(germany, japan) {
		t.Error("Germany and Japan should be allies")
	}

	// Allied powers are allies
	if !areAllies(ussr, uk) {
		t.Error("USSR and UK should be allies")
	}

	// Axis and Allies are not allies
	if areAllies(germany, ussr) {
		t.Error("Germany and USSR should not be allies")
	}

	// Neutrals are not allies with anyone
	if areAllies(neutral, germany) {
		t.Error("Neutral should not be allies with Germany")
	}

	// Player is ally with itself
	if !areAllies(germany, germany) {
		t.Error("Germany should be allies with itself")
	}
}

// TestCannotAttackAlliedTerritory tests that combat moves cannot target allied territories
func TestCannotAttackAlliedTerritory(t *testing.T) {
	// Set up a simple game with two Axis powers and one Allied power
	game := models.NewGame()

	// Create players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"
	japan := game.GetOrCreatePlayer("Japan")
	japan.Side = "Axis"
	ussr := game.GetOrCreatePlayer("USSR")
	ussr.Side = "Allies"

	// Create territories
	game.AddTerritory("Berlin", models.Land, "Germany", 3)
	game.AddTerritory("Tokyo", models.Land, "Japan", 3)
	game.AddTerritory("Moscow", models.Land, "USSR", 3)
	game.AddTerritory("Poland", models.Land, "Germany", 1)

	// Connect territories: Berlin <-> Poland <-> Moscow <-> Tokyo
	game.ConnectTerritories("Berlin", "Poland")
	game.ConnectTerritories("Poland", "Berlin")
	game.ConnectTerritories("Poland", "Moscow")
	game.ConnectTerritories("Moscow", "Poland")
	game.ConnectTerritories("Moscow", "Tokyo")
	game.ConnectTerritories("Tokyo", "Moscow")

	// Add infantry units
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.PlacePieces("Berlin", "infantry", 2)
	game.PlacePieces("Tokyo", "infantry", 2)
	game.PlacePieces("Moscow", "infantry", 2)

	// Find the infantry in Berlin
	berlinTerritory := game.Board["Berlin"]
	infantryID := berlinTerritory.Pieces[0]

	// Test 1: Germany should NOT be able to attack Japan (both Axis)
	_, _, err := CalculateMovementPathForPiece(game, game.Pieces[infantryID], "Berlin", "Tokyo", germany, CombatMove)
	if err == nil {
		t.Error("Germany should not be able to attack allied Japanese territory Tokyo")
	}

	// Test 2: Germany SHOULD be able to attack USSR (enemy)
	_, _, err = CalculateMovementPathForPiece(game, game.Pieces[infantryID], "Berlin", "Moscow", germany, CombatMove)
	if err != nil {
		t.Errorf("Germany should be able to attack enemy USSR territory Moscow: %v", err)
	}
}

// TestNPCAIDoesNotAttackAllies tests that NPC AI doesn't target allied territories
func TestNPCAIDoesNotAttackAllies(t *testing.T) {
	// Set up a game
	game := models.NewGame()

	// Create players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"
	japan := game.GetOrCreatePlayer("Japan")
	japan.Side = "Axis"
	ussr := game.GetOrCreatePlayer("USSR")
	ussr.Side = "Allies"

	// Create territories
	game.AddTerritory("Berlin", models.Land, "Germany", 3)
	game.AddTerritory("Poland", models.Land, "Germany", 1)
	game.AddTerritory("Tokyo", models.Land, "Japan", 3)
	game.AddTerritory("Moscow", models.Land, "USSR", 3)

	// Connect: Berlin <-> Poland <-> Tokyo (Japan)
	//          Poland <-> Moscow (USSR)
	game.ConnectTerritories("Berlin", "Poland")
	game.ConnectTerritories("Poland", "Berlin")
	game.ConnectTerritories("Poland", "Tokyo")
	game.ConnectTerritories("Tokyo", "Poland")
	game.ConnectTerritories("Poland", "Moscow")
	game.ConnectTerritories("Moscow", "Poland")

	// Create NPC AI for Germany
	npc := NewNPCAIPlayer("Germany AI", "normal")

	// Find attack targets from Germany's perspective
	targets := npc.findAttackTargets(game, germany)

	// Verify that Tokyo (Japan, Axis ally) is NOT in the targets
	for _, target := range targets {
		if target.Name == "Tokyo" {
			t.Error("NPC AI should not target allied Japanese territory Tokyo")
		}
	}

	// Verify that Moscow (USSR, Allied enemy) IS in the targets
	foundMoscow := false
	for _, target := range targets {
		if target.Name == "Moscow" {
			foundMoscow = true
			break
		}
	}
	if !foundMoscow {
		t.Error("NPC AI should target enemy USSR territory Moscow")
	}
}

// TestControllerCombatMoveIntoAlliedTerritoryIsNotAnAttack: a combat-phase
// move onto an ally's ground is allowed -- it is a friendly move, the same as
// a move onto our own ground -- but it is never an attack: no battle is
// staged and the territory stays the ally's. (Refusing the move outright, as
// this test once demanded, stranded units whose only neighbour was allied.)
func TestControllerCombatMoveIntoAlliedTerritoryIsNotAnAttack(t *testing.T) {
	// Set up game
	game := models.NewGame()

	// Create players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"
	japan := game.GetOrCreatePlayer("Japan")
	japan.Side = "Axis"

	game.PlayerOrder = []string{"Germany", "Japan"}

	// Create territories
	game.AddTerritory("Berlin", models.Land, "Germany", 3)
	game.AddTerritory("Tokyo", models.Land, "Japan", 3)
	game.ConnectTerritories("Berlin", "Tokyo")
	game.ConnectTerritories("Tokyo", "Berlin")

	// Add infantry
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.PlacePieces("Berlin", "infantry", 1)

	// Create controller
	controller := NewGameController(game)
	controller.StartGame()

	// Set phase to combat move
	game.CurrentPhase = models.CombatMovePhase

	// Try to plan a move from Berlin to Tokyo (both Axis)
	berlinTerritory := game.Board["Berlin"]
	infantryID := berlinTerritory.Pieces[0]

	err := controller.PlanMove(infantryID, "Berlin", "Tokyo")
	if err != nil {
		t.Fatalf("combat-phase move into allied Tokyo should be allowed: %v", err)
	}
	if attacks := controller.GetPlannedAttacks(); len(attacks) != 0 {
		t.Errorf("move into allied territory listed as an attack: %v", attacks)
	}
	if err := controller.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing combat moves: %v", err)
	}
	if len(controller.PendingBattles) != 0 {
		t.Errorf("a battle was staged in allied territory: %v", controller.PendingBattles)
	}
	if game.Board["Tokyo"].Owner != japan {
		t.Errorf("Tokyo changed hands to %s; allies do not capture each other's ground",
			game.Board["Tokyo"].Owner.Name)
	}
	if len(game.Board["Tokyo"].Pieces) != 1 || game.Pieces[infantryID].Owner != germany {
		t.Errorf("the infantry should be standing in Tokyo, still German")
	}
}

// TestCanMoveToAlliedTerritoryNoncombat tests that noncombat moves CAN go to allied territories
func TestCanMoveToAlliedTerritoryNoncombat(t *testing.T) {
	// Set up game
	game := models.NewGame()

	// Create players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"
	japan := game.GetOrCreatePlayer("Japan")
	japan.Side = "Axis"

	game.PlayerOrder = []string{"Germany", "Japan"}

	// Create territories
	game.AddTerritory("Berlin", models.Land, "Germany", 3)
	game.AddTerritory("Tokyo", models.Land, "Japan", 3)
	game.ConnectTerritories("Berlin", "Tokyo")
	game.ConnectTerritories("Tokyo", "Berlin")

	// Add infantry
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.PlacePieces("Berlin", "infantry", 1)

	// Create controller
	controller := NewGameController(game)
	controller.StartGame()

	// Set phase to noncombat move
	game.CurrentPhase = models.NoncombatMovePhase

	// Try to plan a noncombat move from Berlin to Tokyo (both Axis)
	// This SHOULD work - allies can move into each other's territories during noncombat
	berlinTerritory := game.Board["Berlin"]
	infantryID := berlinTerritory.Pieces[0]

	err := controller.PlanMove(infantryID, "Berlin", "Tokyo")
	if err != nil {
		t.Errorf("Noncombat move to allied territory should be allowed: %v", err)
	}
}
