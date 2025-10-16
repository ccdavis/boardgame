package game

import (
	"boardgame/models"
	"testing"
)

// Test ValidateFighterLaunch function
func TestValidateFighterLaunch(t *testing.T) {
	game := models.NewGame()
	game.AddTerritory("SeaZone1", models.Water, "USA", 0)

	// Add carrier template and fighter template
	game.AddPieceTemplate("carrier", models.Water, 2, 1, 2, 14)
	game.SetContainerCapacity("carrier", 2, []string{"fighter"})
	game.AddPieceTemplate("fighter", models.Air, 4, 3, 4, 10)

	// Create a carrier
	carrierPiece := &models.Piece{
		Name:     "carrier",
		Terrain:  models.Water,
		Movement: 2,
		Attack:   1,
		Defend:   2,
		Cost:     14,
		Capacity: 2,
		CanCarry: []string{"fighter"},
		Holding:  make([]int, 0),
	}
	game.Pieces[1] = carrierPiece
	game.Board["SeaZone1"].Pieces = append(game.Board["SeaZone1"].Pieces, 1)

	// Create a fighter on the carrier
	fighterPiece := &models.Piece{
		Name:     "fighter",
		Terrain:  models.Air,
		Movement: 4,
		Attack:   3,
		Defend:   4,
		Cost:     10,
	}
	game.Pieces[2] = fighterPiece
	carrierPiece.Holding = append(carrierPiece.Holding, 2)

	// Test valid launch
	err := ValidateFighterLaunch(game, 1, 2, "SeaZone1")
	if err != nil {
		t.Errorf("Valid fighter launch should succeed: %v", err)
	}

	// Test invalid - fighter not on carrier
	fighterPiece2 := &models.Piece{Name: "fighter", Terrain: models.Air}
	game.Pieces[3] = fighterPiece2

	err = ValidateFighterLaunch(game, 1, 3, "SeaZone1")
	if err == nil {
		t.Error("Should fail when fighter is not on carrier")
	}

	// Test invalid - carrier doesn't exist
	err = ValidateFighterLaunch(game, 999, 2, "SeaZone1")
	if err == nil {
		t.Error("Should fail when carrier doesn't exist")
	}
}

// Test LaunchFighter function
func TestLaunchFighter(t *testing.T) {
	game := models.NewGame()
	game.AddTerritory("SeaZone1", models.Water, "USA", 0)

	// Create carrier with fighter
	carrierPiece := &models.Piece{
		Name:     "carrier",
		Capacity: 2,
		Holding:  []int{2}, // Fighter ID 2 is on board
	}
	game.Pieces[1] = carrierPiece

	fighterPiece := &models.Piece{
		Name:    "fighter",
		Terrain: models.Air,
	}
	game.Pieces[2] = fighterPiece

	game.Board["SeaZone1"].Pieces = []int{1, 2}

	// Launch the fighter
	record, err := LaunchFighter(game, 1, 2, "SeaZone1")
	if err != nil {
		t.Fatalf("LaunchFighter failed: %v", err)
	}

	if record.CarrierPieceID != 1 {
		t.Errorf("Expected carrier ID 1, got %d", record.CarrierPieceID)
	}

	if record.FighterPieceID != 2 {
		t.Errorf("Expected fighter ID 2, got %d", record.FighterPieceID)
	}

	// Verify fighter was removed from carrier's Holding
	if len(carrierPiece.Holding) != 0 {
		t.Errorf("Expected carrier Holding to be empty, got %d items", len(carrierPiece.Holding))
	}

	// Verify fighter is in territory
	fighterInTerritory := false
	for _, pieceID := range game.Board["SeaZone1"].Pieces {
		if pieceID == 2 {
			fighterInTerritory = true
			break
		}
	}
	if !fighterInTerritory {
		t.Error("Fighter should be in territory after launch")
	}
}

// Test LandFighterOnCarrier function
func TestLandFighterOnCarrier(t *testing.T) {
	game := models.NewGame()
	game.AddTerritory("SeaZone1", models.Water, "USA", 0)

	// Create carrier
	carrierPiece := &models.Piece{
		Name:     "carrier",
		Capacity: 2,
		Holding:  make([]int, 0),
	}
	game.Pieces[1] = carrierPiece

	// Create fighter
	fighterPiece := &models.Piece{
		Name:    "fighter",
		Terrain: models.Air,
	}
	game.Pieces[2] = fighterPiece

	// Both in same territory
	game.Board["SeaZone1"].Pieces = []int{1, 2}

	// Land fighter on carrier
	err := LandFighterOnCarrier(game, 2, 1, "SeaZone1")
	if err != nil {
		t.Fatalf("LandFighterOnCarrier failed: %v", err)
	}

	// Verify fighter was added to carrier's Holding
	if len(carrierPiece.Holding) != 1 {
		t.Errorf("Expected carrier Holding to have 1 item, got %d", len(carrierPiece.Holding))
	}

	if carrierPiece.Holding[0] != 2 {
		t.Errorf("Expected fighter ID 2 in Holding, got %d", carrierPiece.Holding[0])
	}
}

// Test LandFighterOnCarrier at capacity
func TestLandFighterOnCarrierAtCapacity(t *testing.T) {
	game := models.NewGame()
	game.AddTerritory("SeaZone1", models.Water, "USA", 0)

	// Create carrier at full capacity
	carrierPiece := &models.Piece{
		Name:     "carrier",
		Capacity: 2,
		Holding:  []int{3, 4}, // Already has 2 fighters
	}
	game.Pieces[1] = carrierPiece

	// Create another fighter
	fighterPiece := &models.Piece{
		Name:    "fighter",
		Terrain: models.Air,
	}
	game.Pieces[2] = fighterPiece

	game.Board["SeaZone1"].Pieces = []int{1, 2}

	// Try to land fighter on full carrier
	err := LandFighterOnCarrier(game, 2, 1, "SeaZone1")
	if err == nil {
		t.Error("Should fail when carrier is at full capacity")
	}
}

// Test FindAvailableLandingZones
func TestFindAvailableLandingZones(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("USA")
	game.GetOrCreatePlayer("Japan")

	// Create a network of territories
	game.AddTerritory("SeaZone1", models.Water, "USA", 0)
	game.AddTerritory("SeaZone2", models.Water, "USA", 0)
	game.AddTerritory("Island", models.Land, "USA", 2)
	game.AddTerritory("EnemyTerritory", models.Land, "Japan", 3)

	// Connect territories
	game.ConnectTerritories("SeaZone1", "SeaZone2")
	game.ConnectTerritories("SeaZone1", "Island")
	game.ConnectTerritories("SeaZone2", "Island")
	game.ConnectTerritories("SeaZone2", "EnemyTerritory")

	// Create a fighter
	fighterPiece := &models.Piece{
		Name:     "fighter",
		Terrain:  models.Air,
		Movement: 4,
	}
	game.Pieces[1] = fighterPiece

	// Create a carrier in SeaZone2
	carrierPiece := &models.Piece{
		Name:     "carrier",
		Capacity: 2,
		Holding:  make([]int, 0),
	}
	game.Pieces[2] = carrierPiece
	game.Board["SeaZone2"].Pieces = append(game.Board["SeaZone2"].Pieces, 2)

	// Find landing zones from SeaZone1 with 2 movement remaining
	territories, carriers, err := FindAvailableLandingZones(game, 1, "SeaZone1", 2, "USA")
	if err != nil {
		t.Fatalf("FindAvailableLandingZones failed: %v", err)
	}

	t.Logf("Found %d territories and %d carriers as landing zones", len(territories), len(carriers))

	// Should include friendly territories within range
	hasSeaZone1 := false
	hasSeaZone2 := false
	hasIsland := false
	hasEnemyTerritory := false

	for _, terr := range territories {
		if terr == "SeaZone1" {
			hasSeaZone1 = true
		}
		if terr == "SeaZone2" {
			hasSeaZone2 = true
		}
		if terr == "Island" {
			hasIsland = true
		}
		if terr == "EnemyTerritory" {
			hasEnemyTerritory = true
		}
	}

	if !hasSeaZone1 {
		t.Error("Should include SeaZone1 (starting position)")
	}

	if !hasSeaZone2 {
		t.Error("Should include SeaZone2 (1 move away)")
	}

	if !hasIsland {
		t.Error("Should include Island (1 move away)")
	}

	if hasEnemyTerritory {
		t.Error("Should not include enemy territory")
	}

	// Should find carrier in SeaZone2
	hasCarrier := false
	for _, carrierID := range carriers {
		if carrierID == 2 {
			hasCarrier = true
		}
	}

	if !hasCarrier {
		t.Error("Should find carrier in SeaZone2")
	}
}

// Test ValidateAirUnitLanding
func TestValidateAirUnitLanding(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("USA")

	game.AddTerritory("SeaZone1", models.Water, "USA", 0)
	game.AddTerritory("SeaZone2", models.Water, "USA", 0)
	game.AddTerritory("Island", models.Land, "USA", 2)

	game.ConnectTerritories("SeaZone1", "SeaZone2")
	game.ConnectTerritories("SeaZone2", "Island")

	fighterPiece := &models.Piece{
		Name:     "fighter",
		Terrain:  models.Air,
		Movement: 4,
	}
	game.Pieces[1] = fighterPiece

	// Test valid landing - within range, friendly territory
	err := ValidateAirUnitLanding(game, 1, "SeaZone1", "SeaZone2", 1, "USA")
	if err != nil {
		t.Errorf("Valid landing should succeed: %v", err)
	}

	// Test valid landing - 2 moves to Island
	err = ValidateAirUnitLanding(game, 1, "SeaZone1", "Island", 2, "USA")
	if err != nil {
		t.Errorf("Valid landing at Island should succeed: %v", err)
	}
}

// Test carrier operations integration
func TestCarrierOperationsIntegration(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("USA")

	// Setup territories
	game.AddTerritory("SeaZone1", models.Water, "USA", 0)
	game.AddTerritory("SeaZone2", models.Water, "USA", 0)
	game.AddTerritory("EnemySeaZone", models.Water, "Japan", 0)
	game.AddTerritory("FriendlyIsland", models.Land, "USA", 3)

	game.ConnectTerritories("SeaZone1", "SeaZone2")
	game.ConnectTerritories("SeaZone2", "EnemySeaZone")
	game.ConnectTerritories("SeaZone1", "FriendlyIsland")

	// Create carrier with 2 fighters
	carrierPiece := &models.Piece{
		Name:     "carrier",
		Capacity: 2,
		Movement: 2,
		Holding:  []int{2, 3}, // Two fighters on board
	}
	game.Pieces[1] = carrierPiece

	fighter1 := &models.Piece{Name: "fighter", Terrain: models.Air, Movement: 4}
	fighter2 := &models.Piece{Name: "fighter", Terrain: models.Air, Movement: 4}
	game.Pieces[2] = fighter1
	game.Pieces[3] = fighter2

	game.Board["SeaZone1"].Pieces = []int{1, 2, 3}

	t.Log("=== Carrier Operations Test ===")

	// Step 1: Launch first fighter
	record1, err := LaunchFighter(game, 1, 2, "SeaZone1")
	if err != nil {
		t.Fatalf("Failed to launch fighter 1: %v", err)
	}
	t.Logf("Launched fighter %d from carrier %d", record1.FighterPieceID, record1.CarrierPieceID)

	// Step 2: Launch second fighter
	record2, err := LaunchFighter(game, 1, 3, "SeaZone1")
	if err != nil {
		t.Fatalf("Failed to launch fighter 2: %v", err)
	}
	t.Logf("Launched fighter %d from carrier %d", record2.FighterPieceID, record2.CarrierPieceID)

	// Carrier should now be empty
	if len(carrierPiece.Holding) != 0 {
		t.Errorf("Carrier should be empty after launching both fighters, got %d", len(carrierPiece.Holding))
	}

	// Step 3: Fighters can now move independently
	// (Movement simulation would go here - not testing actual movement)

	// Step 4: Land one fighter back on carrier
	err = LandFighterOnCarrier(game, 2, 1, "SeaZone1")
	if err != nil {
		t.Fatalf("Failed to land fighter on carrier: %v", err)
	}
	t.Log("Fighter 2 landed back on carrier")

	if len(carrierPiece.Holding) != 1 {
		t.Errorf("Carrier should have 1 fighter after landing, got %d", len(carrierPiece.Holding))
	}

	// Step 5: Validate that second fighter could reach friendly island
	err = ValidateAirUnitLanding(game, 3, "SeaZone1", "FriendlyIsland", 1, "USA")
	if err != nil {
		t.Errorf("Fighter should be able to land on friendly island: %v", err)
	}

	t.Log("=== Carrier Operations Complete ===")
}

// Test fighter launched from carrier participating in combat
func TestFighterLaunchedFromCarrierCombat(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("USA")
	game.GetOrCreatePlayer("Japan")

	game.AddTerritory("SeaZone1", models.Water, "USA", 0)
	game.AddTerritory("EnemySeaZone", models.Water, "Japan", 0)

	game.ConnectTerritories("SeaZone1", "EnemySeaZone")

	// Carrier with fighter
	carrierPiece := &models.Piece{
		Name:     "carrier",
		Capacity: 2,
		Holding:  []int{2},
	}
	game.Pieces[1] = carrierPiece

	fighterPiece := &models.Piece{
		Name:     "fighter",
		Terrain:  models.Air,
		Movement: 4,
		Attack:   3,
		Defend:   4,
	}
	game.Pieces[2] = fighterPiece

	game.Board["SeaZone1"].Pieces = []int{1, 2}

	// Launch fighter
	_, err := LaunchFighter(game, 1, 2, "SeaZone1")
	if err != nil {
		t.Fatalf("Failed to launch fighter: %v", err)
	}

	// Fighter is now available for combat
	// Verify fighter can be used in battle
	roller := NewSeededDiceRoller(100)
	battle := NewBattle("EnemySeaZone", SeaBattle, "USA", "Japan")

	// Fighter participates in battle
	battle.AddAttacker(fighterPiece)

	// Enemy submarine
	enemySub := &models.Piece{
		Name:    "submarine",
		Terrain: models.Water,
		Attack:  2,
		Defend:  1,
	}
	battle.AddDefender(enemySub)

	result, err := ResolveCombat(battle, roller, 10)
	if err != nil {
		t.Fatalf("Combat failed: %v", err)
	}

	t.Logf("Combat result - Attacker wins: %v, Rounds: %d", result.AttackerWins, result.Rounds)
}
