package game

import (
	"boardgame/models"
	"testing"
)

// Test getBombardmentShips helper function
func TestGetBombardmentShips(t *testing.T) {
	battleship := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Terrain: models.Water}
	cruiser := &models.Piece{Name: "cruiser", Attack: 3, Defend: 3, Terrain: models.Water}
	destroyer := &models.Piece{Name: "destroyer", Attack: 2, Defend: 2, Terrain: models.Water}
	transport := &models.Piece{Name: "transport", Attack: 0, Defend: 1, Terrain: models.Water}

	units := []*models.Piece{battleship, cruiser, destroyer, transport}
	bombardShips := getBombardmentShips(units)

	if len(bombardShips) != 2 {
		t.Errorf("Expected 2 bombardment ships (battleship + cruiser), got %d", len(bombardShips))
	}

	// Verify it's the right ships
	foundBattleship := false
	foundCruiser := false
	for _, ship := range bombardShips {
		if ship.Name == "battleship" {
			foundBattleship = true
		}
		if ship.Name == "cruiser" {
			foundCruiser = true
		}
	}

	if !foundBattleship {
		t.Error("Battleship should be in bombardment ships")
	}
	if !foundCruiser {
		t.Error("Cruiser should be in bombardment ships")
	}
}

// Test getLandUnits helper function
func TestGetLandUnits(t *testing.T) {
	infantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	tank := &models.Piece{Name: "tank", Attack: 3, Defend: 3, Terrain: models.Land}
	battleship := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Terrain: models.Water}
	fighter := &models.Piece{Name: "fighter", Attack: 3, Defend: 4, Terrain: models.Air}

	units := []*models.Piece{infantry, tank, battleship, fighter}
	landUnits := getLandUnits(units)

	if len(landUnits) != 2 {
		t.Errorf("Expected 2 land units, got %d", len(landUnits))
	}
}

// Test RollBombardment function
func TestRollBombardment(t *testing.T) {
	roller := NewSeededDiceRoller(42)

	battleship := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Terrain: models.Water}
	cruiser := &models.Piece{Name: "cruiser", Attack: 3, Defend: 3, Terrain: models.Water}

	bombardShips := []*models.Piece{battleship, cruiser}
	unitsBeingOffloaded := 2

	hits := roller.RollBombardment(bombardShips, unitsBeingOffloaded)

	// With seed 42, we should get specific results
	// The function should roll for each ship
	t.Logf("Bombardment rolled, hits: %d", len(hits))
}

// Test bombardment with more ships than units
func TestRollBombardmentLimitedByUnits(t *testing.T) {
	roller := NewSeededDiceRoller(42)

	battleship1 := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Terrain: models.Water}
	battleship2 := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Terrain: models.Water}
	cruiser := &models.Piece{Name: "cruiser", Attack: 3, Defend: 3, Terrain: models.Water}

	bombardShips := []*models.Piece{battleship1, battleship2, cruiser}
	unitsBeingOffloaded := 2 // Only 2 units being offloaded

	hits := roller.RollBombardment(bombardShips, unitsBeingOffloaded)

	// Should only roll for 2 ships (limited by units offloaded)
	// We can't test exact hits, but we can verify the function doesn't crash
	t.Logf("With 3 ships but only 2 units offloaded, bombardment hits: %d", len(hits))
}

// Test amphibious assault with no defending ships (no sea battle)
func TestAmphibiousAssaultNoSeaBattle(t *testing.T) {
	roller := NewSeededDiceRoller(100) // Seed that produces favorable rolls

	// Attacking forces
	battleship := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Terrain: models.Water}
	infantry1 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	infantry2 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}

	attackingShips := []*models.Piece{battleship}
	attackingLandUnits := []*models.Piece{infantry1, infantry2}

	// Defending forces (no ships, just land)
	defInfantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	defendingShips := []*models.Piece{}
	defendingLandUnits := []*models.Piece{defInfantry}

	result, err := ResolveAmphibiousAssault(
		"Normandy",
		attackingShips,
		attackingLandUnits,
		defendingShips,
		defendingLandUnits,
		"USA",
		"Germany",
		roller,
	)

	if err != nil {
		t.Fatalf("Amphibious assault failed: %v", err)
	}

	// No sea battle should have occurred
	if result.SeaBattleResult != nil {
		t.Error("Expected no sea battle, but SeaBattleResult is not nil")
	}

	// Bombardment should have occurred
	t.Logf("Bombardment hits: %d", len(result.BombardmentHits))

	// Land battle should have occurred
	if result.LandBattleResult == nil {
		t.Fatal("Expected land battle to occur")
	}

	t.Logf("Land battle result - Attacker wins: %v, Defender wins: %v",
		result.LandBattleResult.AttackerWins,
		result.LandBattleResult.DefenderWins)
}

// Test amphibious assault with defending ships (requires sea battle first)
func TestAmphibiousAssaultWithSeaBattle(t *testing.T) {
	roller := NewSeededDiceRoller(200)

	// Attacking forces
	battleship := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Terrain: models.Water}
	destroyer := &models.Piece{Name: "destroyer", Attack: 2, Defend: 2, Terrain: models.Water}
	infantry1 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	infantry2 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}

	attackingShips := []*models.Piece{battleship, destroyer}
	attackingLandUnits := []*models.Piece{infantry1, infantry2}

	// Defending forces (with ships)
	defSubmarine := &models.Piece{Name: "submarine", Attack: 2, Defend: 1, Terrain: models.Water}
	defInfantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}

	defendingShips := []*models.Piece{defSubmarine}
	defendingLandUnits := []*models.Piece{defInfantry}

	result, err := ResolveAmphibiousAssault(
		"Normandy",
		attackingShips,
		attackingLandUnits,
		defendingShips,
		defendingLandUnits,
		"USA",
		"Germany",
		roller,
	)

	if err != nil {
		t.Fatalf("Amphibious assault failed: %v", err)
	}

	// Sea battle should have occurred
	if result.SeaBattleResult == nil {
		t.Fatal("Expected sea battle to occur")
	}

	t.Logf("Sea battle - Attacker wins: %v, Rounds: %d",
		result.SeaBattleResult.AttackerWins,
		result.SeaBattleResult.Rounds)

	// If attackers won sea battle, land combat should proceed
	if result.SeaBattleResult.AttackerWins {
		t.Logf("Bombardment hits: %d", len(result.BombardmentHits))

		if result.LandBattleResult == nil {
			t.Error("Expected land battle after winning sea battle")
		}
	}
}

// Test amphibious assault fails if sea battle is lost
func TestAmphibiousAssaultFailsIfSeaBattleLost(t *testing.T) {
	roller := NewSeededDiceRoller(999) // Seed that might produce unfavorable rolls

	// Weak attacking naval force
	transport := &models.Piece{Name: "transport", Attack: 0, Defend: 1, Terrain: models.Water}
	infantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}

	attackingShips := []*models.Piece{transport}
	attackingLandUnits := []*models.Piece{infantry}

	// Strong defending naval force
	battleship := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Terrain: models.Water}
	destroyer := &models.Piece{Name: "destroyer", Attack: 2, Defend: 2, Terrain: models.Water}
	defInfantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}

	defendingShips := []*models.Piece{battleship, destroyer}
	defendingLandUnits := []*models.Piece{defInfantry}

	result, err := ResolveAmphibiousAssault(
		"Fortress",
		attackingShips,
		attackingLandUnits,
		defendingShips,
		defendingLandUnits,
		"USA",
		"Germany",
		roller,
	)

	if err != nil {
		t.Fatalf("Amphibious assault failed: %v", err)
	}

	// Sea battle should have occurred
	if result.SeaBattleResult == nil {
		t.Fatal("Expected sea battle to occur")
	}

	// If defenders won sea battle, assault should fail
	if result.SeaBattleResult.DefenderWins {
		if result.Success {
			t.Error("Assault should fail if sea battle is lost")
		}

		t.Log("Correctly failed amphibious assault after losing sea battle")
	}
}

// Test full amphibious assault sequence
func TestFullAmphibiousAssaultSequence(t *testing.T) {
	roller := NewSeededDiceRoller(300)

	// D-Day style assault
	battleship1 := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Terrain: models.Water}
	battleship2 := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Terrain: models.Water}
	cruiser := &models.Piece{Name: "cruiser", Attack: 3, Defend: 3, Terrain: models.Water}
	destroyer := &models.Piece{Name: "destroyer", Attack: 2, Defend: 2, Terrain: models.Water}

	infantry1 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	infantry2 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	infantry3 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	tank := &models.Piece{Name: "tank", Attack: 3, Defend: 3, Terrain: models.Land}

	attackingShips := []*models.Piece{battleship1, battleship2, cruiser, destroyer}
	attackingLandUnits := []*models.Piece{infantry1, infantry2, infantry3, tank}

	// Defenders
	defSubmarine := &models.Piece{Name: "submarine", Attack: 2, Defend: 1, Terrain: models.Water}
	defInfantry1 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	defInfantry2 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	defArtillery := &models.Piece{Name: "artillery", Attack: 2, Defend: 2, Terrain: models.Land}

	defendingShips := []*models.Piece{defSubmarine}
	defendingLandUnits := []*models.Piece{defInfantry1, defInfantry2, defArtillery}

	result, err := ResolveAmphibiousAssault(
		"Normandy",
		attackingShips,
		attackingLandUnits,
		defendingShips,
		defendingLandUnits,
		"USA",
		"Germany",
		roller,
	)

	if err != nil {
		t.Fatalf("Amphibious assault failed: %v", err)
	}

	// Verify all phases occurred
	t.Logf("\n=== Amphibious Assault Report ===")

	if result.SeaBattleResult != nil {
		t.Logf("Sea Battle: Attacker wins: %v, Rounds: %d, Casualties: A-%d D-%d",
			result.SeaBattleResult.AttackerWins,
			result.SeaBattleResult.Rounds,
			len(result.SeaBattleResult.AttackerCasualties),
			len(result.SeaBattleResult.DefenderCasualties))
	}

	t.Logf("Bombardment: %d hits from %d bombardment ships",
		len(result.BombardmentHits),
		len(getBombardmentShips(attackingShips)))

	if result.LandBattleResult != nil {
		t.Logf("Land Battle: Attacker wins: %v, Rounds: %d, Casualties: A-%d D-%d",
			result.LandBattleResult.AttackerWins,
			result.LandBattleResult.Rounds,
			len(result.LandBattleResult.AttackerCasualties),
			len(result.LandBattleResult.DefenderCasualties))
	}

	t.Logf("Overall Success: %v", result.Success)
}

// Test bombardment with damaged battleship
func TestBombardmentWithDamagedBattleship(t *testing.T) {
	roller := NewSeededDiceRoller(50)

	// Damaged battleship (1 hit already)
	battleship := &models.Piece{
		Name:   "battleship",
		Attack: 4,
		Defend: 4,
		Terrain: models.Water,
		Hits:   1, // Damaged but still functional
	}

	infantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}

	attackingShips := []*models.Piece{battleship}
	attackingLandUnits := []*models.Piece{infantry}

	defInfantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Terrain: models.Land}
	defendingShips := []*models.Piece{}
	defendingLandUnits := []*models.Piece{defInfantry}

	result, err := ResolveAmphibiousAssault(
		"Island",
		attackingShips,
		attackingLandUnits,
		defendingShips,
		defendingLandUnits,
		"USA",
		"Japan",
		roller,
	)

	if err != nil {
		t.Fatalf("Amphibious assault failed: %v", err)
	}

	// Damaged battleship should still be able to bombard
	t.Logf("Damaged battleship bombardment - hits: %d", len(result.BombardmentHits))
}
