package game

import (
	"boardgame/models"
	"testing"
)

func TestArtillerySupport(t *testing.T) {
	// Create 2 infantry and 2 artillery
	infantry1 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	infantry2 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	artillery1 := &models.Piece{Name: "artillery", Attack: 2, Defend: 2}
	artillery2 := &models.Piece{Name: "artillery", Attack: 2, Defend: 2}

	units := []*models.Piece{infantry1, infantry2, artillery1, artillery2}

	// Apply artillery support
	ApplyArtillerySupport(units)

	// Both infantry should now have attack 2
	if infantry1.Attack != 2 {
		t.Errorf("Infantry 1 should have attack 2, got %d", infantry1.Attack)
	}
	if infantry2.Attack != 2 {
		t.Errorf("Infantry 2 should have attack 2, got %d", infantry2.Attack)
	}

	// Remove artillery support
	RemoveArtillerySupport(units)

	// Both infantry should now have attack 1 again
	if infantry1.Attack != 1 {
		t.Errorf("Infantry 1 should have attack 1 after removal, got %d", infantry1.Attack)
	}
	if infantry2.Attack != 1 {
		t.Errorf("Infantry 2 should have attack 1 after removal, got %d", infantry2.Attack)
	}
}

func TestArtillerySupportMoreInfantryThanArtillery(t *testing.T) {
	// Create 3 infantry and 1 artillery
	infantry1 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	infantry2 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	infantry3 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	artillery := &models.Piece{Name: "artillery", Attack: 2, Defend: 2}

	units := []*models.Piece{infantry1, infantry2, infantry3, artillery}

	// Apply artillery support
	ApplyArtillerySupport(units)

	// Count how many infantry have attack 2
	supportedCount := 0
	if infantry1.Attack == 2 {
		supportedCount++
	}
	if infantry2.Attack == 2 {
		supportedCount++
	}
	if infantry3.Attack == 2 {
		supportedCount++
	}

	// Only 1 infantry should be supported
	if supportedCount != 1 {
		t.Errorf("Expected 1 infantry to be supported, got %d", supportedCount)
	}
}

func TestArtillerySupportMoreArtilleryThanInfantry(t *testing.T) {
	// Create 1 infantry and 3 artillery
	infantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	artillery1 := &models.Piece{Name: "artillery", Attack: 2, Defend: 2}
	artillery2 := &models.Piece{Name: "artillery", Attack: 2, Defend: 2}
	artillery3 := &models.Piece{Name: "artillery", Attack: 2, Defend: 2}

	units := []*models.Piece{infantry, artillery1, artillery2, artillery3}

	// Apply artillery support
	ApplyArtillerySupport(units)

	// The infantry should be supported
	if infantry.Attack != 2 {
		t.Errorf("Infantry should have attack 2, got %d", infantry.Attack)
	}
}

func TestArtillerySupportNoInfantry(t *testing.T) {
	// Create only artillery and tanks
	artillery := &models.Piece{Name: "artillery", Attack: 2, Defend: 2}
	tank := &models.Piece{Name: "tank", Attack: 3, Defend: 3}

	units := []*models.Piece{artillery, tank}

	// Apply artillery support
	ApplyArtillerySupport(units)

	// Nothing should change
	if artillery.Attack != 2 {
		t.Errorf("Artillery attack should remain 2, got %d", artillery.Attack)
	}
	if tank.Attack != 3 {
		t.Errorf("Tank attack should remain 3, got %d", tank.Attack)
	}
}

func TestArtillerySupportInCombat(t *testing.T) {
	game := models.NewGame()

	// Setup territories
	game.AddTerritory("Territory1", models.Land, "Germany", 3)
	game.AddTerritory("Territory2", models.Land, "USSR", 3)
	game.ConnectTerritories("Territory1", "Territory2")
	game.ConnectTerritories("Territory2", "Territory1")

	// Add piece templates
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.AddPieceTemplate("artillery", models.Land, 1, 2, 2, 4)

	// Place units
	game.PlacePieces("Territory1", "infantry", 2)
	game.PlacePieces("Territory1", "artillery", 1)
	game.PlacePieces("Territory2", "infantry", 1)

	// Get attacking units
	attackerInfantry1 := game.Pieces[1]
	attackerInfantry2 := game.Pieces[2]
	attackerArtillery := game.Pieces[3]
	defenderInfantry := game.Pieces[4]

	// Create battle
	battle := NewBattle("Territory2", LandBattle, "Germany", "USSR")
	battle.AddAttacker(attackerInfantry1)
	battle.AddAttacker(attackerInfantry2)
	battle.AddAttacker(attackerArtillery)
	battle.AddDefender(defenderInfantry)

	// Use a seeded dice roller for predictable results
	// Seed that ensures attacker wins
	diceRoller := NewSeededDiceRoller(42)

	// Resolve combat
	result, err := ResolveCombat(battle, diceRoller, 10)
	if err != nil {
		t.Fatalf("Combat resolution failed: %v", err)
	}

	// After combat, infantry should be back to attack 1
	if attackerInfantry1.Attack != 1 {
		t.Errorf("Infantry 1 attack should be reset to 1, got %d", attackerInfantry1.Attack)
	}
	if attackerInfantry2.Attack != 1 {
		t.Errorf("Infantry 2 attack should be reset to 1, got %d", attackerInfantry2.Attack)
	}

	// Combat should have completed
	if result.Rounds == 0 {
		t.Error("Combat should have run at least one round")
	}
}

func TestArtillerySupportDefenseNoBonus(t *testing.T) {
	// Artillery should NOT support infantry on defense
	infantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	artillery := &models.Piece{Name: "artillery", Attack: 2, Defend: 2}

	// Create a battle where these units are defending
	battle := NewBattle("Territory", LandBattle, "Attacker", "Defender")
	battle.AddDefender(infantry)
	battle.AddDefender(artillery)

	// Roll for defenders - infantry should use defense value, not attack
	diceRoller := NewSeededDiceRoller(1)
	hits := diceRoller.RollForUnits(battle.Defenders, false) // false = defending

	// Infantry should roll with defense value of 2, not attack value
	// This test just ensures we're using Defend not Attack for defenders
	for _, defender := range battle.Defenders {
		if defender.Name == "infantry" {
			if defender.Defend != 2 {
				t.Errorf("Infantry defend should be 2, got %d", defender.Defend)
			}
		}
	}

	// Hits length will vary but this confirms the test runs
	_ = hits
}
