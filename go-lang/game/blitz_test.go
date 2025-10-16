package game

import (
	"boardgame/models"
	"testing"
)

// Test ValidateTankBlitz with valid blitz
func TestValidateTankBlitzValid(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("Germany")
	ussr := game.GetOrCreatePlayer("USSR")

	// Create territories: Germany -> Poland (empty, hostile) -> USSR
	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("Poland", models.Land, "USSR", 2) // Enemy territory
	game.AddTerritory("USSR", models.Land, "USSR", 8)

	// Connect territories
	game.ConnectTerritories("Germany", "Poland")
	game.ConnectTerritories("Poland", "USSR")

	// Poland is enemy but empty (no pieces) - can blitz through
	poland := game.Board["Poland"]
	poland.Owner = ussr
	poland.Pieces = []int{} // Empty!

	// Create a tank in Germany
	tank := &models.Piece{
		Name:     "tank",
		Movement: 2,
		Terrain:  models.Land,
	}
	game.Pieces[1] = tank
	game.Board["Germany"].Pieces = []int{1}

	// Validate blitz through Poland to USSR
	err := ValidateTankBlitz(game, 1, "Germany", "Poland", "USSR", "Germany")
	if err != nil {
		t.Errorf("Valid blitz should succeed: %v", err)
	}
}

// Test ValidateTankBlitz with enemy units in intermediate territory
func TestValidateTankBlitzEnemyUnits(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("Germany")
	ussr := game.GetOrCreatePlayer("USSR")

	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("Poland", models.Land, "USSR", 2)
	game.AddTerritory("USSR", models.Land, "USSR", 8)

	game.ConnectTerritories("Germany", "Poland")
	game.ConnectTerritories("Poland", "USSR")

	// Poland has enemy units - cannot blitz
	poland := game.Board["Poland"]
	poland.Owner = ussr
	poland.Pieces = []int{10} // Enemy infantry present

	tank := &models.Piece{
		Name:     "tank",
		Movement: 2,
		Terrain:  models.Land,
	}
	game.Pieces[1] = tank

	err := ValidateTankBlitz(game, 1, "Germany", "Poland", "USSR", "Germany")
	if err == nil {
		t.Error("Should fail when intermediate territory has enemy units")
	}
}

// Test ValidateTankBlitz with non-tank unit
func TestValidateTankBlitzNonTank(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("Germany")

	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("Poland", models.Land, "USSR", 2)
	game.AddTerritory("USSR", models.Land, "USSR", 8)

	game.ConnectTerritories("Germany", "Poland")
	game.ConnectTerritories("Poland", "USSR")

	game.Board["Poland"].Pieces = []int{} // Empty

	// Infantry cannot blitz
	infantry := &models.Piece{
		Name:     "infantry",
		Movement: 1,
		Terrain:  models.Land,
	}
	game.Pieces[1] = infantry

	err := ValidateTankBlitz(game, 1, "Germany", "Poland", "USSR", "Germany")
	if err == nil {
		t.Error("Should fail when unit is not a tank")
	}
}

// Test ValidateTankBlitz through friendly territory
func TestValidateTankBlitzFriendlyTerritory(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("Germany")

	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("Poland", models.Land, "Germany", 2) // Friendly!
	game.AddTerritory("Balkans", models.Land, "Italy", 3)

	game.ConnectTerritories("Germany", "Poland")
	game.ConnectTerritories("Poland", "Balkans")

	game.Board["Poland"].Pieces = []int{} // Empty

	tank := &models.Piece{
		Name:     "tank",
		Movement: 2,
		Terrain:  models.Land,
	}
	game.Pieces[1] = tank

	// Cannot blitz through friendly territory
	err := ValidateTankBlitz(game, 1, "Germany", "Poland", "Balkans", "Germany")
	if err == nil {
		t.Error("Should fail when trying to blitz through friendly territory")
	}
}

// Test ValidateTankBlitz with disconnected territories
func TestValidateTankBlitzDisconnected(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("Germany")

	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("Poland", models.Land, "USSR", 2)
	game.AddTerritory("USSR", models.Land, "USSR", 8)

	// Only connect Germany to Poland, not Poland to USSR
	game.ConnectTerritories("Germany", "Poland")

	game.Board["Poland"].Pieces = []int{} // Empty

	tank := &models.Piece{
		Name:     "tank",
		Movement: 2,
		Terrain:  models.Land,
	}
	game.Pieces[1] = tank

	err := ValidateTankBlitz(game, 1, "Germany", "Poland", "USSR", "Germany")
	if err == nil {
		t.Error("Should fail when territories are not connected")
	}
}

// Test ExecuteTankBlitz
func TestExecuteTankBlitz(t *testing.T) {
	game := models.NewGame()
	germany := game.GetOrCreatePlayer("Germany")
	ussr := game.GetOrCreatePlayer("USSR")

	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("Poland", models.Land, "USSR", 2)
	game.AddTerritory("USSR", models.Land, "USSR", 8)

	game.ConnectTerritories("Germany", "Poland")
	game.ConnectTerritories("Poland", "USSR")

	poland := game.Board["Poland"]
	poland.Owner = ussr
	poland.Pieces = []int{} // Empty - can blitz

	tank := &models.Piece{
		Name:     "tank",
		Movement: 2,
		Terrain:  models.Land,
	}
	game.Pieces[1] = tank

	blitzMove := &BlitzMove{
		PieceID:          1,
		StartTerritory:   "Germany",
		IntermediateStop: "Poland",
		FinalDestination: "USSR",
	}

	err := ExecuteTankBlitz(game, blitzMove, "Germany")
	if err != nil {
		t.Fatalf("ExecuteTankBlitz failed: %v", err)
	}

	// Verify Poland was captured
	if poland.Owner != germany {
		t.Error("Poland should be captured by Germany during blitz")
	}

	// Verify movement was tracked
	if blitzMove.MovementUsed != 2 {
		t.Errorf("Expected movement used 2, got %d", blitzMove.MovementUsed)
	}

	t.Log("Tank successfully blitzed through Poland to USSR")
}

// Test CanBlitz helper
func TestCanBlitz(t *testing.T) {
	tank := &models.Piece{Name: "tank"}
	armor := &models.Piece{Name: "armor"}
	infantry := &models.Piece{Name: "infantry"}
	fighter := &models.Piece{Name: "fighter"}

	if !CanBlitz(tank) {
		t.Error("Tank should be able to blitz")
	}

	if !CanBlitz(armor) {
		t.Error("Armor should be able to blitz")
	}

	if CanBlitz(infantry) {
		t.Error("Infantry should not be able to blitz")
	}

	if CanBlitz(fighter) {
		t.Error("Fighter should not be able to blitz")
	}
}

// Test GetBlitzCapableUnits
func TestGetBlitzCapableUnits(t *testing.T) {
	tank := &models.Piece{Name: "tank"}
	armor := &models.Piece{Name: "armor"}
	infantry1 := &models.Piece{Name: "infantry"}
	infantry2 := &models.Piece{Name: "infantry"}
	artillery := &models.Piece{Name: "artillery"}

	units := []*models.Piece{tank, infantry1, armor, infantry2, artillery}

	blitzers := GetBlitzCapableUnits(units)

	if len(blitzers) != 2 {
		t.Errorf("Expected 2 blitz-capable units, got %d", len(blitzers))
	}

	// Verify it's the tank and armor
	foundTank := false
	foundArmor := false
	for _, unit := range blitzers {
		if unit.Name == "tank" {
			foundTank = true
		}
		if unit.Name == "armor" {
			foundArmor = true
		}
	}

	if !foundTank || !foundArmor {
		t.Error("Should find tank and armor as blitz-capable units")
	}
}

// Test ValidateRetreat with valid retreat
func TestValidateRetreatValid(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("Germany")

	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("Poland", models.Land, "USSR", 2)

	game.ConnectTerritories("Germany", "Poland")
	game.ConnectTerritories("Poland", "Germany")

	// Germany attacks Poland, can retreat back to Germany
	err := ValidateRetreat(game, "Poland", "Germany", "Germany", false, 1)
	if err != nil {
		t.Errorf("Valid retreat should succeed: %v", err)
	}
}

// Test ValidateRetreat to enemy territory
func TestValidateRetreatEnemyTerritory(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("Germany")

	game.AddTerritory("Poland", models.Land, "USSR", 2)
	game.AddTerritory("USSR", models.Land, "USSR", 8)

	game.ConnectTerritories("Poland", "USSR")

	// Cannot retreat to enemy territory
	err := ValidateRetreat(game, "Poland", "USSR", "Germany", false, 1)
	if err == nil {
		t.Error("Should fail when retreating to enemy territory")
	}
}

// Test ValidateRetreat from amphibious assault
func TestValidateRetreatAmphibious(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("USA")

	game.AddTerritory("SeaZone", models.Water, "USA", 0)
	game.AddTerritory("Normandy", models.Land, "Germany", 3)

	game.ConnectTerritories("SeaZone", "Normandy")

	// Cannot retreat from amphibious assault
	err := ValidateRetreat(game, "Normandy", "SeaZone", "USA", true, 1)
	if err == nil {
		t.Error("Should fail when trying to retreat from amphibious assault")
	}
}

// Test ValidateRetreat to non-adjacent territory
func TestValidateRetreatNonAdjacent(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("Germany")

	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("Poland", models.Land, "USSR", 2)
	game.AddTerritory("Balkans", models.Land, "Germany", 3)

	game.ConnectTerritories("Germany", "Poland")
	// Balkans not connected to Poland

	err := ValidateRetreat(game, "Poland", "Balkans", "Germany", false, 1)
	if err == nil {
		t.Error("Should fail when retreat destination is not adjacent")
	}
}

// Test ExecuteRetreat
func TestExecuteRetreat(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("Germany")

	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("Poland", models.Land, "USSR", 2)

	game.ConnectTerritories("Germany", "Poland")
	game.ConnectTerritories("Poland", "Germany")

	// Create a battle
	battle := NewBattle("Poland", LandBattle, "Germany", "USSR")
	battle.Round = 2

	infantry := &models.Piece{Name: "infantry", Terrain: models.Land}
	tank := &models.Piece{Name: "tank", Terrain: models.Land}
	battle.Attackers = []*models.Piece{infantry, tank}

	// Execute retreat
	retreatMove, err := ExecuteRetreat(game, battle, "Germany", "Germany")
	if err != nil {
		t.Fatalf("ExecuteRetreat failed: %v", err)
	}

	if retreatMove.BattleLocation != "Poland" {
		t.Errorf("Expected battle location Poland, got %s", retreatMove.BattleLocation)
	}

	if retreatMove.RetreatDestination != "Germany" {
		t.Errorf("Expected retreat destination Germany, got %s", retreatMove.RetreatDestination)
	}

	if len(retreatMove.RetreatingUnits) != 2 {
		t.Errorf("Expected 2 retreating units, got %d", len(retreatMove.RetreatingUnits))
	}

	if retreatMove.Round != 2 {
		t.Errorf("Expected retreat on round 2, got %d", retreatMove.Round)
	}

	t.Log("Successfully executed retreat from Poland to Germany")
}

// Test tank blitz integration scenario
func TestTankBlitzScenario(t *testing.T) {
	game := models.NewGame()
	game.GetOrCreatePlayer("Germany")
	game.GetOrCreatePlayer("USSR")

	// Setup: Germany -> Poland (empty hostile) -> Balkans (empty hostile) -> USSR (has units)
	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("Poland", models.Land, "USSR", 2)
	game.AddTerritory("Balkans", models.Land, "Italy", 3)
	game.AddTerritory("USSR", models.Land, "USSR", 8)

	game.ConnectTerritories("Germany", "Poland")
	game.ConnectTerritories("Poland", "Balkans")
	game.ConnectTerritories("Balkans", "USSR")

	// Poland and Balkans are empty
	game.Board["Poland"].Pieces = []int{}
	game.Board["Balkans"].Pieces = []int{}

	// USSR has defenders
	game.Board["USSR"].Pieces = []int{10, 11}

	t.Log("=== Tank Blitz Scenario ===")
	t.Log("Germany has tank in Germany")
	t.Log("Poland is empty hostile (USSR) - can blitz through")
	t.Log("Balkans is empty hostile (Italy) - can blitz through")
	t.Log("USSR has defenders - must stop and fight")

	tank := &models.Piece{
		Name:     "tank",
		Movement: 2,
		Terrain:  models.Land,
	}
	game.Pieces[1] = tank

	// Blitz through Poland
	blitz1 := &BlitzMove{
		PieceID:          1,
		StartTerritory:   "Germany",
		IntermediateStop: "Poland",
		FinalDestination: "Balkans",
	}

	err := ExecuteTankBlitz(game, blitz1, "Germany")
	if err != nil {
		t.Fatalf("First blitz failed: %v", err)
	}

	t.Log("✓ Tank blitzed through Poland to Balkans (captured Poland)")

	// Cannot blitz through Balkans to USSR because USSR has units
	// (Blitzing is THROUGH empty territories)
	// For this test, let's verify we captured Poland
	poland := game.Board["Poland"]
	if poland.Owner.Name != "Germany" {
		t.Error("Poland should be captured by Germany")
	}

	t.Log("=== Scenario Complete ===")
}

// Test retreat during multi-round combat
func TestRetreatDuringCombat(t *testing.T) {
	game := models.NewGame()
	roller := NewSeededDiceRoller(600)

	game.GetOrCreatePlayer("Germany")
	game.GetOrCreatePlayer("USSR")

	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("Poland", models.Land, "USSR", 2)

	game.ConnectTerritories("Germany", "Poland")
	game.ConnectTerritories("Poland", "Germany")

	t.Log("=== Combat with Retreat ===")

	// Create battle
	battle := NewBattle("Poland", LandBattle, "Germany", "USSR")

	// Attackers
	infantry1 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	infantry2 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	battle.Attackers = []*models.Piece{infantry1, infantry2}

	// Defenders
	defInf1 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	defInf2 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	defInf3 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	battle.Defenders = []*models.Piece{defInf1, defInf2, defInf3}

	t.Logf("Initial forces - Attackers: %d, Defenders: %d", len(battle.Attackers), len(battle.Defenders))

	// Fight one round
	attackerHits, defenderHits, _ := roller.CombatRound(battle)

	t.Logf("Round 1 - Attacker hits: %d, Defender hits: %d", len(attackerHits), len(defenderHits))

	// Attacker decides to retreat after round 1
	retreatMove, err := ExecuteRetreat(game, battle, "Germany", "Germany")
	if err != nil {
		t.Fatalf("Retreat failed: %v", err)
	}

	t.Logf("Attackers retreated to %s with %d units", retreatMove.RetreatDestination, len(retreatMove.RetreatingUnits))
	t.Log("=== Combat Ended via Retreat ===")
}
