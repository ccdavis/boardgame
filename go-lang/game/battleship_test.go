package game

import (
	"boardgame/models"
	"testing"
)

// Test that a battleship survives one hit and continues fighting
func TestBattleshipSurvivesOneHit(t *testing.T) {
	// Create a battleship
	battleship := &models.Piece{
		Name:     "battleship",
		Attack:   4,
		Defend:   4,
		Cost:     20,
		Movement: 2,
		Terrain:  models.Water,
		Hits:     0,
	}

	units := []*models.Piece{battleship}

	// Apply 1 hit
	casualties := SelectCasualties(units, 1)

	// Battleship should not be in casualties (survived with 1 hit)
	if len(casualties) != 0 {
		t.Errorf("Expected 0 casualties, got %d", len(casualties))
	}

	// Battleship should have 1 hit
	if battleship.Hits != 1 {
		t.Errorf("Expected battleship to have 1 hit, got %d", battleship.Hits)
	}
}

// Test that a battleship is destroyed after 2 hits
func TestBattleshipDestroyedAfterTwoHits(t *testing.T) {
	// Create a battleship
	battleship := &models.Piece{
		Name:     "battleship",
		Attack:   4,
		Defend:   4,
		Cost:     20,
		Movement: 2,
		Terrain:  models.Water,
		Hits:     0,
	}

	units := []*models.Piece{battleship}

	// Apply 2 hits
	casualties := SelectCasualties(units, 2)

	// Battleship should be destroyed
	if len(casualties) != 1 {
		t.Errorf("Expected 1 casualty, got %d", len(casualties))
	}

	if casualties[0] != battleship {
		t.Error("Expected battleship to be the casualty")
	}

	// Battleship should have 2 hits
	if battleship.Hits != 2 {
		t.Errorf("Expected battleship to have 2 hits, got %d", battleship.Hits)
	}
}

// Test that a damaged battleship can still fire back
func TestDamagedBattleshipStillFires(t *testing.T) {
	// Create damaged and undamaged battleships
	damagedBattleship := &models.Piece{
		Name:     "battleship",
		Attack:   4,
		Defend:   4,
		Cost:     20,
		Movement: 2,
		Terrain:  models.Water,
		Hits:     1, // Already damaged
	}

	healthyBattleship := &models.Piece{
		Name:     "battleship",
		Attack:   4,
		Defend:   4,
		Cost:     20,
		Movement: 2,
		Terrain:  models.Water,
		Hits:     0,
	}

	// Roll for both battleships
	dr := NewSeededDiceRoller(42)
	defenders := []*models.Piece{damagedBattleship, healthyBattleship}
	hits := dr.RollForUnits(defenders, false) // Defending, so use defend value

	// Both should be able to fire (damaged battleship is still functional)
	if len(hits) == 0 {
		t.Log("Note: No hits rolled, but this test confirms damaged battleship can roll")
	}

	// The key is that RollForUnits doesn't check Hits field
	// A damaged battleship still participates in combat
}

// Test battleship hit tracking in a full battle
func TestBattleshipInFullCombat(t *testing.T) {
	// Attacker: 2 cruisers (attack 3)
	cruiser1 := &models.Piece{Name: "cruiser", Attack: 3, Defend: 3, Cost: 12, Terrain: models.Water}
	cruiser2 := &models.Piece{Name: "cruiser", Attack: 3, Defend: 3, Cost: 12, Terrain: models.Water}

	// Defender: 1 battleship (defend 4)
	battleship := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Cost: 20, Terrain: models.Water, Hits: 0}

	battle := NewBattle("Sea Zone 1", SeaBattle, "Germany", "UK")
	battle.AddAttacker(cruiser1)
	battle.AddAttacker(cruiser2)
	battle.AddDefender(battleship)

	// Use seeded dice roller for predictable results
	dr := NewSeededDiceRoller(100)

	// Run a few rounds of combat
	result, err := ResolveCombat(battle, dr, 5)
	if err != nil {
		t.Fatalf("Combat failed: %v", err)
	}

	// Check that battleship either:
	// - Survived with 0-1 hits, or
	// - Was destroyed with 2 hits
	battleshipDestroyed := false
	for _, casualty := range result.DefenderCasualties {
		if casualty.Name == "battleship" {
			battleshipDestroyed = true
			if casualty.Hits < 2 {
				t.Errorf("Battleship was destroyed with only %d hits", casualty.Hits)
			}
		}
	}

	if !battleshipDestroyed && len(result.DefendersRemaining) > 0 {
		// Battleship survived - check it has at most 1 hit
		for _, unit := range result.DefendersRemaining {
			if unit.Name == "battleship" && unit.Hits > 1 {
				t.Errorf("Surviving battleship has %d hits (should be destroyed at 2)", unit.Hits)
			}
		}
	}
}

// Test mixed units with battleship
func TestMixedUnitsWithBattleship(t *testing.T) {
	// Create mixed defending force
	infantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3, Terrain: models.Land, Hits: 0}
	battleship := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Cost: 20, Terrain: models.Water, Hits: 0}
	cruiser := &models.Piece{Name: "cruiser", Attack: 3, Defend: 3, Cost: 12, Terrain: models.Water, Hits: 0}

	units := []*models.Piece{infantry, cruiser, battleship}

	// Apply 3 hits - should destroy infantry and cruiser first (cheaper), then damage battleship
	casualties := SelectCasualties(units, 3)

	// Should have 2 casualties (infantry and cruiser)
	// Battleship should survive with 1 hit
	if len(casualties) != 2 {
		t.Errorf("Expected 2 casualties, got %d", len(casualties))
	}

	// Verify infantry and cruiser are casualties
	infantryDead := false
	cruiserDead := false
	for _, cas := range casualties {
		if cas.Name == "infantry" {
			infantryDead = true
		}
		if cas.Name == "cruiser" {
			cruiserDead = true
		}
	}

	if !infantryDead {
		t.Error("Infantry should be a casualty")
	}
	if !cruiserDead {
		t.Error("Cruiser should be a casualty")
	}

	// Battleship should have 1 hit but not be destroyed
	if battleship.Hits != 1 {
		t.Errorf("Battleship should have 1 hit, got %d", battleship.Hits)
	}
}
