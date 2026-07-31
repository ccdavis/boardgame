package models

import "testing"

// aaaRoster mirrors the unit names aaa.gdf actually declares. The point of the
// registry is that these names work without the board being renamed to suit the
// code, so the test uses the board's spellings, not the code's old ones.
func aaaRoster() map[string]*Piece {
	return map[string]*Piece{
		"carrier":    {Name: "carrier", Terrain: Water, Movement: 2, Attack: 0, Defend: 1, Cost: 24},
		"sub":        {Name: "sub", Terrain: Water, Movement: 2, Attack: 2, Defend: 2, Cost: 8},
		"battleship": {Name: "battleship", Terrain: Water, Movement: 2, Attack: 4, Defend: 4, Cost: 24},
		"transport":  {Name: "transport", Terrain: Water, Movement: 2, Attack: 0, Defend: 1, Cost: 10},
		"infantry":   {Name: "infantry", Terrain: Land, Movement: 1, Attack: 1, Defend: 2, Cost: 3},
		"armor":      {Name: "armor", Terrain: Land, Movement: 2, Attack: 3, Defend: 2, Cost: 5},
		"fighter":    {Name: "fighter", Terrain: Air, Movement: 4, Attack: 3, Defend: 4, Cost: 12},
		"bomber":     {Name: "bomber", Terrain: Air, Movement: 6, Attack: 4, Defend: 2, Cost: 16},
		"AAA":        {Name: "AAA", Terrain: Land, Movement: 0, Attack: 0, Defend: 1, Cost: 5},
		"factory":    {Name: "factory", Terrain: Land, Movement: 0, Attack: 0, Defend: 0, Cost: 32},
	}
}

// This is the bug the registry exists to fix: the board says "sub", the old
// combat code looked for "submarine", so submarines never used a single one of
// their special rules in a real game.
func TestRegistry_BoardSpellingOfSubmarine(t *testing.T) {
	registry := BuildUnitRegistry(aaaRoster())

	if !registry.Of("sub").IsSubmarine {
		t.Error(`"sub" is not recognised as a submarine`)
	}
	if !registry.Of("submarine").IsSubmarine {
		t.Error(`"submarine" is not recognised as a submarine`)
	}
}

// Likewise the board says "factory" while parts of the AI looked up
// "industrial_complex", so the NPC could never buy or find one.
func TestRegistry_BoardSpellingOfFactory(t *testing.T) {
	registry := BuildUnitRegistry(aaaRoster())

	for _, name := range []string{"factory", "industrial_complex", "industrialcomplex"} {
		if !registry.Of(name).IsStructure {
			t.Errorf("%q is not recognised as a structure", name)
		}
	}
}

func TestRegistry_StructuresAreNotCombatUnits(t *testing.T) {
	registry := BuildUnitRegistry(aaaRoster())

	if !registry.Of("factory").IsStructure {
		t.Error("factory should be a structure")
	}
	if registry.Of("infantry").IsStructure {
		t.Error("infantry must not be a structure")
	}
	if registry.Of("battleship").IsStructure {
		t.Error("battleship must not be a structure")
	}
}

// A board may invent its own immobile, harmless unit. It should behave like a
// structure without this build having to know its name.
func TestRegistry_ImmobileHarmlessUnitIsAStructure(t *testing.T) {
	roster := aaaRoster()
	roster["bunkercomplex"] = &Piece{Name: "bunkercomplex", Movement: 0, Attack: 0, Defend: 0}

	if !BuildUnitRegistry(roster).Of("bunkercomplex").IsStructure {
		t.Error("an immobile unit that neither attacks nor defends should be a structure")
	}
}

func TestRegistry_AntiAircraft(t *testing.T) {
	registry := BuildUnitRegistry(aaaRoster())

	if !registry.Of("AAA").IsAA {
		t.Error(`"AAA" should be anti-aircraft`)
	}
	// AAA defends at 1, so the immobile-and-harmless rule must not also make it
	// a structure -- it is a casualty like any other unit.
	if registry.Of("AAA").IsStructure {
		t.Error("AAA must not be treated as a structure")
	}
}

func TestRegistry_BattleshipTakesTwoHits(t *testing.T) {
	registry := BuildUnitRegistry(aaaRoster())

	if got := registry.Of("battleship").MaxHits; got != 2 {
		t.Errorf("battleship MaxHits = %d, want 2", got)
	}
	if got := registry.Of("infantry").MaxHits; got != 1 {
		t.Errorf("infantry MaxHits = %d, want 1", got)
	}
}

func TestRegistry_BlitzAndBombard(t *testing.T) {
	registry := BuildUnitRegistry(aaaRoster())

	if !registry.Of("armor").CanBlitz {
		t.Error("armor should be able to blitz")
	}
	if !registry.Of("tank").CanBlitz {
		t.Error(`the alternative spelling "tank" should also blitz`)
	}
	if registry.Of("infantry").CanBlitz {
		t.Error("infantry must not blitz")
	}
	if !registry.Of("battleship").CanBombard {
		t.Error("battleship should be able to bombard")
	}
	if !registry.Of("cruiser").CanBombard {
		t.Error("cruiser should be able to bombard")
	}
	// Destroyers escort and hunt submarines; shore bombardment is for
	// battleships and cruisers only.
	if registry.Of("destroyer").CanBombard {
		t.Error("destroyer must not bombard")
	}
}

// aaa.gdf declares no destroyer or artillery. Asking about them must not panic
// or misreport -- it should simply say the capability is absent from this board.
func TestRegistry_UnitsAbsentFromTheBoard(t *testing.T) {
	registry := BuildUnitRegistry(aaaRoster())

	if _, declared := registry.byName["destroyer"]; declared {
		t.Error("the roster should not contain a destroyer")
	}
	// Asked directly, the capability is still known, so a board that does field
	// destroyers gets the right behaviour with no code change.
	if !registry.Of("destroyer").NegatesSubmarines {
		t.Error("a destroyer should negate submarines when a board declares one")
	}
	if !registry.Of("artillery").SupportsInfantry {
		t.Error("artillery should support infantry when a board declares it")
	}
}

func TestRegistry_NilSafe(t *testing.T) {
	var registry *UnitRegistry
	if !registry.Of("sub").IsSubmarine {
		t.Error("a nil registry should still answer from the name")
	}
	if got := registry.For(nil).MaxHits; got != 1 {
		t.Errorf("nil piece MaxHits = %d, want 1", got)
	}
}
