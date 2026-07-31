package game

import (
	"boardgame/models"
	"testing"
)

// Test that submarines get surprise strike when no enemy destroyer
func TestSubmarineSurpriseStrikeWithoutDestroyer(t *testing.T) {
	// Attacker: 1 submarine
	sub := &models.Piece{Name: "submarine", Attack: 2, Defend: 1, Cost: 6, Terrain: models.Water}

	// Defender: 1 cruiser (no destroyer)
	cruiser := &models.Piece{Name: "cruiser", Attack: 3, Defend: 3, Cost: 12, Terrain: models.Water}

	battle := NewBattle("Sea Zone 1", SeaBattle, "Germany", "UK")
	battle.AddAttacker(sub)
	battle.AddDefender(cruiser)

	dr := NewSeededDiceRoller(1) // Seed to get predictable hit on roll of 1 or 2

	// Execute one round
	attackerHits, defenderHits, surpriseCas := dr.CombatRound(battle)

	// Submarine surprise hits travel in SurpriseLosses; the returned hit
	// lists carry only unapplied regular hits.
	if len(surpriseCas.AttackerHits) == 0 {
		t.Log("Submarine rolled but didn't hit (acceptable due to dice)")
	}
	if len(attackerHits) != 0 {
		t.Errorf("a lone submarine produced %d regular hits; its strike is the surprise", len(attackerHits))
	}

	// An attacking submarine kills DEFENDING units, so any surprise loss must be
	// recorded against the defender. Previously both sides were credited with
	// every surprise casualty, so this could not be checked at all.
	if len(surpriseCas.Attacker) != 0 {
		t.Errorf("attacking submarine cost the attacker %d units", len(surpriseCas.Attacker))
	}
	if len(surpriseCas.Defender) > 0 && len(defenderHits) > 0 {
		t.Error("Cruiser fired despite being surprise strike casualty")
	}

	t.Logf("Attacker hits: %d, Defender hits: %d, defender surprise losses: %d",
		len(attackerHits), len(defenderHits), len(surpriseCas.Defender))
}

// Test that submarines do NOT get surprise strike when enemy destroyer present
func TestSubmarineNoSurpriseStrikeWithDestroyer(t *testing.T) {
	// Attacker: 1 submarine
	sub := &models.Piece{Name: "submarine", Attack: 2, Defend: 1, Cost: 6, Terrain: models.Water}

	// Defender: 1 destroyer (cancels surprise strike)
	destroyer := &models.Piece{Name: "destroyer", Attack: 2, Defend: 2, Cost: 8, Terrain: models.Water}

	battle := NewBattle("Sea Zone 1", SeaBattle, "Germany", "UK")
	battle.AddAttacker(sub)
	battle.AddDefender(destroyer)

	dr := NewSeededDiceRoller(1)

	// Execute one round
	attackerHits, defenderHits, surpriseCas := dr.CombatRound(battle)

	// There should be NO surprise casualties because destroyer cancels it
	if n := len(surpriseCas.Attacker) + len(surpriseCas.Defender); n != 0 {
		t.Errorf("Expected 0 surprise casualties with destroyer present, got %d", n)
	}

	// Both units should have chance to fire in regular combat
	t.Logf("Attacker hits: %d, Defender hits: %d (both should have fired)",
		len(attackerHits), len(defenderHits))
}

// Test defending submarine surprise strike
func TestDefendingSubmarineSurpriseStrike(t *testing.T) {
	// Attacker: 1 cruiser (no destroyer)
	cruiser := &models.Piece{Name: "cruiser", Attack: 3, Defend: 3, Cost: 12, Terrain: models.Water}

	// Defender: 1 submarine
	sub := &models.Piece{Name: "submarine", Attack: 2, Defend: 1, Cost: 6, Terrain: models.Water}

	battle := NewBattle("Sea Zone 1", SeaBattle, "Germany", "UK")
	battle.AddAttacker(cruiser)
	battle.AddDefender(sub)

	dr := NewSeededDiceRoller(1) // Submarine defends at 1, so roll of 1 hits

	// Execute one round
	attackerHits, defenderHits, surpriseCas := dr.CombatRound(battle)

	// A defending submarine kills ATTACKING units, so the loss belongs to the
	// attacker's column.
	if len(surpriseCas.Defender) != 0 {
		t.Errorf("defending submarine cost the defender %d units", len(surpriseCas.Defender))
	}
	if len(surpriseCas.DefenderHits) > 0 && len(surpriseCas.Attacker) > 0 {
		cruiserKilled := false
		for _, cas := range surpriseCas.Attacker {
			if cas.Name == "cruiser" {
				cruiserKilled = true
			}
		}
		if cruiserKilled && len(attackerHits) > 0 {
			t.Error("Cruiser fired despite being killed in surprise strike")
		}
	}

	t.Logf("Attacker hits: %d, Defender hits: %d, attacker surprise losses: %d",
		len(attackerHits), len(defenderHits), len(surpriseCas.Attacker))
}

// Test both sides have submarines without destroyers
func TestMutualSubmarineSurpriseStrikes(t *testing.T) {
	// Attacker: 1 submarine
	attackSub := &models.Piece{Name: "submarine", Attack: 2, Defend: 1, Cost: 6, Terrain: models.Water}

	// Defender: 1 submarine
	defendSub := &models.Piece{Name: "submarine", Attack: 2, Defend: 1, Cost: 6, Terrain: models.Water}

	battle := NewBattle("Sea Zone 1", SeaBattle, "Germany", "UK")
	battle.AddAttacker(attackSub)
	battle.AddDefender(defendSub)

	dr := NewSeededDiceRoller(42)

	// Execute one round
	attackerHits, defenderHits, surpriseCas := dr.CombatRound(battle)

	// Both submarines fire in the surprise strike phase, and each side's losses
	// are attributed separately.
	t.Logf("Mutual sub battle - attacker hits: %d, defender hits: %d, "+
		"attacker losses: %d, defender losses: %d",
		len(attackerHits), len(defenderHits),
		len(surpriseCas.Attacker), len(surpriseCas.Defender))

	// This is valid - both subs fire simultaneously in surprise strike
}

// Test mixed fleet with submarine - destroyer cancels surprise
func TestMixedFleetWithDestroyerCancelsSurprise(t *testing.T) {
	// Attacker: 1 submarine, 1 cruiser
	sub := &models.Piece{Name: "submarine", Attack: 2, Defend: 1, Cost: 6, Terrain: models.Water}
	cruiser := &models.Piece{Name: "cruiser", Attack: 3, Defend: 3, Cost: 12, Terrain: models.Water}

	// Defender: 1 destroyer, 1 battleship
	destroyer := &models.Piece{Name: "destroyer", Attack: 2, Defend: 2, Cost: 8, Terrain: models.Water}
	battleship := &models.Piece{Name: "battleship", Attack: 4, Defend: 4, Cost: 20, Terrain: models.Water, Hits: 0}

	battle := NewBattle("Sea Zone 1", SeaBattle, "Germany", "UK")
	battle.AddAttacker(sub)
	battle.AddAttacker(cruiser)
	battle.AddDefender(destroyer)
	battle.AddDefender(battleship)

	dr := NewSeededDiceRoller(100)

	// Execute one round
	_, _, surpriseCas := dr.CombatRound(battle)

	// Destroyer should prevent submarine surprise strike
	if n := len(surpriseCas.Attacker) + len(surpriseCas.Defender); n != 0 {
		t.Errorf("Expected no surprise casualties when destroyer present, got %d", n)
	}
}

// Test that air units participating in sea battle
func TestAirUnitsInSeaBattle(t *testing.T) {
	// Attacker: 1 fighter
	fighter := &models.Piece{Name: "fighter", Attack: 3, Defend: 4, Cost: 10, Terrain: models.Air}

	// Defender: 1 submarine (air cannot hit without friendly destroyer)
	sub := &models.Piece{Name: "submarine", Attack: 2, Defend: 1, Cost: 6, Terrain: models.Water}

	battle := NewBattle("Sea Zone 1", SeaBattle, "Germany", "UK")
	battle.AddAttacker(fighter)
	battle.AddDefender(sub)

	dr := NewSeededDiceRoller(1) // Very low roll to ensure hits

	// Run full combat
	result, err := ResolveCombat(battle, dr, 10)
	if err != nil {
		t.Fatalf("Combat failed: %v", err)
	}

	// Fighter should not be able to hit submarine without a destroyer
	// Submarine should survive unless removed by other means
	// Note: Current implementation doesn't fully enforce this yet
	// This test documents expected behavior

	t.Logf("Result: Attacker casualties: %d, Defender casualties: %d",
		len(result.AttackerCasualties), len(result.DefenderCasualties))
}

// Helper test to verify hasDestroyer function
func TestHasDestroyer(t *testing.T) {
	destroyer := &models.Piece{Name: "destroyer", Cost: 8}
	cruiser := &models.Piece{Name: "cruiser", Cost: 12}
	sub := &models.Piece{Name: "submarine", Cost: 6}

	// Fleet with destroyer
	fleetWithDD := []*models.Piece{destroyer, cruiser}
	if !hasDestroyer(fleetWithDD) {
		t.Error("Should detect destroyer in fleet")
	}

	// Fleet without destroyer
	fleetWithoutDD := []*models.Piece{cruiser, sub}
	if hasDestroyer(fleetWithoutDD) {
		t.Error("Should not detect destroyer when none present")
	}

	// Empty fleet
	if hasDestroyer([]*models.Piece{}) {
		t.Error("Empty fleet should not have destroyer")
	}
}

// Test getSubmarines helper
func TestGetSubmarines(t *testing.T) {
	sub1 := &models.Piece{Name: "submarine", Cost: 6}
	sub2 := &models.Piece{Name: "submarine", Cost: 6}
	cruiser := &models.Piece{Name: "cruiser", Cost: 12}
	destroyer := &models.Piece{Name: "destroyer", Cost: 8}

	fleet := []*models.Piece{sub1, cruiser, sub2, destroyer}
	subs := getSubmarines(fleet)

	if len(subs) != 2 {
		t.Errorf("Expected 2 submarines, got %d", len(subs))
	}

	for _, sub := range subs {
		if sub.Name != "submarine" {
			t.Errorf("Expected submarine, got %s", sub.Name)
		}
	}
}

// One surprise hit kills exactly one unit, and the victim is dead: not
// fighting on, not among the survivors.
//
// Two compounding bugs made this fail, found by batch play (two of a hundred
// games ended with corrupt boards). CombatRound applied surprise casualties
// to the battle's lists but returned the surprise hits merged into the hit
// lists, so the resolver applied every submarine hit a second time against
// its own stale local slices -- and because the surprise pass shields
// submarines from air while the plain pass does not, the two passes could
// pick DIFFERENT victims: two units died to one hit, and the first victim
// stayed in the local survivor list. On a retreat, the controller then
// deleted that piece as a casualty AND withdrew it as a survivor, leaving a
// dead ID in the origin territory's list.
func TestSurpriseStrike_OneHitKillsExactlyOneUnit(t *testing.T) {
	// Defending sub with defend 6 always hits. The attackers include a
	// submarine of their own -- the cheapest unit, which the surprise pass
	// shields and a naive second pass would kill instead.
	defendingSub := &models.Piece{Name: "sub", Attack: 2, Defend: 6, Cost: 8, Terrain: models.Water, ID: 100}
	attackingSub := &models.Piece{Name: "sub", Attack: 0, Defend: 1, Cost: 6, Terrain: models.Water, ID: 101}
	battleship := &models.Piece{Name: "cruiserish", Attack: 3, Defend: 6, Cost: 20, Terrain: models.Water, ID: 102}

	battle := NewBattle("Deep Water", SeaBattle, "Germany", "UK")
	battle.Attackers = []*models.Piece{attackingSub, battleship}
	battle.Defenders = []*models.Piece{defendingSub}

	// Break off after the first round, so survivors are computed mid-fight.
	retreat := func(_, _, _, _, round int) bool { return round >= 1 }

	result, err := ResolveCombatWithRetreat(battle, NewSeededDiceRoller(3), 10, retreat)
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}

	// The always-hitting sub fired once in round one: exactly one attacker is
	// dead, and it is the shielded-selection victim (the warship, not the sub).
	if len(result.AttackerCasualties) != 1 {
		t.Fatalf("one surprise hit killed %d units, want exactly 1: %v",
			len(result.AttackerCasualties), result.AttackerCasualties)
	}
	if result.AttackerCasualties[0].ID != battleship.ID {
		t.Errorf("the surprise killed piece %d; air-shielded selection should spare the submarine",
			result.AttackerCasualties[0].ID)
	}

	dead := make(map[int]bool)
	for _, casualty := range result.AttackerCasualties {
		dead[casualty.ID] = true
	}
	for _, survivor := range result.AttackersRemaining {
		if dead[survivor.ID] {
			t.Fatalf("piece %d is recorded both as a casualty and a survivor", survivor.ID)
		}
	}
}
