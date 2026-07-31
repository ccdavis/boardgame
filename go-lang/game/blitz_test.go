package game

import (
	"testing"

	"boardgame/models"
)

// blitzGame lays out Home - Gap - Target, where Gap and Target belong to the
// enemy, so a tank driving Home -> Target passes through Gap.
func blitzGame(t *testing.T) (*models.Game, *GameController) {
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

	g.AddTerritory("Home", models.Land, "Germany", 3)
	g.AddTerritory("Gap", models.Land, "USSR", 2)
	g.AddTerritory("Target", models.Land, "USSR", 4)
	g.ConnectTerritories("Home", "Gap")
	g.ConnectTerritories("Gap", "Target")

	g.AddPieceTemplate("armor", models.Land, 2, 3, 2, 5)
	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("factory", models.Land, 0, 0, 0, 32)

	controller := NewGameController(g)
	if err := controller.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	g.CurrentPhase = models.CombatMovePhase
	return g, controller
}

func TestBlitz_TakesTheEmptyTerritoryDrivenThrough(t *testing.T) {
	g, controller := blitzGame(t)
	if err := g.PlacePieces("Home", "armor", 1); err != nil {
		t.Fatalf("placing armor: %v", err)
	}
	armorID := g.NextPieceID - 1

	if err := controller.PlanMove(armorID, "Home", "Target"); err != nil {
		t.Fatalf("planning the blitz: %v", err)
	}
	if err := controller.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing: %v", err)
	}

	if owner := g.Board["Gap"].Owner; owner == nil || owner.Name != "Germany" {
		t.Errorf("Gap was driven through but not taken; owner is %v", owner)
	}
	// The tank ends up at its destination, not parked in the territory it took.
	// The old implementation flipped ownership and never moved the tank at all.
	if len(g.Board["Target"].Pieces) != 1 {
		t.Errorf("expected the tank to arrive in Target, found %d pieces", len(g.Board["Target"].Pieces))
	}
}

// A garrisoned territory has to be fought for, so it cannot be blitzed past.
func TestBlitz_DefendedTerritoryBlocksTheDrive(t *testing.T) {
	g, controller := blitzGame(t)
	if err := g.PlacePieces("Home", "armor", 1); err != nil {
		t.Fatalf("placing armor: %v", err)
	}
	armorID := g.NextPieceID - 1
	if err := g.PlacePieces("Gap", "infantry", 1); err != nil {
		t.Fatalf("placing defender: %v", err)
	}

	if err := controller.PlanMove(armorID, "Home", "Target"); err == nil {
		t.Error("blitzed through a defended territory")
	}
}

// Only units with the capability blitz. Infantry has one movement point anyway,
// but the rule should be the capability, not the distance.
func TestBlitz_InfantryCannotBlitz(t *testing.T) {
	g, _ := blitzGame(t)
	infantry := &models.Piece{Name: "infantry", Terrain: models.Land, Movement: 1}
	armor := &models.Piece{Name: "armor", Terrain: models.Land, Movement: 2}
	germany := g.Players["Germany"]

	if canBlitzThrough(g, infantry, g.Board["Gap"], germany) {
		t.Error("infantry should not be able to blitz")
	}
	if !canBlitzThrough(g, armor, g.Board["Gap"], germany) {
		t.Error("armor should be able to blitz an empty enemy territory")
	}
}

// The old check counted every piece present, so a friendly unit standing in the
// territory was enough to forbid the blitz.
func TestBlitz_FriendlyUnitDoesNotBlockTheDrive(t *testing.T) {
	g, _ := blitzGame(t)
	armor := &models.Piece{Name: "armor", Terrain: models.Land, Movement: 2}
	germany := g.Players["Germany"]

	if err := g.PlacePieces("Gap", "infantry", 1); err != nil {
		t.Fatalf("placing unit: %v", err)
	}
	// Make that unit ours rather than the enemy's.
	for _, id := range g.Board["Gap"].Pieces {
		g.Pieces[id].Owner = germany
	}

	if !canBlitzThrough(g, armor, g.Board["Gap"], germany) {
		t.Error("a friendly unit in the territory should not block a blitz")
	}
}

// An undefended factory does not hold a territory.
func TestBlitz_UndefendedStructureDoesNotBlock(t *testing.T) {
	g, _ := blitzGame(t)
	armor := &models.Piece{Name: "armor", Terrain: models.Land, Movement: 2}
	germany := g.Players["Germany"]

	if err := g.PlacePieces("Gap", "factory", 1); err != nil {
		t.Fatalf("placing factory: %v", err)
	}

	if !canBlitzThrough(g, armor, g.Board["Gap"], germany) {
		t.Error("an undefended factory should not block a blitz")
	}
}

func TestBlitz_OwnTerritoryIsNotBlitzed(t *testing.T) {
	g, _ := blitzGame(t)
	armor := &models.Piece{Name: "armor", Terrain: models.Land, Movement: 2}
	germany := g.Players["Germany"]

	if canBlitzThrough(g, armor, g.Board["Home"], germany) {
		t.Error("moving through your own territory is not a blitz")
	}
}
