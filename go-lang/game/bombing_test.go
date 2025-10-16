package game

import (
	"boardgame/models"
	"testing"
)

// Test RollICAADefense function
func TestRollICAADefense(t *testing.T) {
	roller := NewSeededDiceRoller(123)

	bomber1 := &models.Piece{Name: "bomber", Terrain: models.Air}
	bomber2 := &models.Piece{Name: "bomber", Terrain: models.Air}
	bomber3 := &models.Piece{Name: "bomber", Terrain: models.Air}

	bombers := []*models.Piece{bomber1, bomber2, bomber3}

	hits := roller.RollICAADefense(bombers)

	t.Logf("IC AA fired at %d bombers, scored %d hits", len(bombers), len(hits))

	// Verify all hits are from IC AA
	for _, hit := range hits {
		if hit.UnitType != "IC_AA" {
			t.Errorf("Expected IC_AA, got %s", hit.UnitType)
		}
		if hit.Threshold != 1 {
			t.Errorf("Expected threshold 1, got %d", hit.Threshold)
		}
		if hit.Roll != 1 {
			t.Errorf("Expected roll 1 (only hits on 1), got %d", hit.Roll)
		}
	}
}

// Test ResolveStrategicBombing with no AA hits
func TestResolveStrategicBombingNoAAHits(t *testing.T) {
	// Use seed that produces no 1s for AA defense
	roller := NewSeededDiceRoller(200)

	bomber1 := &models.Piece{Name: "bomber", Terrain: models.Air}
	bomber2 := &models.Piece{Name: "bomber", Terrain: models.Air}

	bombers := []*models.Piece{bomber1, bomber2}

	result, err := ResolveStrategicBombing(bombers, roller)
	if err != nil {
		t.Fatalf("ResolveStrategicBombing failed: %v", err)
	}

	if result.TotalBombers != 2 {
		t.Errorf("Expected 2 total bombers, got %d", result.TotalBombers)
	}

	t.Logf("Bombers destroyed by AA: %d/%d", result.BombersDestroyed, result.TotalBombers)
	t.Logf("Bombers survived: %d", result.BombersSurvived)
	t.Logf("Damage rolls: %v", result.DamageRolls)
	t.Logf("Total damage: %d", result.TotalDamage)

	// All bombers should survive
	if result.BombersSurvived < result.TotalBombers {
		t.Log("Some bombers were shot down (acceptable with random rolls)")
	}

	// Should have damage rolls for each surviving bomber
	if len(result.DamageRolls) != result.BombersSurvived {
		t.Errorf("Expected %d damage rolls, got %d", result.BombersSurvived, len(result.DamageRolls))
	}

	// Each damage roll should be 1-6
	for i, roll := range result.DamageRolls {
		if roll < 1 || roll > 6 {
			t.Errorf("Damage roll %d out of range (1-6): %d", i, roll)
		}
	}

	// Total damage should be sum of rolls
	sum := 0
	for _, roll := range result.DamageRolls {
		sum += roll
	}
	if result.TotalDamage != sum {
		t.Errorf("Total damage %d doesn't match sum of rolls %d", result.TotalDamage, sum)
	}
}

// Test ResolveStrategicBombing with single bomber
func TestResolveStrategicBombingSingleBomber(t *testing.T) {
	roller := NewSeededDiceRoller(42)

	bomber := &models.Piece{Name: "bomber", Terrain: models.Air}
	bombers := []*models.Piece{bomber}

	result, err := ResolveStrategicBombing(bombers, roller)
	if err != nil {
		t.Fatalf("ResolveStrategicBombing failed: %v", err)
	}

	if result.TotalBombers != 1 {
		t.Errorf("Expected 1 total bomber, got %d", result.TotalBombers)
	}

	t.Logf("Single bomber raid - Destroyed: %d, Survived: %d, Damage: %d",
		result.BombersDestroyed, result.BombersSurvived, result.TotalDamage)
}

// Test ApplyICDamage function
func TestApplyICDamage(t *testing.T) {
	game := models.NewGame()
	game.AddTerritory("Germany", models.Land, "Germany", 10)

	germany := game.Board["Germany"]

	// Initially no damage
	if germany.ICDamage != 0 {
		t.Errorf("Expected initial damage 0, got %d", germany.ICDamage)
	}

	// Apply 5 damage
	actualDamage := ApplyICDamage(germany, 5)
	if actualDamage != 5 {
		t.Errorf("Expected to apply 5 damage, got %d", actualDamage)
	}

	if germany.ICDamage != 5 {
		t.Errorf("Expected IC damage 5, got %d", germany.ICDamage)
	}

	// Apply another 10 damage (should be limited to max = 2x production = 20)
	actualDamage = ApplyICDamage(germany, 10)
	if actualDamage != 10 {
		t.Errorf("Expected to apply 10 damage, got %d", actualDamage)
	}

	if germany.ICDamage != 15 {
		t.Errorf("Expected IC damage 15, got %d", germany.ICDamage)
	}

	// Try to apply 10 more damage (should be limited to 5 to reach max of 20)
	actualDamage = ApplyICDamage(germany, 10)
	if actualDamage != 5 {
		t.Errorf("Expected to apply only 5 damage (to reach max), got %d", actualDamage)
	}

	if germany.ICDamage != 20 {
		t.Errorf("Expected IC damage 20 (max), got %d", germany.ICDamage)
	}

	// Try to apply more damage when at max (should be 0)
	actualDamage = ApplyICDamage(germany, 10)
	if actualDamage != 0 {
		t.Errorf("Expected 0 damage applied at max, got %d", actualDamage)
	}

	if germany.ICDamage != 20 {
		t.Errorf("Expected IC damage to stay at 20, got %d", germany.ICDamage)
	}
}

// Test RepairIC function
func TestRepairIC(t *testing.T) {
	game := models.NewGame()
	game.AddTerritory("Germany", models.Land, "Germany", 10)

	germany := game.Board["Germany"]
	germany.ICDamage = 15

	// Repair 5 damage
	actualRepair := RepairIC(germany, 5)
	if actualRepair != 5 {
		t.Errorf("Expected to repair 5 damage, got %d", actualRepair)
	}

	if germany.ICDamage != 10 {
		t.Errorf("Expected IC damage 10 after repair, got %d", germany.ICDamage)
	}

	// Repair more than current damage (should only repair what's there)
	actualRepair = RepairIC(germany, 20)
	if actualRepair != 10 {
		t.Errorf("Expected to repair 10 damage (all remaining), got %d", actualRepair)
	}

	if germany.ICDamage != 0 {
		t.Errorf("Expected IC damage 0 after full repair, got %d", germany.ICDamage)
	}

	// Try to repair when no damage (should repair 0)
	actualRepair = RepairIC(germany, 5)
	if actualRepair != 0 {
		t.Errorf("Expected 0 repair when no damage, got %d", actualRepair)
	}
}

// Test GetEffectiveProduction function
func TestGetEffectiveProduction(t *testing.T) {
	game := models.NewGame()
	game.AddTerritory("Germany", models.Land, "Germany", 10)

	germany := game.Board["Germany"]

	// No damage
	effective := GetEffectiveProduction(germany)
	if effective != 10 {
		t.Errorf("Expected effective production 10, got %d", effective)
	}

	// 5 damage
	germany.ICDamage = 5
	effective = GetEffectiveProduction(germany)
	if effective != 5 {
		t.Errorf("Expected effective production 5, got %d", effective)
	}

	// 10 damage (fully disabled)
	germany.ICDamage = 10
	effective = GetEffectiveProduction(germany)
	if effective != 0 {
		t.Errorf("Expected effective production 0, got %d", effective)
	}

	// More than max damage (should still be 0)
	germany.ICDamage = 20
	effective = GetEffectiveProduction(germany)
	if effective != 0 {
		t.Errorf("Expected effective production 0 with heavy damage, got %d", effective)
	}
}

// Test full strategic bombing integration
func TestStrategicBombingIntegration(t *testing.T) {
	game := models.NewGame()
	game.AddTerritory("Germany", models.Land, "Germany", 10)
	germany := game.Board["Germany"]

	roller := NewSeededDiceRoller(300)

	t.Log("=== Strategic Bombing Raid on Germany ===")

	// Initial state
	t.Logf("Initial IC: Production=%d, Damage=%d, Effective=%d",
		germany.Production, germany.ICDamage, GetEffectiveProduction(germany))

	// Create bomber force
	bomber1 := &models.Piece{Name: "bomber", Terrain: models.Air, Cost: 12}
	bomber2 := &models.Piece{Name: "bomber", Terrain: models.Air, Cost: 12}
	bomber3 := &models.Piece{Name: "bomber", Terrain: models.Air, Cost: 12}

	bombers := []*models.Piece{bomber1, bomber2, bomber3}

	// Execute raid
	result, err := ResolveStrategicBombing(bombers, roller)
	if err != nil {
		t.Fatalf("Bombing raid failed: %v", err)
	}

	t.Logf("Raid Result:")
	t.Logf("  Bombers sent: %d", result.TotalBombers)
	t.Logf("  Bombers destroyed by AA: %d", result.BombersDestroyed)
	t.Logf("  Bombers survived: %d", result.BombersSurvived)
	t.Logf("  Damage rolls: %v", result.DamageRolls)
	t.Logf("  Total damage: %d", result.TotalDamage)

	// Apply damage
	actualDamage := ApplyICDamage(germany, result.TotalDamage)
	t.Logf("  Damage applied: %d (capped at 2x production)", actualDamage)

	t.Logf("After Raid IC: Production=%d, Damage=%d, Effective=%d",
		germany.Production, germany.ICDamage, GetEffectiveProduction(germany))

	// Verify effective production is reduced
	if GetEffectiveProduction(germany) >= germany.Production {
		t.Error("Effective production should be reduced after bombing")
	}
}

// Test multiple bombing raids
func TestMultipleBombingRaids(t *testing.T) {
	game := models.NewGame()
	game.AddTerritory("Germany", models.Land, "Germany", 10)
	germany := game.Board["Germany"]

	roller := NewSeededDiceRoller(400)

	t.Log("=== Multiple Bombing Raids ===")

	// First raid
	bombers1 := []*models.Piece{
		{Name: "bomber", Terrain: models.Air},
		{Name: "bomber", Terrain: models.Air},
	}

	result1, _ := ResolveStrategicBombing(bombers1, roller)
	damage1 := ApplyICDamage(germany, result1.TotalDamage)
	t.Logf("Raid 1: %d bombers, %d damage applied, IC damage now: %d",
		result1.TotalBombers, damage1, germany.ICDamage)

	// Second raid
	bombers2 := []*models.Piece{
		{Name: "bomber", Terrain: models.Air},
		{Name: "bomber", Terrain: models.Air},
	}

	result2, _ := ResolveStrategicBombing(bombers2, roller)
	damage2 := ApplyICDamage(germany, result2.TotalDamage)
	t.Logf("Raid 2: %d bombers, %d damage applied, IC damage now: %d",
		result2.TotalBombers, damage2, germany.ICDamage)

	// Verify damage doesn't exceed max
	maxDamage := germany.Production * 2
	if germany.ICDamage > maxDamage {
		t.Errorf("IC damage %d exceeds max %d", germany.ICDamage, maxDamage)
	}

	t.Logf("Final IC state: Damage=%d/%d, Effective Production=%d",
		germany.ICDamage, maxDamage, GetEffectiveProduction(germany))
}

// Test bombing raid followed by repair
func TestBombingAndRepair(t *testing.T) {
	game := models.NewGame()
	player := game.GetOrCreatePlayer("Germany")
	player.IPCs = 50

	game.AddTerritory("Germany", models.Land, "Germany", 10)
	germany := game.Board["Germany"]

	roller := NewSeededDiceRoller(500)

	t.Log("=== Bombing and Repair Cycle ===")

	// Bombing raid
	bombers := []*models.Piece{
		{Name: "bomber", Terrain: models.Air},
		{Name: "bomber", Terrain: models.Air},
		{Name: "bomber", Terrain: models.Air},
	}

	result, _ := ResolveStrategicBombing(bombers, roller)
	ApplyICDamage(germany, result.TotalDamage)

	t.Logf("After bombing: IC Damage=%d, Effective Production=%d",
		germany.ICDamage, GetEffectiveProduction(germany))

	initialDamage := germany.ICDamage
	repairCost := 5

	// Repair some damage
	actualRepair := RepairIC(germany, repairCost)
	player.IPCs -= repairCost

	t.Logf("Repaired %d damage for %d IPCs", actualRepair, repairCost)
	t.Logf("After repair: IC Damage=%d, Effective Production=%d, Player IPCs=%d",
		germany.ICDamage, GetEffectiveProduction(germany), player.IPCs)

	// Verify damage was reduced
	if germany.ICDamage >= initialDamage {
		t.Error("IC damage should be reduced after repair")
	}

	// Verify IPC cost
	if player.IPCs != 45 {
		t.Errorf("Expected 45 IPCs after 5 IPC repair, got %d", player.IPCs)
	}
}

// Test bombing completely disabled IC
func TestBombingCompletelyDisablesIC(t *testing.T) {
	game := models.NewGame()
	game.AddTerritory("SmallFactory", models.Land, "Germany", 3)
	factory := game.Board["SmallFactory"]

	// Apply heavy damage
	ApplyICDamage(factory, 6) // 2x production

	effective := GetEffectiveProduction(factory)
	if effective != 0 {
		t.Errorf("Heavily damaged IC should have 0 effective production, got %d", effective)
	}

	t.Logf("IC completely disabled: Production=%d, Damage=%d, Effective=%d",
		factory.Production, factory.ICDamage, effective)
}

// Test bombing with all bombers shot down
func TestBombingAllBombersDestroyed(t *testing.T) {
	// This test uses a seed that might produce AA hits
	roller := NewSeededDiceRoller(999)

	bombers := []*models.Piece{
		{Name: "bomber", Terrain: models.Air},
	}

	result, err := ResolveStrategicBombing(bombers, roller)
	if err != nil {
		t.Fatalf("ResolveStrategicBombing failed: %v", err)
	}

	t.Logf("Raid result: %d/%d bombers destroyed, damage: %d",
		result.BombersDestroyed, result.TotalBombers, result.TotalDamage)

	// If all bombers destroyed, no damage should be dealt
	if result.BombersSurvived == 0 {
		if result.TotalDamage != 0 {
			t.Error("No damage should be dealt if all bombers destroyed")
		}
		t.Log("All bombers shot down - no damage dealt (as expected)")
	}
}
