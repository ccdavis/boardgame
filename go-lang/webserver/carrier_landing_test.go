package webserver

import (
	"strings"
	"testing"

	"boardgame/models"
)

// carrierIn returns the first carrier in a sea zone.
func carrierIn(t *testing.T, session *GameSession, zone string) *models.Piece {
	t.Helper()
	g := session.Controller.Game
	for _, id := range g.Board[zone].Pieces {
		if piece := g.Pieces[id]; piece != nil && piece.Name == "carrier" {
			return piece
		}
	}
	t.Fatalf("no carrier in %s", zone)
	return nil
}

// The complaint this exists for: a fighter flown into a sea zone holding a
// carrier, with nothing on screen to say a landing had happened -- and then the
// fighter lost at the end of the turn. The zone must be offered as a CARRIER
// landing, distinctly from an ordinary move, and must name the deck so the
// browser can ask before booking it.
func TestCarrier_LandingIsOfferedAndNamed(t *testing.T) {
	session := usaSession(t, models.NoncombatMovePhase)
	fighter := pick(t, session, "Western US", "fighter", 1)[0]

	dests, err := reachableForPiece(session, fighter, "Western US")
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}

	dest, ok := dests["Western US Pacific"]
	if !ok {
		t.Fatal("the sea zone with our carrier was not offered at all")
	}
	if !dest.IsCarrier {
		t.Error("the landing was offered as an ordinary move, with nothing to " +
			"distinguish open water from a deck")
	}
	// One fighter is parked there already, so three of the four decks are free.
	if !strings.Contains(dest.Note, "carrier") || !strings.Contains(dest.Note, "3 deck spaces") {
		t.Errorf("note does not say what the fighter lands on: %q", dest.Note)
	}
}

// A fighter coming home to its own fleet is not an attack. Judging hostility by
// the territory's owner called every sea zone enemy ground -- nobody holds an
// ocean, so its owner is Neutral -- and painted the carrier red.
func TestCarrier_OwnFleetIsNotAnAttack(t *testing.T) {
	session := usaSession(t, models.CombatMovePhase)
	fighter := pick(t, session, "Western US", "fighter", 1)[0]

	dests, err := reachableForPiece(session, fighter, "Western US")
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}

	dest, ok := dests["Western US Pacific"]
	if !ok {
		t.Fatal("our own fleet's sea zone was not offered")
	}
	if dest.IsAttack {
		t.Error("flying to our own carrier was marked as an attack")
	}
	if !dest.IsCarrier {
		t.Error("a combat-move landing on our own deck is still a landing")
	}
}

// Open water is not a destination. A sea zone with no deck must not light up,
// in either phase, unless there is a fleet to fight there -- the old code
// offered it during combat move and the plane was lost when the turn ended.
func TestCarrier_OpenWaterIsNotOffered(t *testing.T) {
	for _, phase := range []models.Phase{models.CombatMovePhase, models.NoncombatMovePhase} {
		session := powerSession(t, "USA", phase)
		fighter := pick(t, session, "Eastern US", "fighter", 1)[0]

		dests, err := reachableForPiece(session, fighter, "Eastern US")
		if err != nil {
			t.Fatalf("%s: reachable: %v", phase, err)
		}
		// Eastern USA Atlantic holds a battleship, a transport and a sub, all
		// American: friendly water, but not a deck.
		if dest, ok := dests["Eastern USA Atlantic"]; ok {
			t.Errorf("%s: offered a carrier-less sea zone to a fighter: %+v", phase, dest)
		}
	}
}

// A deck with no room left is the case that most looks like a broken
// interface: the carrier is plainly visible on the map and the zone refuses to
// light up. The server must say why.
func TestCarrier_FullDeckIsExplained(t *testing.T) {
	session := usaSession(t, models.NoncombatMovePhase)

	// One fighter is already parked in Western US Pacific; a single-seat deck
	// is therefore full.
	carrierIn(t, session, "Western US Pacific").Capacity = 1

	fighter := pick(t, session, "Western US", "fighter", 1)[0]
	dests, err := reachableForPiece(session, fighter, "Western US")
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	if _, ok := dests["Western US Pacific"]; ok {
		t.Fatal("a full deck was still offered as a landing")
	}

	_, notes := transportOptions(session, []int{fighter}, "Western US")
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "Western US Pacific") {
		t.Fatalf("the full deck was never explained, notes: %v", notes)
	}
	if !strings.Contains(joined, "spoken for") {
		t.Errorf("explanation does not say the decks are taken: %q", joined)
	}
}

// An aircraft group must not be told about transports. The boarding
// explanation ("only land units can board transports") was the only thing a
// fighter ever heard, which is an answer to a question nobody asked.
func TestCarrier_AircraftAreNotLecturedAboutTransports(t *testing.T) {
	session := usaSession(t, models.NoncombatMovePhase)
	fighter := pick(t, session, "Western US", "fighter", 1)[0]

	dests, notes := transportOptions(session, []int{fighter}, "Western US")
	if len(dests) != 0 {
		t.Errorf("aircraft have no transport destinations, got %+v", dests)
	}
	for _, note := range notes {
		if strings.Contains(note, "transport") {
			t.Errorf("a fighter was told about transports: %q", note)
		}
	}
}

// Two fighters cannot both be promised the last seat: the second attempt must
// find the zone gone, and be told why.
func TestCarrier_SeatsAreCountedAgainstPlannedLandings(t *testing.T) {
	session := usaSession(t, models.NoncombatMovePhase)
	carrierIn(t, session, "Western US Pacific").Capacity = 2 // one seat spare

	first := pick(t, session, "Western US", "fighter", 1)[0]
	if err := session.Controller.PlanMove(first, "Western US", "Western US Pacific"); err != nil {
		t.Fatalf("first landing refused: %v", err)
	}

	// A second fighter, flown across the continent on the same turn.
	second := pick(t, session, "Eastern US", "fighter", 1)[0]
	dests, err := reachableForPiece(session, second, "Eastern US")
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	if _, ok := dests["Western US Pacific"]; ok {
		t.Error("the seat already promised to the first fighter was offered again")
	}
}
