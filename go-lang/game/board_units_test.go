package game

import (
	"testing"

	"boardgame/models"
)

// boardWithRoster builds a game whose unit roster uses the board's spellings.
func boardWithRoster(t *testing.T) *models.Game {
	t.Helper()

	g := models.NewGame()
	for name, spec := range map[string]struct {
		terrain            models.TerrainType
		move, atk, def, cost int16
	}{
		"sub":        {models.Water, 2, 2, 2, 8},
		"battleship": {models.Water, 2, 4, 4, 24},
		"transport":  {models.Water, 2, 0, 1, 10},
		"infantry":   {models.Land, 1, 1, 2, 3},
		"armor":      {models.Land, 2, 3, 2, 5},
		"fighter":    {models.Air, 4, 3, 4, 12},
		"AAA":        {models.Land, 0, 0, 1, 5},
		"factory":    {models.Land, 0, 0, 0, 32},
	} {
		g.GlobalPieceTemplates[name] = &models.Piece{
			Name: name, Terrain: spec.terrain, Movement: spec.move,
			Attack: spec.atk, Defend: spec.def, Cost: spec.cost,
		}
	}
	SetUnitRegistry(g.Units())
	return g
}

// The board calls the unit "sub". Combat used to look for "submarine", so none
// of the submarine rules ever fired in a real game.
func TestBoardSpelling_SubmarinesAreRecognised(t *testing.T) {
	boardWithRoster(t)

	sub := &models.Piece{Name: "sub", Terrain: models.Water, Attack: 2, Defend: 2}
	battleship := &models.Piece{Name: "battleship", Terrain: models.Water, Attack: 4, Defend: 4}
	units := []*models.Piece{sub, battleship}

	subs := getSubmarines(units)
	if len(subs) != 1 || subs[0] != sub {
		t.Errorf("getSubmarines found %d submarines, want the one named %q", len(subs), "sub")
	}

	others := getNonSubmarines(units)
	if len(others) != 1 || others[0] != battleship {
		t.Errorf("getNonSubmarines returned %d units, want just the battleship", len(others))
	}
}

// A board that declares no destroyer must not report one.
func TestBoardSpelling_NoDestroyerOnThisBoard(t *testing.T) {
	boardWithRoster(t)

	units := []*models.Piece{
		{Name: "sub", Terrain: models.Water},
		{Name: "battleship", Terrain: models.Water},
	}
	if hasDestroyer(units) {
		t.Error("reported a destroyer in a fleet that has none")
	}
}

// ...but a board that does declare one gets the behaviour, with no code change.
func TestBoardSpelling_DestroyerNegatesWhenDeclared(t *testing.T) {
	g := boardWithRoster(t)
	g.GlobalPieceTemplates["destroyer"] = &models.Piece{
		Name: "destroyer", Terrain: models.Water, Movement: 2, Attack: 2, Defend: 2, Cost: 8,
	}
	SetUnitRegistry(models.BuildUnitRegistry(g.GlobalPieceTemplates))

	units := []*models.Piece{
		{Name: "sub", Terrain: models.Water},
		{Name: "destroyer", Terrain: models.Water},
	}
	if !hasDestroyer(units) {
		t.Error("a declared destroyer was not recognised")
	}
}

func TestBoardSpelling_AntiAircraftAndStructures(t *testing.T) {
	g := boardWithRoster(t)
	registry := g.Units()

	if !registry.Of("AAA").IsAA {
		t.Error(`the board's "AAA" should be anti-aircraft`)
	}
	if !registry.Of("factory").IsStructure {
		t.Error(`the board's "factory" should be a structure`)
	}
	if registry.Of("infantry").IsStructure {
		t.Error("infantry must not be a structure")
	}
}

// Bombardment must find the board's battleships without a "cruiser" declared.
func TestBoardSpelling_BombardmentShips(t *testing.T) {
	boardWithRoster(t)

	units := []*models.Piece{
		{Name: "battleship", Terrain: models.Water},
		{Name: "transport", Terrain: models.Water},
		{Name: "sub", Terrain: models.Water},
	}
	ships := getBombardmentShips(units)
	if len(ships) != 1 || ships[0].Name != "battleship" {
		t.Errorf("expected only the battleship to bombard, got %d ships", len(ships))
	}
}
