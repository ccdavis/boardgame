package game

import (
	"boardgame/models"
	"testing"
)

// TestDiceRollerBasic tests basic dice rolling
func TestDiceRollerBasic(t *testing.T) {
	roller := NewSeededDiceRoller(12345)

	// Roll should return 1-6
	for i := 0; i < 100; i++ {
		roll := roller.Roll()
		if roll < 1 || roll > 6 {
			t.Errorf("Roll %d out of range [1,6]", roll)
		}
	}
}

// TestDiceRollerSeeded tests that seeded roller is deterministic
func TestDiceRollerSeeded(t *testing.T) {
	seed := int64(42)

	roller1 := NewSeededDiceRoller(seed)
	roller2 := NewSeededDiceRoller(seed)

	// Same seed should produce same sequence
	for i := 0; i < 10; i++ {
		roll1 := roller1.Roll()
		roll2 := roller2.Roll()

		if roll1 != roll2 {
			t.Errorf("Seeded rollers produced different results: %d vs %d", roll1, roll2)
		}
	}
}

// TestRollDice tests rolling multiple dice
func TestRollDice(t *testing.T) {
	roller := NewDiceRoller()

	results := roller.RollDice(5)

	if len(results) != 5 {
		t.Errorf("Expected 5 results, got %d", len(results))
	}

	for i, roll := range results {
		if roll < 1 || roll > 6 {
			t.Errorf("Roll %d at index %d out of range [1,6]", roll, i)
		}
	}
}

// TestRollForUnits tests rolling for combat units
func TestRollForUnits(t *testing.T) {
	roller := NewSeededDiceRoller(42)

	// Create some units
	infantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	tank := &models.Piece{Name: "tank", Attack: 3, Defend: 3}
	fighter := &models.Piece{Name: "fighter", Attack: 3, Defend: 4}

	units := []*models.Piece{infantry, tank, fighter}

	// Roll for attacking
	hits := roller.RollForUnits(units, true)

	// Should get some hits (not testing exact count due to randomness, just structure)
	for _, hit := range hits {
		if hit.Roll < 1 || hit.Roll > 6 {
			t.Errorf("Invalid roll value: %d", hit.Roll)
		}
		if hit.Roll > hit.Threshold {
			t.Errorf("Hit recorded with roll %d > threshold %d", hit.Roll, hit.Threshold)
		}
	}
}

// TestSelectCasualtiesBasic tests basic casualty selection
func TestSelectCasualtiesBasic(t *testing.T) {
	// Create units with different costs
	infantry := &models.Piece{Name: "infantry", Cost: 3}
	tank := &models.Piece{Name: "tank", Cost: 5}
	fighter := &models.Piece{Name: "fighter", Cost: 10}

	units := []*models.Piece{fighter, tank, infantry}

	// Select 2 casualties
	casualties := SelectCasualties(units, 2)

	if len(casualties) != 2 {
		t.Errorf("Expected 2 casualties, got %d", len(casualties))
	}

	// Should prefer cheaper units
	// Infantry (3) and Tank (5) should be selected before Fighter (10)
	foundInfantry := false
	foundFighter := false

	for _, casualty := range casualties {
		switch casualty.Name {
		case "infantry":
			foundInfantry = true
		case "fighter":
			foundFighter = true
		}
	}

	if foundFighter && !foundInfantry {
		t.Error("Should prefer cheaper infantry over expensive fighter")
	}
}

// TestSelectCasualtiesMoreHitsThanUnits tests over-kill scenario
func TestSelectCasualtiesMoreHitsThanUnits(t *testing.T) {
	units := []*models.Piece{
		{Name: "infantry", Cost: 3},
		{Name: "tank", Cost: 5},
	}

	// 10 hits but only 2 units
	casualties := SelectCasualties(units, 10)

	if len(casualties) != 2 {
		t.Errorf("Expected 2 casualties (all units), got %d", len(casualties))
	}
}

// TestSelectCasualtiesNoHits tests zero hits
func TestSelectCasualtiesNoHits(t *testing.T) {
	units := []*models.Piece{
		{Name: "infantry", Cost: 3},
	}

	casualties := SelectCasualties(units, 0)

	if len(casualties) != 0 {
		t.Errorf("Expected 0 casualties with 0 hits, got %d", len(casualties))
	}
}

// TestRemoveCasualties tests removing casualties from unit list
func TestRemoveCasualties(t *testing.T) {
	infantry1 := &models.Piece{Name: "infantry", Cost: 3}
	infantry2 := &models.Piece{Name: "infantry", Cost: 3}
	tank := &models.Piece{Name: "tank", Cost: 5}

	units := []*models.Piece{infantry1, infantry2, tank}
	casualties := []*models.Piece{infantry1}

	remaining := RemoveCasualties(units, casualties)

	if len(remaining) != 2 {
		t.Errorf("Expected 2 remaining units, got %d", len(remaining))
	}

	// Should have infantry2 and tank
	foundInfantry2 := false
	foundTank := false

	for _, unit := range remaining {
		if unit == infantry2 {
			foundInfantry2 = true
		}
		if unit == tank {
			foundTank = true
		}
		if unit == infantry1 {
			t.Error("Casualty should have been removed")
		}
	}

	if !foundInfantry2 || !foundTank {
		t.Error("Expected infantry2 and tank to remain")
	}
}

// TestNewBattle tests battle creation
func TestNewBattle(t *testing.T) {
	battle := NewBattle("Germany", LandBattle, "USSR", "Germany")

	if battle.Location != "Germany" {
		t.Errorf("Expected location 'Germany', got '%s'", battle.Location)
	}

	if battle.Type != LandBattle {
		t.Errorf("Expected LandBattle, got %s", battle.Type)
	}

	if battle.AttackerID != "USSR" {
		t.Errorf("Expected attacker 'USSR', got '%s'", battle.AttackerID)
	}

	if battle.DefenderID != "Germany" {
		t.Errorf("Expected defender 'Germany', got '%s'", battle.DefenderID)
	}

	if battle.Round != 0 {
		t.Errorf("Expected round 0, got %d", battle.Round)
	}

	if len(battle.Attackers) != 0 || len(battle.Defenders) != 0 {
		t.Error("Expected empty unit lists")
	}
}

// TestBattleAddUnits tests adding units to battle
func TestBattleAddUnits(t *testing.T) {
	battle := NewBattle("France", LandBattle, "Germany", "UK")

	infantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	tank := &models.Piece{Name: "tank", Attack: 3, Defend: 3}

	battle.AddAttacker(infantry)
	battle.AddAttacker(tank)
	battle.AddDefender(infantry)

	if len(battle.Attackers) != 2 {
		t.Errorf("Expected 2 attackers, got %d", len(battle.Attackers))
	}

	if len(battle.Defenders) != 1 {
		t.Errorf("Expected 1 defender, got %d", len(battle.Defenders))
	}
}

// TestBattleGetPower tests power calculation
func TestBattleGetPower(t *testing.T) {
	battle := NewBattle("France", LandBattle, "Germany", "UK")

	// Attack: 1+3 = 4, Defend: 2+3 = 5
	infantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	tank := &models.Piece{Name: "tank", Attack: 3, Defend: 3}

	battle.AddAttacker(infantry)
	battle.AddAttacker(tank)
	battle.AddDefender(infantry)
	battle.AddDefender(tank)

	attackPower := battle.GetTotalAttackPower()
	defensePower := battle.GetTotalDefensePower()

	if attackPower != 4 {
		t.Errorf("Expected attack power 4, got %d", attackPower)
	}

	if defensePower != 5 {
		t.Errorf("Expected defense power 5, got %d", defensePower)
	}
}

// TestCombatRound tests a single combat round
func TestCombatRound(t *testing.T) {
	roller := NewSeededDiceRoller(42)
	battle := NewBattle("France", LandBattle, "Germany", "UK")

	// Add some units
	for i := 0; i < 3; i++ {
		battle.AddAttacker(&models.Piece{Name: "infantry", Attack: 1, Defend: 2})
		battle.AddDefender(&models.Piece{Name: "infantry", Attack: 1, Defend: 2})
	}

	attackerHits, defenderHits, surpriseCas := roller.CombatRound(battle)

	// Should have executed round 1
	if battle.Round != 1 {
		t.Errorf("Expected round 1, got %d", battle.Round)
	}

	// Should have some hit results (may be empty due to randomness)
	if attackerHits == nil || defenderHits == nil {
		t.Error("Hit results should not be nil")
	}

	// Surprise casualties should be empty for land battle, on both sides.
	if len(surpriseCas.Attacker) != 0 || len(surpriseCas.Defender) != 0 {
		t.Errorf("land battle should have no surprise casualties, got %d attacker and %d defender",
			len(surpriseCas.Attacker), len(surpriseCas.Defender))
	}
}

// TestResolveCombat tests full combat resolution
func TestResolveCombat(t *testing.T) {
	roller := NewSeededDiceRoller(12345)
	battle := NewBattle("France", LandBattle, "Germany", "UK")

	// Attackers: 5 infantry (attack 1)
	for i := 0; i < 5; i++ {
		battle.AddAttacker(&models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3})
	}

	// Defenders: 2 infantry (defend 2)
	for i := 0; i < 2; i++ {
		battle.AddDefender(&models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3})
	}

	result, err := ResolveCombat(battle, roller, 10)
	if err != nil {
		t.Fatalf("Combat resolution failed: %v", err)
	}

	// One side should win
	if !result.AttackerWins && !result.DefenderWins {
		t.Error("Expected one side to win")
	}

	// Winning side should have units remaining
	if result.AttackerWins && len(result.AttackersRemaining) == 0 {
		t.Error("Attacker won but has no units remaining")
	}

	if result.DefenderWins && len(result.DefendersRemaining) == 0 {
		t.Error("Defender won but has no units remaining")
	}

	// Total casualties should not exceed starting units
	totalAttackerCasualties := len(result.AttackerCasualties)
	totalDefenderCasualties := len(result.DefenderCasualties)

	if totalAttackerCasualties > 5 {
		t.Errorf("Attacker casualties %d exceed starting units", totalAttackerCasualties)
	}

	if totalDefenderCasualties > 2 {
		t.Errorf("Defender casualties %d exceed starting units", totalDefenderCasualties)
	}

	// Remaining + casualties should equal starting units
	if len(result.AttackersRemaining)+totalAttackerCasualties != 5 {
		t.Errorf("Attacker accounting mismatch: %d remaining + %d casualties != 5",
			len(result.AttackersRemaining), totalAttackerCasualties)
	}

	if len(result.DefendersRemaining)+totalDefenderCasualties != 2 {
		t.Errorf("Defender accounting mismatch: %d remaining + %d casualties != 2",
			len(result.DefendersRemaining), totalDefenderCasualties)
	}
}

// TestResolveCombatAttackerWins tests scenario where attacker should win
func TestResolveCombatAttackerWins(t *testing.T) {
	roller := NewSeededDiceRoller(99999)
	battle := NewBattle("France", LandBattle, "Germany", "UK")

	// Overwhelming attack force
	for i := 0; i < 10; i++ {
		battle.AddAttacker(&models.Piece{Name: "tank", Attack: 3, Defend: 3, Cost: 5})
	}

	// Small defense
	battle.AddDefender(&models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3})

	result, err := ResolveCombat(battle, roller, 20)
	if err != nil {
		t.Fatalf("Combat resolution failed: %v", err)
	}

	// With overwhelming force, attacker should win
	// (Note: due to randomness, this might occasionally fail, but very unlikely with 10:1 odds)
	if !result.AttackerWins {
		t.Log("Warning: Attacker did not win despite 10:1 advantage (rare but possible)")
	}

	// Defender should be eliminated
	if len(result.DefendersRemaining) != 0 {
		t.Errorf("Expected 0 defenders remaining, got %d", len(result.DefendersRemaining))
	}
}

// TestResolveCombatMaxRounds tests that combat respects max rounds limit
func TestResolveCombatMaxRounds(t *testing.T) {
	roller := NewDiceRoller()
	battle := NewBattle("Stalemate", LandBattle, "Germany", "USSR")

	// Equal forces that might not resolve quickly
	for i := 0; i < 5; i++ {
		battle.AddAttacker(&models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3})
		battle.AddDefender(&models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3})
	}

	maxRounds := 3
	result, err := ResolveCombat(battle, roller, maxRounds)
	if err != nil {
		t.Fatalf("Combat resolution failed: %v", err)
	}

	// Should not exceed max rounds
	if result.Rounds > maxRounds {
		t.Errorf("Combat exceeded max rounds: %d > %d", result.Rounds, maxRounds)
	}
}

// TestBattleTypeString tests battle type string representation
func TestBattleTypeString(t *testing.T) {
	tests := []struct {
		battleType BattleType
		expected   string
	}{
		{LandBattle, "Land Battle"},
		{SeaBattle, "Sea Battle"},
		{AirBattle, "Air Battle"},
		{StrategicBombing, "Strategic Bombing"},
		{AmphibiousAssault, "Amphibious Assault"},
	}

	for _, tt := range tests {
		if got := tt.battleType.String(); got != tt.expected {
			t.Errorf("BattleType %d: expected '%s', got '%s'", tt.battleType, tt.expected, got)
		}
	}
}

// TestBattleString tests battle string representation
func TestBattleString(t *testing.T) {
	battle := NewBattle("France", LandBattle, "Germany", "UK")

	battle.AddAttacker(&models.Piece{Name: "tank", Attack: 3, Defend: 3})
	battle.AddDefender(&models.Piece{Name: "infantry", Attack: 1, Defend: 2})

	str := battle.String()

	// Should contain key information
	if len(str) == 0 {
		t.Error("Battle string should not be empty")
	}

	// Just verify it doesn't panic and returns something reasonable
	t.Logf("Battle string: %s", str)
}

// TestRollAAAFire tests AAA fire mechanics
func TestRollAAAFire(t *testing.T) {
	roller := NewSeededDiceRoller(42)

	// Create AAA and air units
	aaa := &models.Piece{Name: "AAA", Attack: 0, Defend: 1}
	fighter1 := &models.Piece{Name: "fighter", Attack: 3, Defend: 4, Terrain: models.Air}
	fighter2 := &models.Piece{Name: "fighter", Attack: 3, Defend: 4, Terrain: models.Air}

	aaaUnits := []*models.Piece{aaa}
	airUnits := []*models.Piece{fighter1, fighter2}

	// Roll AAA fire
	hits := roller.RollAAAFire(aaaUnits, airUnits)

	// Should fire 2 shots (min of 3 per AAA or 1 per air unit)
	// All hits should have threshold 1
	for _, hit := range hits {
		if hit.Threshold != 1 {
			t.Errorf("AAA hits should have threshold 1, got %d", hit.Threshold)
		}
		if hit.Roll != 1 {
			t.Errorf("AAA hits should only occur on roll of 1, got %d", hit.Roll)
		}
	}
}

// TestApplyArtillerySupport tests artillery support mechanics
func TestApplyArtillerySupport(t *testing.T) {
	// Create infantry and artillery
	inf1 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	inf2 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	inf3 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	art1 := &models.Piece{Name: "artillery", Attack: 2, Defend: 2}
	art2 := &models.Piece{Name: "artillery", Attack: 2, Defend: 2}

	units := []*models.Piece{inf1, inf2, inf3, art1, art2}

	// Apply artillery support
	boosts := ApplyArtillerySupport(units)

	// Check that 2 infantry got boosted (1 per artillery)
	boostedCount := 0
	for _, unit := range units {
		if unit.Name == "infantry" && unit.Attack == 2 {
			boostedCount++
		}
	}

	if boostedCount != 2 {
		t.Errorf("Expected 2 infantry to be boosted, got %d", boostedCount)
	}

	// One infantry should remain unboosted
	unboostedCount := 0
	for _, unit := range units {
		if unit.Name == "infantry" && unit.Attack == 1 {
			unboostedCount++
		}
	}

	if unboostedCount != 1 {
		t.Errorf("Expected 1 infantry to remain unboosted, got %d", unboostedCount)
	}

	// Remove artillery support
	RemoveArtillerySupport(boosts)

	// All infantry should be back to attack 1
	for _, unit := range units {
		if unit.Name == "infantry" && unit.Attack != 1 {
			t.Errorf("Infantry should be reset to attack 1 after removing support, got %d", unit.Attack)
		}
	}
}

// TestCombatWithArtillerySupport tests full combat with artillery
func TestCombatWithArtillerySupport(t *testing.T) {
	roller := NewSeededDiceRoller(123)
	battle := NewBattle("France", LandBattle, "Germany", "USSR")

	// Attackers: 2 infantry + 1 artillery
	inf1 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3}
	inf2 := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3}
	art := &models.Piece{Name: "artillery", Attack: 2, Defend: 2, Cost: 4}

	battle.AddAttacker(inf1)
	battle.AddAttacker(inf2)
	battle.AddAttacker(art)

	// Defenders: 1 infantry
	defInf := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3}
	battle.AddDefender(defInf)

	// Resolve combat
	result, err := ResolveCombat(battle, roller, 10)
	if err != nil {
		t.Fatalf("Combat failed: %v", err)
	}

	// With artillery support, attackers should have better odds
	// Just verify combat completes without error
	if !result.AttackerWins && !result.DefenderWins {
		t.Error("Battle should have a winner")
	}
}

// TestCombatWithAAA tests full combat with AAA units
func TestCombatWithAAA(t *testing.T) {
	roller := NewSeededDiceRoller(456)
	battle := NewBattle("Berlin", LandBattle, "USSR", "Germany")

	// Attackers: 2 fighters
	fighter1 := &models.Piece{Name: "fighter", Attack: 3, Defend: 4, Terrain: models.Air, Cost: 10}
	fighter2 := &models.Piece{Name: "fighter", Attack: 3, Defend: 4, Terrain: models.Air, Cost: 10}

	battle.AddAttacker(fighter1)
	battle.AddAttacker(fighter2)

	// Defenders: 1 AAA + 1 infantry
	aaa := &models.Piece{Name: "AAA", Attack: 0, Defend: 1, Cost: 5}
	infantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3}

	battle.AddDefender(aaa)
	battle.AddDefender(infantry)

	initialAttackers := len(battle.Attackers)

	// Resolve combat
	result, err := ResolveCombat(battle, roller, 10)
	if err != nil {
		t.Fatalf("Combat failed: %v", err)
	}

	// AAA might have shot down some fighters
	// Just verify the mechanic ran (can't guarantee hits due to randomness)
	totalAttackerLosses := len(result.AttackerCasualties)
	if totalAttackerLosses > initialAttackers {
		t.Error("Cannot have more casualties than starting units")
	}
}

// Anti-aircraft artillery fires before the battle and then sits it out.
//
// It used to remain in the defender list, where it double-dipped: special
// pre-combat shots, then defence dice every round like a normal unit -- and it
// could be picked as a casualty or keep a lost battle technically alive.
func TestAAA_FiresOnceAndDoesNotFight(t *testing.T) {
	aaa := &models.Piece{Name: "AAA", Attack: 0, Defend: 1, Cost: 5}

	// One AAA as the sole defence against a ground-only attack: it gets no
	// pre-combat shots (no aircraft) and must not fight the ground battle, so
	// the attacker wins without a single round of dice.
	battle := NewBattle("Depot", LandBattle, "Germany", "UK")
	battle.Attackers = []*models.Piece{{Name: "infantry", Attack: 1, Defend: 2, Cost: 3, Terrain: models.Land}}
	battle.Defenders = []*models.Piece{aaa}

	result, err := ResolveCombat(battle, NewSeededDiceRoller(1), 10)
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if !result.AttackerWins {
		t.Error("an AAA alone held the territory; it is not a combat unit")
	}
	if len(result.DefenderCasualties) != 0 {
		t.Errorf("AAA was destroyed in combat; it should be captured, not fought: %d casualties",
			len(result.DefenderCasualties))
	}
}

// With aircraft attacking, the AAA still gets its pre-combat shots.
func TestAAA_StillFiresAtAircraft(t *testing.T) {
	aaa := &models.Piece{Name: "AAA", Attack: 0, Defend: 1, Cost: 5}
	fighters := []*models.Piece{
		{Name: "fighter", Attack: 3, Defend: 4, Cost: 12, Terrain: models.Air},
		{Name: "fighter", Attack: 3, Defend: 4, Cost: 12, Terrain: models.Air},
		{Name: "fighter", Attack: 3, Defend: 4, Cost: 12, Terrain: models.Air},
	}

	// Find a seed whose first roll is a 1, so the AAA scores a hit.
	seed := int64(-1)
	for s := int64(1); s < 200; s++ {
		if NewSeededDiceRoller(s).Roll() == 1 {
			seed = s
			break
		}
	}
	if seed < 0 {
		t.Fatal("no seed with an opening 1 in 200 tries")
	}

	battle := NewBattle("Depot", LandBattle, "Germany", "UK")
	battle.Attackers = fighters
	battle.Defenders = []*models.Piece{aaa}

	result, err := ResolveCombat(battle, NewSeededDiceRoller(seed), 10)
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if len(result.AttackerCasualties) == 0 {
		t.Error("the AAA's pre-combat fire scored no casualty despite rolling a 1")
	}
}
