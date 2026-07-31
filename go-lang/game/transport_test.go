package game

import (
	"boardgame/models"
	"testing"
)

func setupTransportTest() (*models.Game, int, int) {
	game := models.NewGame()

	// Create territories
	game.AddTerritory("France", models.Land, "Germany", 3)
	game.AddTerritory("North Sea", models.Water, "Germany", 0)
	game.AddTerritory("UK", models.Land, "UK", 8)

	// Connect territories
	game.ConnectTerritories("France", "North Sea")
	game.ConnectTerritories("North Sea", "France")
	game.ConnectTerritories("North Sea", "UK")
	game.ConnectTerritories("UK", "North Sea")

	// Add piece templates
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.AddPieceTemplate("transport", models.Water, 2, 0, 1, 8)

	// Set transport capacity
	game.SetContainerCapacity("transport", 2, []string{"infantry"})

	// Place pieces
	game.PlacePieces("France", "infantry", 2)
	game.PlacePieces("North Sea", "transport", 1)

	// Get piece IDs
	infantryID := 1
	transportID := 3

	return game, infantryID, transportID
}

func TestLoadPiece(t *testing.T) {
	game, infantryID, transportID := setupTransportTest()

	// Load infantry onto transport
	err := game.LoadPiece(transportID, infantryID)
	if err != nil {
		t.Fatalf("LoadPiece failed: %v", err)
	}

	// Check infantry is in transport's Holding
	transport := game.Pieces[transportID]
	if len(transport.Holding) != 1 {
		t.Errorf("Expected 1 piece in transport, got %d", len(transport.Holding))
	}
	if transport.Holding[0] != infantryID {
		t.Errorf("Expected infantry %d in transport, got %d", infantryID, transport.Holding[0])
	}

	// Check infantry is removed from France
	france := game.Board["France"]
	for _, id := range france.Pieces {
		if id == infantryID {
			t.Errorf("Infantry %d should be removed from France", infantryID)
		}
	}

	// Check IsLoaded
	if !game.IsLoaded(infantryID) {
		t.Error("Infantry should be marked as loaded")
	}

	// Check GetTransportForPiece
	foundTransport := game.GetTransportForPiece(infantryID)
	if foundTransport != transportID {
		t.Errorf("Expected transport %d, got %d", transportID, foundTransport)
	}
}

func TestLoadPieceCapacity(t *testing.T) {
	game, _, transportID := setupTransportTest()

	// Load two infantry (at capacity)
	err := game.LoadPiece(transportID, 1)
	if err != nil {
		t.Fatalf("First load failed: %v", err)
	}

	err = game.LoadPiece(transportID, 2)
	if err != nil {
		t.Fatalf("Second load failed: %v", err)
	}

	// Try to load a third (should fail)
	game.PlacePieces("France", "infantry", 1)
	thirdInfantryID := 4

	err = game.LoadPiece(transportID, thirdInfantryID)
	if err == nil {
		t.Error("Should not be able to load beyond capacity")
	}
}

func TestLoadPieceWrongType(t *testing.T) {
	game := models.NewGame()

	game.AddTerritory("North Sea", models.Water, "Germany", 0)

	game.AddPieceTemplate("transport", models.Water, 2, 0, 1, 8)
	game.AddPieceTemplate("tank", models.Land, 2, 3, 3, 5)

	game.SetContainerCapacity("transport", 2, []string{"infantry"}) // Can only carry infantry

	game.PlacePieces("North Sea", "transport", 1)
	game.PlacePieces("North Sea", "tank", 1)

	transportID := 1
	tankID := 2

	err := game.LoadPiece(transportID, tankID)
	if err == nil {
		t.Error("Should not be able to load tank on infantry-only transport")
	}
}

func TestUnloadPiece(t *testing.T) {
	game, infantryID, transportID := setupTransportTest()

	// Load infantry
	err := game.LoadPiece(transportID, infantryID)
	if err != nil {
		t.Fatalf("LoadPiece failed: %v", err)
	}

	// Unload to UK
	err = game.UnloadPiece(transportID, infantryID, "UK")
	if err != nil {
		t.Fatalf("UnloadPiece failed: %v", err)
	}

	// Check infantry is removed from transport
	transport := game.Pieces[transportID]
	if len(transport.Holding) != 0 {
		t.Errorf("Expected 0 pieces in transport, got %d", len(transport.Holding))
	}

	// Check infantry is in UK
	uk := game.Board["UK"]
	found := false
	for _, id := range uk.Pieces {
		if id == infantryID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Infantry should be in UK territory")
	}

	// Check IsLoaded
	if game.IsLoaded(infantryID) {
		t.Error("Infantry should not be marked as loaded")
	}
}

func TestUnloadPieceNotLoaded(t *testing.T) {
	game, infantryID, transportID := setupTransportTest()

	// Try to unload without loading first
	err := game.UnloadPiece(transportID, infantryID, "UK")
	if err == nil {
		t.Error("Should not be able to unload piece that is not loaded")
	}
}

func TestValidateLoad(t *testing.T) {
	game, infantryID, transportID := setupTransportTest()

	// Valid load
	err := ValidateLoad(game, transportID, infantryID, "Germany")
	if err != nil {
		t.Errorf("ValidateLoad should succeed: %v", err)
	}

	// Try to load from wrong player
	err = ValidateLoad(game, transportID, infantryID, "UK")
	if err == nil {
		t.Error("Should not be able to load enemy pieces")
	}
}

func TestValidateUnload(t *testing.T) {
	game, infantryID, transportID := setupTransportTest()

	// Load first
	err := game.LoadPiece(transportID, infantryID)
	if err != nil {
		t.Fatalf("LoadPiece failed: %v", err)
	}

	// Valid unload to adjacent territory
	err = ValidateUnload(game, transportID, infantryID, "UK")
	if err != nil {
		t.Errorf("ValidateUnload should succeed: %v", err)
	}

	// Invalid unload to non-adjacent territory
	game.AddTerritory("Japan", models.Land, "Japan", 8)
	err = ValidateUnload(game, transportID, infantryID, "Japan")
	if err == nil {
		t.Error("Should not be able to unload to non-adjacent territory")
	}
}

func TestControllerLoadUnit(t *testing.T) {
	game, infantryID, transportID := setupTransportTest()

	game.PlayerOrder = []string{"Germany", "UK"}
	game.CurrentPower = "Germany"
	game.CurrentPhase = models.NoncombatMovePhase

	gc := NewGameController(game)

	// Load unit
	err := gc.LoadUnit(transportID, infantryID)
	if err != nil {
		t.Fatalf("LoadUnit failed: %v", err)
	}

	// Check it's loaded
	if !game.IsLoaded(infantryID) {
		t.Error("Infantry should be loaded")
	}
}

// Loading is legal in either movement phase.
//
// This test used to assert the opposite for the combat-move phase, which made
// an amphibious assault impossible: the rules have a transport load, sail and
// land within combat movement, so a landing force could never get aboard.
func TestControllerLoadUnitDuringCombatMove(t *testing.T) {
	game, infantryID, transportID := setupTransportTest()

	game.PlayerOrder = []string{"Germany", "UK"}
	game.CurrentPower = "Germany"
	game.CurrentPhase = models.CombatMovePhase

	gc := NewGameController(game)

	if err := gc.LoadUnit(transportID, infantryID); err != nil {
		t.Errorf("loading during combat movement should be allowed: %v", err)
	}
}

// Outside a movement phase there is no loading at all.
func TestControllerLoadUnitOutsideMovement(t *testing.T) {
	for _, phase := range []models.Phase{
		models.PurchasePhase,
		models.ConductCombatPhase,
		models.MobilizePhase,
		models.CollectIncomePhase,
	} {
		game, infantryID, transportID := setupTransportTest()
		game.PlayerOrder = []string{"Germany", "UK"}
		game.CurrentPower = "Germany"
		game.CurrentPhase = phase

		gc := NewGameController(game)
		if err := gc.LoadUnit(transportID, infantryID); err == nil {
			t.Errorf("loading should be refused during %s", phase)
		}
	}
}

func TestControllerUnloadUnit(t *testing.T) {
	game, infantryID, transportID := setupTransportTest()

	game.PlayerOrder = []string{"Germany", "UK"}
	game.CurrentPower = "Germany"
	game.CurrentPhase = models.NoncombatMovePhase

	gc := NewGameController(game)

	// Load first
	err := gc.LoadUnit(transportID, infantryID)
	if err != nil {
		t.Fatalf("LoadUnit failed: %v", err)
	}

	// Unload to UK
	err = gc.UnloadUnit(transportID, infantryID, "UK")
	if err != nil {
		t.Fatalf("UnloadUnit failed: %v", err)
	}

	// Check it's unloaded
	if game.IsLoaded(infantryID) {
		t.Error("Infantry should not be loaded")
	}

	// Check it's in UK
	uk := game.Board["UK"]
	found := false
	for _, id := range uk.Pieces {
		if id == infantryID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Infantry should be in UK")
	}
}

func TestGetTransportCargo(t *testing.T) {
	game, infantryID, transportID := setupTransportTest()

	game.PlayerOrder = []string{"Germany"}
	game.CurrentPower = "Germany"

	gc := NewGameController(game)

	// Load infantry
	game.LoadPiece(transportID, infantryID)

	// Get cargo
	cargo, err := gc.GetTransportCargo(transportID)
	if err != nil {
		t.Fatalf("GetTransportCargo failed: %v", err)
	}

	if len(cargo) != 1 {
		t.Errorf("Expected 1 cargo item, got %d", len(cargo))
	}

	if cargo[0] != infantryID {
		t.Errorf("Expected infantry %d in cargo, got %d", infantryID, cargo[0])
	}
}

// Loading is about who owns the pieces, not who nominally owns the water.
//
// The old validation required the sea zone's owner to be the current player --
// sea-zone ownership being exactly the thing the movement rules treat as a
// meaningless starting marker -- and never checked the transport or the
// troops, so a power could load its infantry into anyone's shipping.
func TestValidateLoad_ChecksPieceOwnershipNotSeaOwnership(t *testing.T) {
	g := models.NewGame()
	g.PlayerOrder = []string{"Germany", "UK"}
	germany := g.GetOrCreatePlayer("Germany")
	uk := g.GetOrCreatePlayer("UK")
	germany.Side, uk.Side = "Axis", "Allies"
	germany.TakesTurns, uk.TakesTurns = true, true

	g.AddTerritory("Port", models.Land, "Germany", 2)
	// The sea zone nominally belongs to UK -- which must not matter.
	g.AddTerritory("Roadstead", models.Water, "UK", 0)
	g.ConnectTerritories("Port", "Roadstead")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("transport", models.Water, 2, 0, 1, 10)
	g.GlobalPieceTemplates["transport"].Capacity = 2
	g.GlobalPieceTemplates["transport"].CanCarry = []string{"infantry"}

	g.PlacePieces("Port", "infantry", 1)
	g.PlacePieces("Roadstead", "transport", 1)
	infantryID := g.Board["Port"].Pieces[0]
	transportID := g.Board["Roadstead"].Pieces[0]
	g.Pieces[transportID].Owner = germany // our ship, in "UK" water

	if err := ValidateLoad(g, transportID, infantryID, "Germany"); err != nil {
		t.Errorf("loading our own transport in nominally foreign water refused: %v", err)
	}

	// An enemy transport in the same water is not ours to load.
	g.Pieces[transportID].Owner = uk
	if err := ValidateLoad(g, transportID, infantryID, "Germany"); err == nil {
		t.Error("loading our infantry into an enemy transport was allowed")
	}
}
