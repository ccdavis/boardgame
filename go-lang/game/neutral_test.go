package game

import (
	"boardgame/models"
	"testing"
)

// TestCannotAttackStrictNeutral verifies that strict neutrals cannot be attacked
func TestCannotAttackStrictNeutral(t *testing.T) {
	game := models.NewGame()

	// Create players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"
	usa := game.GetOrCreatePlayer("USA")
	usa.Side = "Allies"

	game.PlayerOrder = []string{"Germany", "USA"}

	// Create territories - Turkey is a strict neutral
	game.AddTerritory("Berlin", models.Land, "Germany", 3)
	game.AddTerritory("Turkey", models.Land, "Neutral", 4) // This will be marked as StrictNeutral
	game.ConnectTerritories("Berlin", "Turkey")
	game.ConnectTerritories("Turkey", "Berlin")

	// Add infantry
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.PlacePieces("Berlin", "infantry", 1)

	// Verify Turkey is a strict neutral
	turkey := game.Board["Turkey"]
	if turkey.NeutralType != models.StrictNeutral {
		t.Errorf("Turkey should be a strict neutral, got %v", turkey.NeutralType)
	}

	// Create controller
	controller := NewGameController(game)
	controller.StartGame()
	game.CurrentPhase = models.CombatMovePhase

	// Try to plan a combat move from Berlin to Turkey
	berlinTerritory := game.Board["Berlin"]
	infantryID := berlinTerritory.Pieces[0]

	err := controller.PlanMove(infantryID, "Berlin", "Turkey")
	if err == nil {
		t.Error("Should not be able to attack strict neutral Turkey")
	}
}

// TestCanActivateProAlliedNeutral verifies that Allied powers can activate pro-Allied neutrals
func TestCanActivateProAlliedNeutral(t *testing.T) {
	game := models.NewGame()

	// Create players
	usa := game.GetOrCreatePlayer("USA")
	usa.Side = "Allies"

	game.PlayerOrder = []string{"USA"}

	// Create territories - Argentina is pro-Allied
	game.AddTerritory("Brazil", models.Land, "USA", 3)
	game.AddTerritory("Argentina", models.Land, "Neutral", 1)
	// Declared, as the board's Neutrality section declares it. Neutral land
	// defaults to strict until the data says otherwise.
	game.Board["Argentina"].NeutralType = models.ProAlliedNeutral
	game.ConnectTerritories("Brazil", "Argentina")
	game.ConnectTerritories("Argentina", "Brazil")

	// Add infantry
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.PlacePieces("Brazil", "infantry", 1)

	// Verify Argentina is pro-Allied
	argentina := game.Board["Argentina"]
	if argentina.NeutralType != models.ProAlliedNeutral {
		t.Errorf("Argentina should be pro-Allied neutral, got %v", argentina.NeutralType)
	}

	// Create controller
	controller := NewGameController(game)
	controller.StartGame()
	game.CurrentPhase = models.NoncombatMovePhase

	// Plan a noncombat move from Brazil to Argentina
	brazilTerritory := game.Board["Brazil"]
	infantryID := brazilTerritory.Pieces[0]

	err := controller.PlanMove(infantryID, "Brazil", "Argentina")
	if err != nil {
		t.Errorf("Should be able to activate pro-Allied neutral Argentina: %v", err)
	}

	// Execute the noncombat move
	err = controller.ExecuteNoncombatMoves()
	if err != nil {
		t.Errorf("Failed to execute noncombat move: %v", err)
	}

	// Verify Argentina is now owned by USA
	if argentina.Owner != usa {
		t.Errorf("Argentina should be owned by USA, got %v", argentina.Owner.Name)
	}

	// Verify free infantry were placed
	if len(argentina.Pieces) < 1 {
		t.Error("Argentina should have received free infantry")
	}
}

// TestCannotActivateProAlliedNeutralAsAxis verifies that Axis cannot activate pro-Allied neutrals
func TestCannotActivateProAlliedNeutralAsAxis(t *testing.T) {
	game := models.NewGame()

	// Create players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"

	game.PlayerOrder = []string{"Germany"}

	// Create territories
	game.AddTerritory("Southern Europe", models.Land, "Germany", 2)
	game.AddTerritory("Argentina", models.Land, "Neutral", 1)
	game.Board["Argentina"].NeutralType = models.ProAlliedNeutral
	game.ConnectTerritories("Southern Europe", "Argentina")
	game.ConnectTerritories("Argentina", "Southern Europe")

	// Add infantry
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.PlacePieces("Southern Europe", "infantry", 1)

	// Create controller
	controller := NewGameController(game)
	controller.StartGame()
	game.CurrentPhase = models.NoncombatMovePhase

	// Try to plan a noncombat move to Argentina
	seTerritory := game.Board["Southern Europe"]
	infantryID := seTerritory.Pieces[0]

	err := controller.PlanMove(infantryID, "Southern Europe", "Argentina")
	if err == nil {
		t.Error("Axis should not be able to activate pro-Allied neutral Argentina")
	}
}

// TestAxisCanAttackProAlliedNeutral verifies that Axis can attack pro-Allied neutrals
func TestAxisCanAttackProAlliedNeutral(t *testing.T) {
	game := models.NewGame()

	// Create players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"

	game.PlayerOrder = []string{"Germany"}

	// Create territories
	game.AddTerritory("Southern Europe", models.Land, "Germany", 2)
	game.AddTerritory("Argentina", models.Land, "Neutral", 1)
	game.Board["Argentina"].NeutralType = models.ProAlliedNeutral
	game.ConnectTerritories("Southern Europe", "Argentina")
	game.ConnectTerritories("Argentina", "Southern Europe")

	// Add infantry
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.PlacePieces("Southern Europe", "infantry", 1)

	// Create controller
	controller := NewGameController(game)
	controller.StartGame()
	game.CurrentPhase = models.CombatMovePhase

	// Plan a combat move to Argentina
	seTerritory := game.Board["Southern Europe"]
	infantryID := seTerritory.Pieces[0]

	err := controller.PlanMove(infantryID, "Southern Europe", "Argentina")
	if err != nil {
		t.Errorf("Axis should be able to attack pro-Allied neutral Argentina: %v", err)
	}
}

// TestStrictNeutralChainReaction verifies that attacking one strict neutral makes all hostile
func TestStrictNeutralChainReaction(t *testing.T) {
	game := models.NewGame()

	// Create players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"
	usa := game.GetOrCreatePlayer("USA")
	usa.Side = "Allies"

	// Both powers play, as a parsed board would declare. The chain reaction
	// hands the neutrals to the first opposing power in turn order.
	game.PlayerOrder = []string{"Germany", "USA"}
	germany.TakesTurns = true
	usa.TakesTurns = true

	// Create territories - multiple strict neutrals
	game.AddTerritory("Southern Europe", models.Land, "Germany", 2)
	game.AddTerritory("Turkey", models.Land, "Neutral", 4)       // Strict neutral
	game.AddTerritory("Afghanistan", models.Land, "Neutral", 1)  // Strict neutral
	game.ConnectTerritories("Southern Europe", "Turkey")
	game.ConnectTerritories("Turkey", "Southern Europe")

	// Add infantry
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.PlacePieces("Southern Europe", "infantry", 2)

	// Verify both are strict neutrals
	turkey := game.Board["Turkey"]
	afghanistan := game.Board["Afghanistan"]
	if turkey.NeutralType != models.StrictNeutral {
		t.Error("Turkey should be strict neutral")
	}
	if afghanistan.NeutralType != models.StrictNeutral {
		t.Error("Afghanistan should be strict neutral")
	}

	// Create controller and trigger chain reaction
	controller := NewGameController(game)
	controller.StartGame()

	// Manually trigger the chain reaction (simulating an attack)
	controller.TriggerStrictNeutralChainReaction(germany)

	// Verify both strict neutrals are now owned by USA (enemy of Germany)
	if turkey.Owner.Name != "USA" {
		t.Errorf("Turkey should be owned by USA after chain reaction, got %v", turkey.Owner.Name)
	}
	if afghanistan.Owner.Name != "USA" {
		t.Errorf("Afghanistan should be owned by USA after chain reaction, got %v", afghanistan.Owner.Name)
	}

	// Verify they have defending infantry
	if len(turkey.Pieces) < 1 {
		t.Error("Turkey should have defending infantry")
	}
	if len(afghanistan.Pieces) < 1 {
		t.Error("Afghanistan should have defending infantry")
	}
}

// TestNeutralWaterTerritoriesNotRestricted verifies that neutral water can be freely traversed
func TestNeutralWaterTerritoriesNotRestricted(t *testing.T) {
	game := models.NewGame()

	// Create players
	usa := game.GetOrCreatePlayer("USA")
	usa.Side = "Allies"

	game.PlayerOrder = []string{"USA"}

	// Create water territories
	game.AddTerritory("Eastern USA Atlantic", models.Water, "USA", 0)
	game.AddTerritory("North Atlantic", models.Water, "Neutral", 0)
	game.AddTerritory("UK Sea Zone", models.Water, "USA", 0)
	game.ConnectTerritories("Eastern USA Atlantic", "North Atlantic")
	game.ConnectTerritories("North Atlantic", "Eastern USA Atlantic")
	game.ConnectTerritories("North Atlantic", "UK Sea Zone")
	game.ConnectTerritories("UK Sea Zone", "North Atlantic")

	// Add destroyer
	game.AddPieceTemplate("destroyer", models.Water, 2, 2, 2, 8)
	game.PlacePieces("Eastern USA Atlantic", "destroyer", 1)

	// Verify neutral water is NotNeutral type
	northAtlantic := game.Board["North Atlantic"]
	if northAtlantic.NeutralType != models.NotNeutral {
		t.Errorf("Neutral water should have NotNeutral type, got %v", northAtlantic.NeutralType)
	}

	// Create controller
	controller := NewGameController(game)
	controller.StartGame()
	game.CurrentPhase = models.NoncombatMovePhase

	// Plan a move through neutral water
	eusaTerritory := game.Board["Eastern USA Atlantic"]
	destroyerID := eusaTerritory.Pieces[0]

	err := controller.PlanMove(destroyerID, "Eastern USA Atlantic", "UK Sea Zone")
	if err != nil {
		t.Errorf("Should be able to move through neutral water: %v", err)
	}
}
