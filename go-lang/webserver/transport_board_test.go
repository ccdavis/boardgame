package webserver

import (
	"strings"
	"testing"

	"boardgame/game"
	"boardgame/models"
	"boardgame/parser"
)

// powerSession puts the real board in front of one power, mid-turn.
func powerSession(t *testing.T, power string, phase models.Phase) *GameSession {
	t.Helper()

	p, err := parser.NewParser("../../aaa.gdf")
	if err != nil {
		t.Fatalf("loading the board: %v", err)
	}
	g, err := p.Parse()
	if err != nil {
		t.Fatalf("parsing the board: %v", err)
	}
	controller := game.NewGameController(g)
	controller.StartGame()
	g.CurrentPower = power
	g.CurrentPhase = phase
	return NewGameSession(controller, power)
}

// usaSession puts the real board in front of a USA player, mid-turn.
func usaSession(t *testing.T, phase models.Phase) *GameSession {
	t.Helper()
	return powerSession(t, "USA", phase)
}

// pick returns the IDs of up to n of the session player's pieces of the given
// type in a territory.
func pick(t *testing.T, session *GameSession, territory, unitType string, n int) []int {
	t.Helper()

	g := session.Controller.Game
	var ids []int
	for _, id := range g.Board[territory].Pieces {
		piece := g.Pieces[id]
		if piece.Owner != nil && piece.Owner.Name == session.HumanPlayer &&
			piece.Name == unitType && len(ids) < n {
			ids = append(ids, id)
		}
	}
	if len(ids) < n {
		t.Fatalf("wanted %d %s in %s, board has %d", n, unitType, territory, len(ids))
	}
	return ids
}

// A group that fits gets the sea zone and no lecture.
func TestBoarding_GroupThatFitsIsOffered(t *testing.T) {
	session := usaSession(t, models.CombatMovePhase)
	group := pick(t, session, "Eastern US", "infantry", 2)

	dests, notes := transportOptions(session, group, "Eastern US")

	if len(dests) != 1 || dests[0].Name != "Eastern USA Atlantic" || !dests[0].IsBoard {
		t.Fatalf("expected to board in Eastern USA Atlantic, got %+v", dests)
	}
	if len(notes) != 0 {
		t.Errorf("a group that can board should draw no explanation, got %v", notes)
	}
}

// The regression this exists for: the picker starts with the whole stack
// selected, the stack does not fit, and the old code answered with silence.
func TestBoarding_TooManyUnitsSaysSo(t *testing.T) {
	session := usaSession(t, models.CombatMovePhase)
	group := pick(t, session, "Eastern US", "infantry", 4)
	group = append(group, pick(t, session, "Eastern US", "armor", 2)...)

	dests, notes := transportOptions(session, group, "Eastern US")

	if len(dests) != 0 {
		t.Fatalf("6 units must not fit one transport, got %+v", dests)
	}
	if len(notes) == 0 {
		t.Fatal("no explanation for a group that is too big -- the sea zone just vanishes")
	}
	note := strings.Join(notes, " ")
	for _, want := range []string{"Eastern USA Atlantic", "room for 4", "picked 6"} {
		if !strings.Contains(note, want) {
			t.Errorf("explanation %q does not mention %q", note, want)
		}
	}
}

// Aircraft in the group kill boarding outright, whatever the capacity.
func TestBoarding_AircraftAreExplained(t *testing.T) {
	session := usaSession(t, models.CombatMovePhase)
	group := append(pick(t, session, "Eastern US", "infantry", 1),
		pick(t, session, "Eastern US", "fighter", 1)...)

	dests, notes := transportOptions(session, group, "Eastern US")

	if len(dests) != 0 {
		t.Fatalf("a fighter cannot board a transport, got %+v", dests)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "Only land units") {
		t.Fatalf("expected an aircraft explanation, got %v", notes)
	}
}

// Inland units are not trying to board and must not be told about transports.
func TestBoarding_InlandMoveIsNotLectured(t *testing.T) {
	session := usaSession(t, models.CombatMovePhase)
	g := session.Controller.Game

	// Russia is landlocked on this board; its garrison includes aircraft-free
	// infantry and, for the second half of the check, a fighter.
	inland := ""
	for name, terr := range g.Board {
		if terr.Terrain == models.Water {
			continue
		}
		coastal := false
		for _, neighbour := range terr.ConnectedTo {
			if neighbour.Terrain == models.Water {
				coastal = true
				break
			}
		}
		if !coastal && len(terr.Pieces) > 0 {
			inland = name
			break
		}
	}
	if inland == "" {
		t.Skip("no landlocked territory with units on this board")
	}

	// Any owner will do -- the point is that a non-coastal source is silent.
	session.HumanPlayer = g.Pieces[g.Board[inland].Pieces[0]].Owner.Name
	group := []int{g.Board[inland].Pieces[0]}

	if _, notes := transportOptions(session, group, inland); len(notes) != 0 {
		t.Errorf("a move from landlocked %s mentioned transports: %v", inland, notes)
	}
}
