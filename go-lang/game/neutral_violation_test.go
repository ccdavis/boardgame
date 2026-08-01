package game

import (
	"testing"

	"boardgame/models"
)

// A board with a strict neutral between the powers, and a second strict
// neutral off to the side to watch the chain reaction.
func neutralBoard(t *testing.T) (*models.Game, *GameController) {
	t.Helper()

	g := models.NewGame()
	g.PlayerOrder = []string{"Germany", "UK"}
	germany := g.GetOrCreatePlayer("Germany")
	uk := g.GetOrCreatePlayer("UK")
	germany.Side, uk.Side = "Axis", "Allies"
	germany.TakesTurns, uk.TakesTurns = true, true

	g.AddTerritory("Reich", models.Land, "Germany", 4)
	g.AddTerritory("Turkey", models.Land, "Neutral", 4)   // strict by default
	g.AddTerritory("Mongolia", models.Land, "Neutral", 1) // strict by default
	g.AddTerritory("Albion", models.Land, "UK", 4)
	g.ConnectTerritories("Reich", "Turkey")
	g.ConnectTerritories("Turkey", "Albion")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("armor", models.Land, 2, 3, 2, 5)
	g.PlacePieces("Reich", "armor", 3)

	gc := NewGameController(g)
	if err := gc.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	g.CurrentPower = "Germany"
	g.CurrentPhase = models.CombatMovePhase
	return g, gc
}

// Violating a strict neutral: allowed, but the attacker pays 3 IPCs, the
// country raises a garrison, and every other strict neutral turns hostile.
func TestStrictNeutral_ViolationCostsAndRaisesTheCountry(t *testing.T) {
	g, gc := neutralBoard(t)
	germany := g.Players["Germany"]
	germany.IPCs = 10

	tank := g.Board["Reich"].Pieces[0]
	if err := gc.PlanMove(tank, "Reich", "Turkey"); err != nil {
		t.Fatalf("attacking a strict neutral must be possible: %v", err)
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing: %v", err)
	}

	if germany.IPCs != 10-NeutralViolationCost {
		t.Errorf("treasury %d after the violation, want %d", germany.IPCs, 10-NeutralViolationCost)
	}

	// Turkey mobilised: production 4 -> 4 infantry, plus the invading tank.
	defenders := 0
	for _, id := range g.Board["Turkey"].Pieces {
		if piece := g.Pieces[id]; piece != nil && piece.Name == "infantry" {
			defenders++
		}
	}
	if defenders != 4 {
		t.Errorf("Turkey raised %d infantry, want 4 (one per point of production)", defenders)
	}
	if _, ok := gc.PendingBattles["Turkey"]; !ok {
		t.Error("no battle was created for the invasion")
	}

	// The chain reaction: Mongolia joins the Allies with a garrison; Turkey,
	// mid-battle, still defends under its own flag.
	if owner := g.Board["Mongolia"].Owner.Name; owner != "UK" {
		t.Errorf("Mongolia belongs to %s after the violation, want UK", owner)
	}
	if len(g.Board["Mongolia"].Pieces) == 0 {
		t.Error("Mongolia turned hostile without a garrison")
	}
	if owner := g.Board["Turkey"].Owner.Name; owner != "Neutral" {
		t.Errorf("Turkey's flag changed to %s mid-battle; it should defend as a neutral", owner)
	}
}

// Too poor to pay the toll: the violation is refused at planning time.
func TestStrictNeutral_TooPoorToViolate(t *testing.T) {
	g, gc := neutralBoard(t)
	g.Players["Germany"].IPCs = NeutralViolationCost - 1

	tank := g.Board["Reich"].Pieces[0]
	if err := gc.PlanMove(tank, "Reich", "Turkey"); err == nil {
		t.Fatal("a power that cannot pay the violation cost was allowed to attack a strict neutral")
	}
}

// Two attackers, one toll, one garrison: the country mobilises once.
func TestStrictNeutral_ViolatedOnceHoweverManyAttackersArrive(t *testing.T) {
	g, gc := neutralBoard(t)
	germany := g.Players["Germany"]
	germany.IPCs = 10

	pieces := append([]int{}, g.Board["Reich"].Pieces...)
	for _, tank := range pieces[:2] {
		if err := gc.PlanMove(tank, "Reich", "Turkey"); err != nil {
			t.Fatalf("planning: %v", err)
		}
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing: %v", err)
	}

	if germany.IPCs != 10-NeutralViolationCost {
		t.Errorf("treasury %d, want the toll paid once, not per attacker", germany.IPCs)
	}
	infantry := 0
	for _, id := range g.Board["Turkey"].Pieces {
		if piece := g.Pieces[id]; piece != nil && piece.Name == "infantry" {
			infantry++
		}
	}
	if infantry != 4 {
		t.Errorf("garrison of %d, want 4 -- the country mobilises once", infantry)
	}
}

// A pro-side neutral defends itself too when invaded (no toll, no chain).
func TestProNeutral_RaisesAGarrisonWhenAttacked(t *testing.T) {
	g, gc := neutralBoard(t)
	germany := g.Players["Germany"]
	germany.IPCs = 10
	g.Board["Turkey"].NeutralType = models.ProAlliedNeutral

	tank := g.Board["Reich"].Pieces[0]
	if err := gc.PlanMove(tank, "Reich", "Turkey"); err != nil {
		t.Fatalf("Axis attacking a pro-Allied neutral: %v", err)
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing: %v", err)
	}

	if germany.IPCs != 10 {
		t.Errorf("treasury %d; only strict neutrals levy the violation toll", germany.IPCs)
	}
	infantry := 0
	for _, id := range g.Board["Turkey"].Pieces {
		if piece := g.Pieces[id]; piece != nil && piece.Name == "infantry" {
			infantry++
		}
	}
	if infantry != 4 {
		t.Errorf("garrison of %d, want 4", infantry)
	}
	if owner := g.Board["Mongolia"].Owner.Name; owner != "Neutral" {
		t.Errorf("attacking a pro-side neutral flipped Mongolia to %s; the chain is for strict neutrals", owner)
	}
}

// Two strict neutrals may not be booked on one treasury that covers only one
// toll: every violation planned in a phase must be payable together.
func TestNeutralTolls_CumulativeAcrossBookings(t *testing.T) {
	g := models.NewGame()
	g.AddTerritory("Reich", models.Land, "Germany", 5)
	g.AddTerritory("Alpinia", models.Land, "Neutral", 1)
	g.AddTerritory("Anatolia", models.Land, "Neutral", 1)
	g.ConnectTerritories("Reich", "Alpinia")
	g.ConnectTerritories("Alpinia", "Reich")
	g.ConnectTerritories("Reich", "Anatolia")
	g.ConnectTerritories("Anatolia", "Reich")
	g.Board["Alpinia"].NeutralType = models.StrictNeutral
	g.Board["Anatolia"].NeutralType = models.StrictNeutral

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.PlacePieces("Reich", "infantry", 2)

	g.PlayerOrder = []string{"Germany"}
	g.CurrentPower = "Germany"
	g.CurrentPhase = models.CombatMovePhase
	germany := g.Players["Germany"]
	germany.Side = "Axis"
	germany.IPCs = NeutralViolationCost + 1 // covers one toll, not two

	gc := NewGameController(g)
	pieces := g.Board["Reich"].Pieces

	if err := gc.PlanMove(pieces[0], "Reich", "Alpinia"); err != nil {
		t.Fatalf("first violation should be affordable: %v", err)
	}
	if err := gc.PlanMove(pieces[1], "Reich", "Anatolia"); err == nil {
		t.Fatal("booked a second strict-neutral violation the treasury cannot cover")
	}

	// With funds for both, both book.
	germany.IPCs = 2 * NeutralViolationCost
	if err := gc.PlanMove(pieces[1], "Reich", "Anatolia"); err != nil {
		t.Fatalf("two tolls, two violations, funds for both: %v", err)
	}
}
