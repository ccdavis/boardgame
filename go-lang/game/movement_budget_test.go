package game

import (
	"testing"

	"boardgame/models"
)

// budgetGame is a short line of territories: A - B - C - D, all owned by one
// power, so movement is never blocked by anything except the allowance.
func budgetGame(t *testing.T) *models.Game {
	t.Helper()

	g := models.NewGame()
	g.PlayerOrder = []string{"Germany", "USSR"}
	for _, name := range g.PlayerOrder {
		player := g.GetOrCreatePlayer(name)
		player.IPCs = 30
		player.TakesTurns = true
	}
	g.Players["Germany"].Side = "Axis"
	g.Players["USSR"].Side = "Allies"

	for _, name := range []string{"A", "B", "C", "D"} {
		g.AddTerritory(name, models.Land, "Germany", 1)
	}
	g.ConnectTerritories("A", "B")
	g.ConnectTerritories("B", "C")
	g.ConnectTerritories("C", "D")

	g.AddPieceTemplate("armor", models.Land, 2, 3, 2, 5)
	return g
}

// A unit that spends its whole allowance attacking must not get it back for the
// noncombat phase. The tracker used to be wiped between the two phases, so
// every unit moved twice as far as the rules allow, every turn.
func TestMovementAllowanceIsNotRefundedBetweenPhases(t *testing.T) {
	g := budgetGame(t)
	controller := NewGameController(g)
	if err := controller.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	if err := g.PlacePieces("A", "armor", 1); err != nil {
		t.Fatalf("placing armor: %v", err)
	}
	armorID := g.NextPieceID - 1

	// Combat move: spend both movement points going A -> C.
	g.CurrentPhase = models.CombatMovePhase
	if err := controller.PlanMove(armorID, "A", "C"); err != nil {
		t.Fatalf("planning A->C: %v", err)
	}
	if err := controller.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing combat moves: %v", err)
	}

	// Noncombat move: the allowance is gone, so C -> D must be refused.
	g.CurrentPhase = models.NoncombatMovePhase
	if err := controller.PlanMove(armorID, "C", "D"); err == nil {
		t.Error("unit moved again after spending its full allowance in the combat phase")
	}
}

// The same accounting must allow a two-movement unit to make two one-space
// moves within a phase, which the old has-this-piece-moved flag forbade.
func TestMovementAllowanceAllowsTwoSingleSteps(t *testing.T) {
	g := budgetGame(t)
	controller := NewGameController(g)
	if err := controller.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	if err := g.PlacePieces("A", "armor", 1); err != nil {
		t.Fatalf("placing armor: %v", err)
	}
	armorID := g.NextPieceID - 1

	g.CurrentPhase = models.CombatMovePhase
	if err := controller.PlanMove(armorID, "A", "B"); err != nil {
		t.Fatalf("planning A->B: %v", err)
	}
	if err := controller.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing: %v", err)
	}

	g.CurrentPhase = models.NoncombatMovePhase
	if err := controller.PlanMove(armorID, "B", "C"); err != nil {
		t.Errorf("a 2-movement unit should be able to take a second single step: %v", err)
	}
}

// Movement is per turn, so the allowance comes back when the turn does.
func TestMovementAllowanceResetsOnNewTurn(t *testing.T) {
	g := budgetGame(t)
	controller := NewGameController(g)
	if err := controller.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	if err := g.PlacePieces("A", "armor", 1); err != nil {
		t.Fatalf("placing armor: %v", err)
	}
	armorID := g.NextPieceID - 1

	g.CurrentPhase = models.CombatMovePhase
	if err := controller.PlanMove(armorID, "A", "C"); err != nil {
		t.Fatalf("planning A->C: %v", err)
	}
	if err := controller.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing: %v", err)
	}

	if err := controller.AdvanceTurn(); err != nil {
		t.Fatalf("advancing turn: %v", err)
	}
	if left := controller.MoveTracker.Remaining(armorID, 2); left != 2 {
		t.Errorf("after a new turn the unit has %d movement left, want 2", left)
	}
}

// Neutral owns territory but is not a playing power, so the turn order must
// step over it rather than handing it a turn.
func TestTurnOrderSkipsNonPlayingPowers(t *testing.T) {
	g := budgetGame(t)
	neutral := g.GetOrCreatePlayer("Neutral")
	neutral.TakesTurns = false
	g.PlayerOrder = []string{"Germany", "Neutral", "USSR"}

	controller := NewGameController(g)
	if err := controller.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	if g.CurrentPower != "Germany" {
		t.Fatalf("game started with %q, want Germany", g.CurrentPower)
	}

	if err := controller.AdvanceTurn(); err != nil {
		t.Fatalf("advancing turn: %v", err)
	}
	if g.CurrentPower == "Neutral" {
		t.Fatal("Neutral was given a turn")
	}
	if g.CurrentPower != "USSR" {
		t.Errorf("turn passed to %q, want USSR", g.CurrentPower)
	}
}
