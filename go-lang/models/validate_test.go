package models

import (
	"strings"
	"testing"
)

// validGame is a small game that satisfies every invariant.
func validGame(t *testing.T) *Game {
	t.Helper()

	g := NewGame()
	g.PlayerOrder = []string{"Germany", "USSR"}
	for _, name := range g.PlayerOrder {
		player := g.GetOrCreatePlayer(name)
		player.TakesTurns = true
	}
	g.Players["Germany"].Side = "Axis"
	g.Players["USSR"].Side = "Allies"
	g.Players["Germany"].Capital = "Berlin"
	g.Players["USSR"].Capital = "Moscow"

	g.AddTerritory("Berlin", Land, "Germany", 10)
	g.AddTerritory("Moscow", Land, "USSR", 8)
	g.ConnectTerritories("Berlin", "Moscow")

	g.AddPieceTemplate("infantry", Land, 1, 1, 2, 3)
	if err := g.PlacePieces("Berlin", "infantry", 2); err != nil {
		t.Fatalf("placing pieces: %v", err)
	}
	return g
}

func kindsOf(problems []Problem) map[string]int {
	counts := make(map[string]int)
	for _, problem := range problems {
		counts[problem.Kind]++
	}
	return counts
}

func TestValidate_CleanGame(t *testing.T) {
	if problems := validGame(t).Validate(); len(problems) != 0 {
		for _, problem := range problems {
			t.Errorf("unexpected problem: %s", problem)
		}
	}
}

// A power that plays needs a side. Without one it has no allies and no
// enemies: allied powers can attack each other, neutral rules never fire, and
// the victory condition can never be evaluated.
func TestValidate_PlayingPowerWithoutASide(t *testing.T) {
	g := validGame(t)
	g.Players["Germany"].Side = ""

	if got := kindsOf(g.Validate())["NO_SIDE"]; got != 1 {
		t.Errorf("expected the missing side to be reported once, got %d", got)
	}
}

// A board that declares no sides marks nobody as playing, so the metadata
// checks must stay quiet rather than reporting every power.
func TestValidate_BoardWithoutSidesIsNotNagged(t *testing.T) {
	g := validGame(t)
	for _, player := range g.Players {
		player.TakesTurns = false
		player.Side = ""
		player.Capital = ""
	}

	counts := kindsOf(g.Validate())
	if counts["NO_SIDE"] != 0 || counts["NO_CAPITAL"] != 0 {
		t.Errorf("a board with no playing powers should not report metadata gaps: %v", counts)
	}
}

func TestValidate_CapitalMustBeARealTerritory(t *testing.T) {
	g := validGame(t)
	g.Players["Germany"].Capital = "Atlantis"

	if got := kindsOf(g.Validate())["BAD_CAPITAL"]; got != 1 {
		t.Errorf("expected the bad capital to be reported, got %d", got)
	}
}

// Territory.Owner and Player.Territories are a hand-maintained index in two
// directions. When they disagree, income differs depending on which side the
// caller walks.
func TestValidate_OwnerIndexDesync(t *testing.T) {
	g := validGame(t)
	// Reassign the territory without updating the player's list.
	g.Board["Berlin"].Owner = g.Players["USSR"]

	if got := kindsOf(g.Validate())["OWNER_DESYNC"]; got == 0 {
		t.Error("a one-sided ownership change was not reported")
	}
}

// This is the transport-cargo leak detector. Cargo lives only in Piece.Holding
// and is absent from Territory.Pieces, so destroying a transport leaves its
// cargo in Game.Pieces forever: invisible on the board, still counted.
func TestValidate_OrphanedPiece(t *testing.T) {
	g := validGame(t)
	g.Pieces[999] = &Piece{ID: 999, Name: "infantry"}

	problems := g.Validate()
	if got := kindsOf(problems)["ORPHANED_PIECE"]; got != 1 {
		t.Fatalf("expected one orphaned piece, got %d (%v)", got, problems)
	}
	if !strings.Contains(problems[0].Detail, "999") {
		t.Errorf("the report should name the piece: %s", problems[0].Detail)
	}
}

// Cargo aboard a transport is legitimately absent from any territory.
func TestValidate_CarriedCargoIsNotOrphaned(t *testing.T) {
	g := validGame(t)
	g.AddPieceTemplate("transport", Water, 2, 0, 1, 10)
	g.AddTerritory("North Sea", Water, "Germany", 0)
	if err := g.PlacePieces("North Sea", "transport", 1); err != nil {
		t.Fatalf("placing transport: %v", err)
	}
	transportID := g.NextPieceID - 1

	// Move one infantry into the hold: out of the territory, into Holding.
	berlin := g.Board["Berlin"]
	cargoID := berlin.Pieces[0]
	berlin.Pieces = berlin.Pieces[1:]
	g.Pieces[transportID].Holding = append(g.Pieces[transportID].Holding, cargoID)

	if got := kindsOf(g.Validate())["ORPHANED_PIECE"]; got != 0 {
		t.Errorf("cargo in a hold was reported as orphaned (%d)", got)
	}
}

func TestValidate_PieceInTwoPlaces(t *testing.T) {
	g := validGame(t)
	// List Berlin's first piece in Moscow as well.
	g.Board["Moscow"].Pieces = append(g.Board["Moscow"].Pieces, g.Board["Berlin"].Pieces[0])

	if got := kindsOf(g.Validate())["DUPLICATE_PIECE"]; got != 1 {
		t.Errorf("expected the duplicated piece to be reported, got %d", got)
	}
}

func TestValidate_TerritoryListingAPieceThatDoesNotExist(t *testing.T) {
	g := validGame(t)
	g.Board["Moscow"].Pieces = append(g.Board["Moscow"].Pieces, 4242)

	if got := kindsOf(g.Validate())["MISSING_PIECE"]; got != 1 {
		t.Errorf("expected the missing piece to be reported, got %d", got)
	}
}

func TestValidate_TurnOrderNamingAnUnknownPower(t *testing.T) {
	g := validGame(t)
	g.PlayerOrder = append(g.PlayerOrder, "Atlantis")

	if got := kindsOf(g.Validate())["UNKNOWN_POWER"]; got != 1 {
		t.Errorf("expected the unknown power to be reported, got %d", got)
	}
}
