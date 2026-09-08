package game

import (
	"sort"
	"testing"

	"boardgame/models"
)

// liveBattleGame stages a land battle: Germany attacks Moscow with armor and
// infantry from Germany; the USSR defends with infantry.
func liveBattleGame(t *testing.T, attackers map[string]int, defenders map[string]int) (*models.Game, *GameController) {
	t.Helper()
	g := createTestGame()
	g.ConnectTerritories("Germany", "Moscow")
	g.ConnectTerritories("Moscow", "Germany")
	for name, capital := range map[string]string{
		"USSR": "Moscow", "Germany": "Germany", "UK": "London", "Japan": "Tokyo", "USA": "Washington",
	} {
		g.Players[name].Capital = capital
	}
	c := NewGameController(g)
	c.StartGame()
	c.Dice = NewSeededDiceRoller(7)

	// Placed in a fixed order: map iteration would shuffle piece IDs and
	// with them the dice, making the seeded fights differ run to run.
	for _, name := range sortedKeys(defenders) {
		if err := g.PlacePieces("Moscow", name, defenders[name]); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range sortedKeys(attackers) {
		if err := g.PlacePieces("Germany", name, attackers[name]); err != nil {
			t.Fatal(err)
		}
	}
	g.CurrentPower = "Germany"
	g.CurrentPhase = models.CombatMovePhase
	for _, id := range append([]int{}, g.Board["Germany"].Pieces...) {
		if err := c.PlanMove(id, "Germany", "Moscow"); err != nil {
			t.Fatalf("planning attack: %v", err)
		}
	}
	if err := c.ExecuteCombatMoves(); err != nil {
		t.Fatal(err)
	}
	g.CurrentPhase = models.ConductCombatPhase
	if _, ok := c.PendingBattles["Moscow"]; !ok {
		t.Fatal("no battle was staged in Moscow")
	}
	return g, c
}

// A battle fought round by round ends in the same kind of state as one
// resolved at once: casualties gone, the ground captured by a winner with
// troops standing, nothing pending, and the board valid.
func TestLiveBattle_RoundsToConclusion(t *testing.T) {
	g, c := liveBattleGame(t, map[string]int{"armor": 4, "infantry": 4}, map[string]int{"infantry": 2})

	live, err := c.BeginBattle("Moscow")
	if err != nil {
		t.Fatal(err)
	}
	if live.Round != 0 || live.Done {
		t.Fatalf("a fresh live battle should be at round 0 and open, got round %d done=%v", live.Round, live.Done)
	}
	for rounds := 0; !live.Done && rounds < 50; rounds++ {
		if live.PendingAttackerHits > 0 {
			// Assign every owed hit to the cheapest units, one each.
			hits := map[int]int{}
			left := live.PendingAttackerHits
			for _, unit := range live.Attackers {
				if left == 0 {
					break
				}
				if unit.Name == "infantry" {
					hits[unit.ID] = 1
					left--
				}
			}
			if left > 0 {
				for _, unit := range live.Attackers {
					if left == 0 {
						break
					}
					if hits[unit.ID] == 0 {
						hits[unit.ID] = 1
						left--
					}
				}
			}
			if _, err := c.AssignCasualties("Moscow", hits); err != nil {
				t.Fatalf("assigning casualties: %v", err)
			}
			continue
		}
		if _, err := c.FightRound("Moscow"); err != nil {
			t.Fatalf("round: %v", err)
		}
	}
	if !live.Done || live.Result == nil {
		t.Fatal("battle never concluded")
	}
	if _, still := c.PendingBattles["Moscow"]; still {
		t.Error("battle still pending after conclusion")
	}
	if _, still := c.LiveBattles["Moscow"]; still {
		t.Error("live battle still registered after conclusion")
	}
	if live.Result.AttackerWins && g.Board["Moscow"].Owner.Name != "Germany" {
		t.Error("attacker won with troops standing but Moscow was not captured")
	}
	if !live.Result.AttackerWins && g.Board["Moscow"].Owner.Name != "USSR" {
		t.Error("attacker lost but Moscow changed hands")
	}
	for _, casualty := range append(live.Result.AttackerCasualties, live.Result.DefenderCasualties...) {
		if _, alive := g.Pieces[casualty.ID]; alive {
			t.Errorf("casualty %s %d is still on the board", casualty.Name, casualty.ID)
		}
	}
	if problems := g.Validate(); len(problems) > 0 {
		t.Errorf("board invalid after live battle: %v", problems)
	}
}

// The attacker may withdraw between rounds; survivors go home and the
// defenders keep the ground.
func TestLiveBattle_RetreatReturnsSurvivors(t *testing.T) {
	g, c := liveBattleGame(t, map[string]int{"infantry": 3}, map[string]int{"infantry": 6})

	live, err := c.BeginBattle("Moscow")
	if err != nil {
		t.Fatal(err)
	}
	if live.CanRetreat() {
		t.Error("retreat should not be offered before the first round")
	}
	if _, err := c.FightRound("Moscow"); err != nil {
		t.Fatal(err)
	}
	if live.PendingAttackerHits > 0 {
		if _, err := c.FinishBattle("Moscow"); err != nil {
			t.Fatal(err)
		}
		return // the dice ended it; nothing to retreat with in this seed
	}
	if live.Done {
		return
	}
	if !live.CanRetreat() {
		t.Fatal("retreat should be offered after a round with survivors")
	}
	before := len(live.Attackers)
	if _, err := c.Retreat("Moscow"); err != nil {
		t.Fatal(err)
	}
	if !live.Done || !live.Result.AttackerRetreated {
		t.Error("retreat with every unit able to withdraw should end the battle as a withdrawal")
	}
	if g.Board["Moscow"].Owner.Name != "USSR" {
		t.Error("a withdrawal must not capture")
	}
	home := 0
	for _, id := range g.Board["Germany"].Pieces {
		if g.Pieces[id].Name == "infantry" {
			home++
		}
	}
	if home != before {
		t.Errorf("%d survivors withdrew but %d are home", before, home)
	}
	if problems := g.Validate(); len(problems) > 0 {
		t.Errorf("board invalid after retreat: %v", problems)
	}
}

// Casualty assignment must account for exactly the hits owed, and a
// two-hit unit may soak one and stay in the fight.
func TestLiveBattle_CasualtyAssignmentIsChecked(t *testing.T) {
	g := createTestGame()
	g.AddPieceTemplate("battleship", models.Water, 2, 4, 4, 24)
	g.AddPieceTemplate("sub", models.Water, 2, 2, 2, 8)
	g.AddTerritory("Home Sea", models.Water, "Germany", 0)
	g.AddTerritory("Far Sea", models.Water, "USSR", 0)
	g.ConnectTerritories("Home Sea", "Far Sea")
	g.ConnectTerritories("Far Sea", "Home Sea")
	for name, capital := range map[string]string{
		"USSR": "Moscow", "Germany": "Germany", "UK": "London", "Japan": "Tokyo", "USA": "Washington",
	} {
		g.Players[name].Capital = capital
	}
	c := NewGameController(g)
	c.StartGame()
	c.Dice = NewSeededDiceRoller(3)

	g.PlacePieces("Far Sea", "battleship", 2)
	for _, id := range g.Board["Far Sea"].Pieces {
		g.Pieces[id].Owner = g.Players["USSR"]
	}
	g.PlacePieces("Home Sea", "battleship", 1)
	g.PlacePieces("Home Sea", "sub", 2)
	for _, id := range g.Board["Home Sea"].Pieces {
		g.Pieces[id].Owner = g.Players["Germany"]
	}
	g.CurrentPower = "Germany"
	g.CurrentPhase = models.CombatMovePhase
	for _, id := range append([]int{}, g.Board["Home Sea"].Pieces...) {
		if err := c.PlanMove(id, "Home Sea", "Far Sea"); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.ExecuteCombatMoves(); err != nil {
		t.Fatal(err)
	}
	g.CurrentPhase = models.ConductCombatPhase

	live, err := c.BeginBattle("Far Sea")
	if err != nil {
		t.Fatal(err)
	}
	if !live.CanSubmerge() {
		t.Error("with no defending destroyer the submarines should be able to submerge")
	}
	for !live.Done && live.PendingAttackerHits == 0 {
		if _, err := c.FightRound("Far Sea"); err != nil {
			t.Fatal(err)
		}
	}
	if live.Done {
		t.Skip("the dice decided the fight before any casualties were owed")
	}
	owed := live.PendingAttackerHits
	if _, err := c.AssignCasualties("Far Sea", map[int]int{}); err == nil {
		t.Error("assigning nothing when hits are owed should be refused")
	}
	// Put every owed hit on the battleship if it can take them: it soaks
	// the first and dies on the second.
	var battleship *models.Piece
	for _, unit := range live.Attackers {
		if unit.Name == "battleship" {
			battleship = unit
		}
	}
	if battleship != nil && owed <= 2 {
		if _, err := c.AssignCasualties("Far Sea", map[int]int{battleship.ID: owed}); err != nil {
			t.Fatalf("assigning %d hit(s) to the battleship: %v", owed, err)
		}
		if owed == 1 {
			if _, alive := g.Pieces[battleship.ID]; !alive || battleship.Hits != 1 {
				t.Error("a battleship taking one hit should be damaged, not sunk")
			}
		}
	} else {
		if _, err := c.FinishBattle("Far Sea"); err != nil {
			t.Fatal(err)
		}
	}
	if problems := g.Validate(); len(problems) > 0 {
		t.Errorf("board invalid: %v", problems)
	}
}

// Resolving a battle that is already under way finishes it from where it
// stands rather than starting again.
func TestLiveBattle_ResolveFinishesFromWhereItStands(t *testing.T) {
	_, c := liveBattleGame(t, map[string]int{"armor": 3, "infantry": 3}, map[string]int{"infantry": 3})
	live, err := c.BeginBattle("Moscow")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.FightRound("Moscow"); err != nil {
		t.Fatal(err)
	}
	if live.Done {
		t.Skip("the first round decided the fight; nothing left to resolve")
	}
	result, err := c.ResolveBattle("Moscow", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !live.Done || result != live.Result {
		t.Error("ResolveBattle should conclude the live battle and hand back its result")
	}
	if result.Rounds < 1 {
		t.Errorf("the round already fought was forgotten: %d rounds", result.Rounds)
	}
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
