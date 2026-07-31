package parser

import (
	"strings"
	"testing"

	"boardgame/models"
)

// parse runs the parser over a board source and fails the test on error.
func parse(t *testing.T, src string) *models.Game {
	t.Helper()

	game, err := NewParserFromReader(strings.NewReader(src)).Parse()
	if err != nil {
		t.Fatalf("parsing:\n%s\nerror: %v", src, err)
	}
	return game
}

// parseErr expects the parse to fail and returns the error.
func parseErr(t *testing.T, src string) error {
	t.Helper()

	_, err := NewParserFromReader(strings.NewReader(src)).Parse()
	if err == nil {
		t.Fatalf("expected an error parsing:\n%s", src)
	}
	return err
}

// minimal is a complete, valid board, kept small so each test can vary one part.
const minimal = `
Players
  Germany, USSR, Neutral;

Turn 1;

Territories
  Germany :land, Germany, 10;
  Russia :land, USSR, 8;
  North Sea :water, Germany, 0;

Map
  Germany: Russia, North Sea;
  Russia: Germany;
  North Sea: Germany;

Units
  infantry: land, 1 movement, 1 attack, 2 defend, 3 cost;
  sub: water, 2 movement, 2 attack, 2 defend, 8 cost;

Placement
  Germany: 3 infantry;
  Russia: 2 infantry;
`

func TestParse_MinimalBoard(t *testing.T) {
	game := parse(t, minimal)

	if len(game.Players) != 3 {
		t.Errorf("got %d players, want 3", len(game.Players))
	}
	if len(game.Board) != 3 {
		t.Errorf("got %d territories, want 3", len(game.Board))
	}
	if game.Board["Germany"].Production != 10 {
		t.Errorf("Germany production = %d, want 10", game.Board["Germany"].Production)
	}
	if game.Board["North Sea"].Terrain != models.Water {
		t.Error("North Sea should be water")
	}
	if len(game.Pieces) != 5 {
		t.Errorf("got %d pieces, want 5", len(game.Pieces))
	}
}

// Territory names are token sequences, so a multi-word name must survive
// intact, and internal whitespace collapses to a single space. Several names
// in the real board wrap across source lines.
func TestParse_MultiWordTerritoryNames(t *testing.T) {
	game := parse(t, strings.Replace(minimal, "North Sea", "South West\n  Indian Ocean", -1))

	if _, ok := game.Board["South West Indian Ocean"]; !ok {
		names := make([]string, 0, len(game.Board))
		for name := range game.Board {
			names = append(names, name)
		}
		t.Errorf("wrapped multi-word name not preserved; got %v", names)
	}
}

// Adjacency is mutual. A board that declares an edge from one side only must
// still produce a symmetric graph, or units can move one way and not back.
func TestParse_AdjacencyIsSymmetric(t *testing.T) {
	game := parse(t, minimal)

	if problems := game.ValidateGraph(); len(problems) > 0 {
		for _, problem := range problems {
			t.Errorf("graph problem: %s", problem)
		}
	}
}

func TestParse_SidesSetAlliancesAndWhoPlays(t *testing.T) {
	game := parse(t, minimal+`
Sides
  Axis: Germany;
  Allies: USSR;

Capitals
  Germany: Germany;
  USSR: Russia;
`)

	if game.Players["Germany"].Side != "Axis" {
		t.Errorf("Germany side = %q, want Axis", game.Players["Germany"].Side)
	}
	if !game.Players["Germany"].TakesTurns {
		t.Error("Germany should take turns")
	}
	// Neutral is absent from Sides, so it owns territory without playing.
	if game.Players["Neutral"].TakesTurns {
		t.Error("Neutral must not take turns")
	}
	if got := game.TurnTakingPowers(); len(got) != 2 {
		t.Errorf("turn-taking powers = %v, want 2", got)
	}
}

func TestParse_Capitals(t *testing.T) {
	game := parse(t, minimal+`
Capitals
  Germany: Germany;
  USSR: Russia;
`)

	if game.Players["USSR"].Capital != "Russia" {
		t.Errorf("USSR capital = %q, want Russia", game.Players["USSR"].Capital)
	}
}

func TestParse_VictoryCities(t *testing.T) {
	game := parse(t, minimal+`
VictoryCities
  Germany, Russia;
`)

	if !game.Board["Germany"].IsVictoryCity || !game.Board["Russia"].IsVictoryCity {
		t.Error("declared victory cities were not marked")
	}
	if game.Board["North Sea"].IsVictoryCity {
		t.Error("North Sea was not declared a victory city")
	}
	if !game.VictoryCitiesEnabled {
		t.Error("a board declaring victory cities should have them available")
	}
}

func TestParse_Neutrality(t *testing.T) {
	game := parse(t, minimal+`
Neutrality
  Russia: strict;
  North Sea: proaxis;
`)

	if game.Board["Russia"].NeutralType != models.StrictNeutral {
		t.Errorf("Russia neutrality = %v, want strict", game.Board["Russia"].NeutralType)
	}
	if game.Board["North Sea"].NeutralType != models.ProAxisNeutral {
		t.Errorf("North Sea neutrality = %v, want pro-Axis", game.Board["North Sea"].NeutralType)
	}
}

// Sections terminate on any keyword, not on one hardcoded successor, so they
// may appear in any order. The original parser assumed a fixed sequence and
// broke as soon as a section was added between two others.
func TestParse_SectionsMayAppearInAnyOrder(t *testing.T) {
	reordered := `
Players
  Germany, USSR;

Sides
  Axis: Germany;
  Allies: USSR;

Turn 1;

Territories
  Germany :land, Germany, 10;
  Russia :land, USSR, 8;

Capitals
  Germany: Germany;
  USSR: Russia;

Map
  Germany: Russia;
  Russia: Germany;

Units
  infantry: land, 1 movement, 1 attack, 2 defend, 3 cost;

Placement
  Germany: 1 infantry;
`
	game := parse(t, reordered)

	if game.Players["Germany"].Side != "Axis" {
		t.Error("Sides before Territories was not applied")
	}
	if game.Players["Germany"].Capital != "Germany" {
		t.Error("Capitals between Territories and Map was not applied")
	}
}

func TestParse_RejectsUnknownTerrain(t *testing.T) {
	err := parseErr(t, strings.Replace(minimal, ":land, Germany, 10", ":swamp, Germany, 10", 1))

	if !strings.Contains(err.Error(), "terrain") {
		t.Errorf("error should mention the terrain: %v", err)
	}
}

func TestParse_RejectsCapitalForUnknownPower(t *testing.T) {
	err := parseErr(t, minimal+`
Capitals
  Atlantis: Germany;
`)
	if !strings.Contains(err.Error(), "Atlantis") {
		t.Errorf("error should name the unknown power: %v", err)
	}
}

func TestParse_RejectsVictoryCityThatIsNotATerritory(t *testing.T) {
	err := parseErr(t, minimal+`
VictoryCities
  Atlantis;
`)
	if !strings.Contains(err.Error(), "Atlantis") {
		t.Errorf("error should name the unknown territory: %v", err)
	}
}

func TestParse_RejectsUnknownNeutrality(t *testing.T) {
	err := parseErr(t, minimal+`
Neutrality
  Russia: friendly;
`)
	if !strings.Contains(err.Error(), "friendly") {
		t.Errorf("error should quote the bad value: %v", err)
	}
}

func TestParse_RejectsSideNamingAnUnknownPower(t *testing.T) {
	err := parseErr(t, minimal+`
Sides
  Axis: Atlantis;
`)
	if !strings.Contains(err.Error(), "Atlantis") {
		t.Errorf("error should name the unknown power: %v", err)
	}
}

// Placement of a unit type the board never declared should be refused rather
// than silently producing nothing.
func TestParse_RejectsPlacementOfUndeclaredUnit(t *testing.T) {
	parseErr(t, strings.Replace(minimal, "Germany: 3 infantry;", "Germany: 3 zeppelin;", 1))
}

func TestParse_EmptyInputIsNotAnError(t *testing.T) {
	game := parse(t, "")
	if len(game.Board) != 0 {
		t.Errorf("empty source produced %d territories", len(game.Board))
	}
}

// NewParser reports a missing file rather than panicking.
func TestNewParser_MissingFile(t *testing.T) {
	if _, err := NewParser("/nonexistent/board.gdf"); err == nil {
		t.Error("expected an error opening a file that does not exist")
	}
}
