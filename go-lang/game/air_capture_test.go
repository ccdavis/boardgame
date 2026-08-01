package game

import (
	"testing"

	"boardgame/models"
)

// setupAirTest builds a small board: Germany holds Berlin (with a fighter, a
// bomber and infantry available for tests), the UK holds London defended by a
// militia that cannot hit back, so battles resolve the same way every run.
func setupAirTest(t *testing.T) (*GameController, *models.Game) {
	t.Helper()
	g := models.NewGame()

	g.AddTerritory("Berlin", models.Land, "Germany", 5)
	g.AddTerritory("London", models.Land, "UK", 5)
	g.AddTerritory("North Sea", models.Water, "Neutral", 0)
	g.ConnectTerritories("Berlin", "London")
	g.ConnectTerritories("London", "Berlin")
	g.ConnectTerritories("Berlin", "North Sea")
	g.ConnectTerritories("North Sea", "Berlin")
	g.ConnectTerritories("North Sea", "London")
	g.ConnectTerritories("London", "North Sea")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("militia", models.Land, 1, 0, 0, 1)
	g.AddPieceTemplate("fighter", models.Air, 4, 3, 4, 10)
	g.AddPieceTemplate("bomber", models.Air, 6, 4, 1, 12)
	g.AddPieceTemplate("carrier", models.Water, 2, 1, 2, 14)
	g.SetContainerCapacity("carrier", 2, []string{"fighter"})

	g.PlayerOrder = []string{"Germany", "UK"}
	g.CurrentPower = "Germany"
	g.CurrentPhase = models.CombatMovePhase
	if germany, ok := g.Players["Germany"]; ok {
		germany.Side = "Axis"
	}
	if uk, ok := g.Players["UK"]; ok {
		uk.Side = "Allies"
	}
	return NewGameController(g), g
}

func pieceIn(t *testing.T, g *models.Game, territory, name string) int {
	t.Helper()
	for _, id := range g.Board[territory].Pieces {
		if g.Pieces[id].Name == name {
			return id
		}
	}
	t.Fatalf("no %s in %s", name, territory)
	return 0
}

// A fighter that wins alone does not take the ground: air cannot capture.
func TestAirOnlyVictoryDoesNotCapture(t *testing.T) {
	gc, g := setupAirTest(t)
	g.PlacePieces("Berlin", "fighter", 1)
	g.PlacePieces("London", "militia", 1)

	fighter := pieceIn(t, g, "Berlin", "fighter")
	if err := gc.PlanMove(fighter, "Berlin", "London"); err != nil {
		t.Fatalf("planning fighter attack: %v", err)
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing combat moves: %v", err)
	}
	gc.Game.CurrentPhase = models.ConductCombatPhase

	result, err := gc.ResolveBattle("London", NewSeededDiceRoller(1))
	if err != nil {
		t.Fatalf("resolving battle: %v", err)
	}
	if !result.AttackerWins {
		t.Fatalf("expected the fighter to win against a militia that cannot hit")
	}
	if g.Board["London"].Owner.Name != "UK" {
		t.Errorf("London owner = %s; an air-only victory must not capture territory",
			g.Board["London"].Owner.Name)
	}
}

// The same fight with infantry along captures normally.
func TestLandUnitVictoryCaptures(t *testing.T) {
	gc, g := setupAirTest(t)
	g.PlacePieces("Berlin", "fighter", 1)
	g.PlacePieces("Berlin", "infantry", 1)
	g.PlacePieces("London", "militia", 1)

	for _, name := range []string{"fighter", "infantry"} {
		id := pieceIn(t, g, "Berlin", name)
		if err := gc.PlanMove(id, "Berlin", "London"); err != nil {
			t.Fatalf("planning %s attack: %v", name, err)
		}
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing combat moves: %v", err)
	}
	gc.Game.CurrentPhase = models.ConductCombatPhase

	result, err := gc.ResolveBattle("London", NewSeededDiceRoller(1))
	if err != nil {
		t.Fatalf("resolving battle: %v", err)
	}
	if !result.AttackerWins {
		t.Fatalf("expected attackers to win against a militia that cannot hit")
	}
	if g.Board["London"].Owner.Name != "Germany" {
		t.Errorf("London owner = %s, want Germany once infantry survives the win",
			g.Board["London"].Owner.Name)
	}
}

// An aircraft still sitting in hostile territory when the turn passes is lost.
func TestStrandedFighterCrashesAtTurnEnd(t *testing.T) {
	gc, g := setupAirTest(t)
	g.PlacePieces("Berlin", "fighter", 1)
	g.PlacePieces("London", "militia", 1)

	fighter := pieceIn(t, g, "Berlin", "fighter")
	if err := gc.PlanMove(fighter, "Berlin", "London"); err != nil {
		t.Fatalf("planning fighter attack: %v", err)
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing combat moves: %v", err)
	}
	gc.Game.CurrentPhase = models.ConductCombatPhase
	if _, err := gc.ResolveBattle("London", NewSeededDiceRoller(1)); err != nil {
		t.Fatalf("resolving battle: %v", err)
	}

	// The player never flies the fighter home; the turn ends.
	gc.Game.CurrentPhase = models.CollectIncomePhase
	if err := gc.AdvancePhase(); err != nil {
		t.Fatalf("advancing turn: %v", err)
	}

	if _, alive := g.Pieces[fighter]; alive {
		t.Errorf("fighter %d survived the turn stranded in hostile London", fighter)
	}
}

// A fighter parked at sea keeps flying only if a friendly carrier has a seat.
func TestCarrierSeatingAtTurnEnd(t *testing.T) {
	gc, g := setupAirTest(t)
	g.PlacePieces("North Sea", "carrier", 1)
	g.PlacePieces("Berlin", "fighter", 1)
	g.PlacePieces("Berlin", "bomber", 1)

	// Own the carrier: PlacePieces assigns by territory owner, and the North
	// Sea belongs to no one, so hand it to Germany explicitly.
	germany := g.Players["Germany"]
	carrier := pieceIn(t, g, "North Sea", "carrier")
	g.Pieces[carrier].Owner = germany

	fighter := pieceIn(t, g, "Berlin", "fighter")
	bomber := pieceIn(t, g, "Berlin", "bomber")
	gc.Game.CurrentPhase = models.NoncombatMovePhase
	if err := gc.PlanMove(fighter, "Berlin", "North Sea"); err != nil {
		t.Fatalf("planning fighter to carrier: %v", err)
	}
	if err := gc.ExecuteNoncombatMoves(); err != nil {
		t.Fatalf("executing noncombat moves: %v", err)
	}
	// The bomber blunders out to sea by hand: no carrier carries bombers.
	berlin, sea := g.Board["Berlin"], g.Board["North Sea"]
	for i, id := range berlin.Pieces {
		if id == bomber {
			berlin.Pieces = append(berlin.Pieces[:i], berlin.Pieces[i+1:]...)
			break
		}
	}
	sea.Pieces = append(sea.Pieces, bomber)

	gc.Game.CurrentPhase = models.CollectIncomePhase
	if err := gc.AdvancePhase(); err != nil {
		t.Fatalf("advancing turn: %v", err)
	}

	if _, alive := g.Pieces[fighter]; !alive {
		t.Errorf("fighter crashed despite a free carrier seat")
	}
	if _, alive := g.Pieces[bomber]; alive {
		t.Errorf("bomber survived at sea; nothing can carry a bomber")
	}
}

// Winning a sea battle clears the zone; it does not annex the ocean.
func TestSeaVictoryDoesNotChangeZoneOwnership(t *testing.T) {
	gc, g := setupAirTest(t)
	g.AddPieceTemplate("destroyer", models.Water, 2, 2, 2, 8)
	g.AddPieceTemplate("barge", models.Water, 1, 0, 0, 2)
	g.PlacePieces("North Sea", "barge", 1)

	germany := g.Players["Germany"]
	uk := g.Players["UK"]
	barge := pieceIn(t, g, "North Sea", "barge")
	g.Pieces[barge].Owner = uk

	// A German destroyer starts in a German-flagged coastal zone.
	g.AddTerritory("Baltic", models.Water, "Germany", 0)
	g.ConnectTerritories("Baltic", "North Sea")
	g.ConnectTerritories("North Sea", "Baltic")
	g.PlacePieces("Baltic", "destroyer", 1)
	destroyer := pieceIn(t, g, "Baltic", "destroyer")
	g.Pieces[destroyer].Owner = germany

	ownerBefore := g.Board["North Sea"].Owner.Name

	if err := gc.PlanMove(destroyer, "Baltic", "North Sea"); err != nil {
		t.Fatalf("planning naval attack: %v", err)
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing combat moves: %v", err)
	}
	gc.Game.CurrentPhase = models.ConductCombatPhase

	result, err := gc.ResolveBattle("North Sea", NewSeededDiceRoller(1))
	if err != nil {
		t.Fatalf("resolving battle: %v", err)
	}
	if !result.AttackerWins {
		t.Fatalf("expected the destroyer to sink a barge that cannot hit")
	}
	if got := g.Board["North Sea"].Owner.Name; got != ownerBefore {
		t.Errorf("North Sea owner changed %s -> %s; sea zones are never captured",
			ownerBefore, got)
	}
}
