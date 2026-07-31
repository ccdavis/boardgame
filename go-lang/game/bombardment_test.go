package game

import (
	"testing"

	"boardgame/models"
)

func bombardier(name string, attack int16) *models.Piece {
	return &models.Piece{Name: name, Attack: attack, Defend: attack, Cost: 24, Terrain: models.Water}
}

// A seed whose first roll is at most the threshold, so a bombardment shot hits.
func seedRollingAtMost(t *testing.T, threshold int) int64 {
	t.Helper()
	for s := int64(1); s < 500; s++ {
		if NewSeededDiceRoller(s).Roll() <= threshold {
			return s
		}
	}
	t.Fatal("no such seed in 500 tries")
	return 0
}

func seedRollingAbove(t *testing.T, threshold int) int64 {
	t.Helper()
	for s := int64(1); s < 500; s++ {
		if NewSeededDiceRoller(s).Roll() > threshold {
			return s
		}
	}
	t.Fatal("no such seed in 500 tries")
	return 0
}

// Shore bombardment softens the defence before the first round, and its
// casualties do not fire back (the Classic rule this board follows).
func TestBombardment_KillsBeforeTheFirstRound(t *testing.T) {
	battle := NewBattle("Beach", LandBattle, "Germany", "UK")
	battle.Attackers = []*models.Piece{
		{Name: "infantry", Attack: 1, Defend: 2, Cost: 3, Terrain: models.Land},
	}
	battle.Defenders = []*models.Piece{
		{Name: "infantry", Attack: 1, Defend: 2, Cost: 3, Terrain: models.Land},
	}
	battle.Bombarding = []*models.Piece{bombardier("battleship", 4)}
	battle.AmphibiousUnits = 1

	// First roll <= 4: the battleship's shot lands, the sole defender dies
	// before rolling a die, and the landing wins without a combat round.
	result, err := ResolveCombat(battle, NewSeededDiceRoller(seedRollingAtMost(t, 4)), 10)
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if len(result.BombardmentHits) != 1 {
		t.Fatalf("bombardment hits = %d, want 1", len(result.BombardmentHits))
	}
	if !result.AttackerWins || result.Rounds != 0 {
		t.Errorf("want the beach taken without a round of combat; got rounds=%d, attackerWins=%v",
			result.Rounds, result.AttackerWins)
	}
	if len(result.AttackerCasualties) != 0 {
		t.Error("a defender killed by bombardment fired back")
	}
	if len(result.DefenderCasualties) != 1 {
		t.Errorf("defender casualties = %d, want 1", len(result.DefenderCasualties))
	}
}

// One supporting shot per unit offloaded: three battleships covering a single
// landed infantry fire once, not three times.
func TestBombardment_CappedByUnitsOffloaded(t *testing.T) {
	battle := NewBattle("Beach", LandBattle, "Germany", "UK")
	battle.Attackers = []*models.Piece{
		{Name: "infantry", Attack: 1, Defend: 2, Cost: 3, Terrain: models.Land},
	}
	battle.Defenders = []*models.Piece{
		{Name: "infantry", Attack: 1, Defend: 2, Cost: 3, Terrain: models.Land},
		{Name: "infantry", Attack: 1, Defend: 2, Cost: 3, Terrain: models.Land},
		{Name: "infantry", Attack: 1, Defend: 2, Cost: 3, Terrain: models.Land},
	}
	battle.Bombarding = []*models.Piece{
		bombardier("battleship", 4), bombardier("battleship", 4), bombardier("battleship", 4),
	}
	battle.AmphibiousUnits = 1

	// Whatever the dice say, at most one shot may be taken.
	result, err := ResolveCombat(battle, NewSeededDiceRoller(seedRollingAtMost(t, 4)), 10)
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if len(result.BombardmentHits) > 1 {
		t.Errorf("three battleships covering one landed unit scored %d hits; the cap is 1",
			len(result.BombardmentHits))
	}
}

// A barrage that misses changes nothing.
func TestBombardment_MissLeavesTheDefenceIntact(t *testing.T) {
	battle := NewBattle("Beach", LandBattle, "Germany", "UK")
	battle.Attackers = []*models.Piece{
		{Name: "infantry", Attack: 1, Defend: 2, Cost: 3, Terrain: models.Land},
	}
	battle.Defenders = []*models.Piece{
		{Name: "infantry", Attack: 1, Defend: 2, Cost: 3, Terrain: models.Land},
	}
	// A "battleship" that only hits on a 1, and a seed that opens above 1.
	battle.Bombarding = []*models.Piece{bombardier("battleship", 1)}
	battle.AmphibiousUnits = 1

	result, err := ResolveCombat(battle, NewSeededDiceRoller(seedRollingAbove(t, 1)), 10)
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if len(result.BombardmentHits) != 0 {
		t.Fatalf("the barrage was supposed to miss, scored %d", len(result.BombardmentHits))
	}
	if result.Rounds == 0 {
		t.Error("with the barrage missing, the battle should have been fought")
	}
}

// The landing flow itself enrols the battleships standing off the beach --
// and only ships that can actually bombard, belonging to the attacker.
func TestLanding_AttachesShoreBombardment(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]
	g.AddPieceTemplate("battleship", models.Water, 2, 4, 4, 24)

	// Defenders on the island, so the landing creates a battle.
	if err := g.PlacePieces("Island", "infantry", 2); err != nil {
		t.Fatalf("garrisoning island: %v", err)
	}

	// The convoy sits in the drop zone: a loaded transport, a battleship, a
	// destroyer (escort, but no bombardment), and an enemy-flagged battleship
	// that must not fire for us.
	for _, kind := range []string{"transport", "battleship", "destroyer", "battleship"} {
		if err := g.PlacePieces("Island Sea", kind, 1); err != nil {
			t.Fatalf("placing %s: %v", kind, err)
		}
	}
	pieces := g.Board["Island Sea"].Pieces
	transportID, ourBattleship, destroyerID, theirBattleship := pieces[0], pieces[1], pieces[2], pieces[3]
	for _, id := range []int{transportID, ourBattleship, destroyerID} {
		g.Pieces[id].Owner = player
	}
	g.Pieces[theirBattleship].Owner = g.Players["UK"]

	// A troop aboard, and a plan about to land it.
	if err := g.PlacePieces("Home", "infantry", 1); err != nil {
		t.Fatalf("placing troop: %v", err)
	}
	troopID := g.Board["Home"].Pieces[0]
	g.Board["Home"].Pieces = nil
	g.Pieces[transportID].Holding = []int{troopID}

	plan := controller.Plans.Add(&AmphibiousPlan{
		Power: "Germany", Target: "Island", Staging: "Home",
		Embark: "Home Sea", DropZone: "Island Sea", State: PlanReady,
	})
	plan.Ships = []int{transportID}
	plan.pendingLanding = []int{transportID}

	g.CurrentPhase = models.CombatMovePhase
	if landed := controller.LandAssaultTroops("Germany", NewGameTranscript("t")); landed != 1 {
		t.Fatalf("landed %d troops, want 1", landed)
	}

	battle, ok := controller.PendingBattles["Island"]
	if !ok {
		t.Fatal("no battle created for the landing")
	}
	if battle.AmphibiousUnits != 1 {
		t.Errorf("amphibious units = %d, want 1", battle.AmphibiousUnits)
	}
	if len(battle.Bombarding) != 1 {
		t.Fatalf("bombarding ships = %d, want exactly our battleship", len(battle.Bombarding))
	}
	if got := battle.Bombarding[0].ID; got != ourBattleship {
		t.Errorf("bombarding ship is piece %d, want our battleship %d", got, ourBattleship)
	}
}

// Sea combat in the drop zone forfeits the bombardment: a fleet in action
// cannot also shell the beach.
func TestLanding_NoBombardmentWhileTheDropZoneIsContested(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]
	g.AddPieceTemplate("battleship", models.Water, 2, 4, 4, 24)

	if err := g.PlacePieces("Island", "infantry", 1); err != nil {
		t.Fatalf("garrisoning island: %v", err)
	}
	for _, kind := range []string{"transport", "battleship"} {
		if err := g.PlacePieces("Island Sea", kind, 1); err != nil {
			t.Fatalf("placing %s: %v", kind, err)
		}
	}
	pieces := g.Board["Island Sea"].Pieces
	transportID, battleshipID := pieces[0], pieces[1]
	g.Pieces[transportID].Owner = player
	g.Pieces[battleshipID].Owner = player

	if err := g.PlacePieces("Home", "infantry", 1); err != nil {
		t.Fatalf("placing troop: %v", err)
	}
	troopID := g.Board["Home"].Pieces[0]
	g.Board["Home"].Pieces = nil
	g.Pieces[transportID].Holding = []int{troopID}

	plan := controller.Plans.Add(&AmphibiousPlan{
		Power: "Germany", Target: "Island", Staging: "Home",
		Embark: "Home Sea", DropZone: "Island Sea", State: PlanReady,
	})
	plan.Ships = []int{transportID}
	plan.pendingLanding = []int{transportID}

	// A sea battle is already pending in the drop zone.
	controller.PendingBattles["Island Sea"] = NewBattle("Island Sea", SeaBattle, "Germany", "UK")

	g.CurrentPhase = models.CombatMovePhase
	controller.LandAssaultTroops("Germany", NewGameTranscript("t"))

	if battle, ok := controller.PendingBattles["Island"]; ok && len(battle.Bombarding) > 0 {
		t.Errorf("%d ship(s) bombarding while their sea zone is being fought over", len(battle.Bombarding))
	}
}
