package webserver

import (
	"testing"

	"boardgame/models"
)

// loadedTransport puts n of the player's infantry from `shore` aboard a
// transport in `zone`, and returns the transport and its cargo IDs.
func loadedTransport(t *testing.T, session *GameSession, zone, shore string, n int) (int, []int) {
	t.Helper()

	g := session.Controller.Game
	transportID := -1
	for _, id := range g.Board[zone].Pieces {
		piece := g.Pieces[id]
		if piece != nil && piece.Name == "transport" && piece.Owner != nil &&
			piece.Owner.Name == session.HumanPlayer {
			transportID = id
			break
		}
	}
	if transportID == -1 {
		t.Fatalf("no %s transport in %s", session.HumanPlayer, zone)
	}

	cargo := pick(t, session, shore, "infantry", n)
	for _, id := range cargo {
		if err := session.Controller.LoadUnit(transportID, id); err != nil {
			t.Fatalf("loading infantry aboard: %v", err)
		}
	}
	return transportID, cargo
}

// destNames indexes unload options by territory.
func destNames(dests []ReachableTerritoryDTO) map[string]ReachableTerritoryDTO {
	out := make(map[string]ReachableTerritoryDTO, len(dests))
	for _, dest := range dests {
		out[dest.Name] = dest
	}
	return out
}

// Combat move: the shores an assault can hit. Friendly ground is still a
// legal place to put troops down, and hostile ground is the whole point.
func TestUnload_CombatOffersEnemyShores(t *testing.T) {
	session := powerSession(t, "UK", models.CombatMovePhase)
	_, cargo := loadedTransport(t, session, "North Sea", "Britain", 2)

	dests, _ := transportOptions(session, cargo, "North Sea")
	byName := destNames(dests)

	// Norway and Western Europe are German; Britain is home.
	for _, want := range []string{"Norway", "Western Europe"} {
		dest, ok := byName[want]
		if !ok {
			t.Errorf("%s was not offered as an assault landing", want)
			continue
		}
		if !dest.IsUnload || !dest.IsAttack {
			t.Errorf("%s should be an opposed landing, got %+v", want, dest)
		}
	}
	if dest, ok := byName["Britain"]; !ok || !dest.IsUnload || dest.IsAttack {
		t.Errorf("putting troops back ashore at home should be a plain unload, got %+v", dest)
	}
}

// Noncombat move: every valid shore EXCEPT enemy ones. An unopposed landing is
// a movement; storming a beach is a battle, and battles belong to combat move.
func TestUnload_NoncombatOffersOnlyFriendlyShores(t *testing.T) {
	session := powerSession(t, "UK", models.NoncombatMovePhase)
	_, cargo := loadedTransport(t, session, "North Sea", "Britain", 2)

	dests, _ := transportOptions(session, cargo, "North Sea")
	byName := destNames(dests)

	if dest, ok := byName["Britain"]; !ok || !dest.IsUnload {
		t.Errorf("friendly shore was not offered in noncombat, got %+v", dest)
	}
	for _, forbidden := range []string{"Norway", "Western Europe", "Germany"} {
		if dest, ok := byName[forbidden]; ok {
			t.Errorf("enemy shore %s offered during noncombat move: %+v", forbidden, dest)
		}
	}
	for _, dest := range dests {
		if dest.IsAttack {
			t.Errorf("noncombat move offered an attack: %+v", dest)
		}
	}
}

// A transport booked to sail can still land its troops -- at the far end. That
// is how an amphibious assault is planned: load, sail and land in one phase.
func TestUnload_FollowsTheTransportsPlannedVoyage(t *testing.T) {
	session := powerSession(t, "UK", models.CombatMovePhase)
	transportID, cargo := loadedTransport(t, session, "North Sea", "Britain", 2)

	if err := session.Controller.PlanMove(transportID, "North Sea", "Baltic Sea"); err != nil {
		t.Fatalf("sailing the transport: %v", err)
	}

	dests, _ := transportOptions(session, cargo, "North Sea")
	byName := destNames(dests)

	// Only the Baltic touches Finland and Sweden; if the voyage were ignored,
	// neither would appear.
	if _, ok := byName["Finland"]; !ok {
		t.Error("a shore on the transport's destination was not offered")
	}
}

// Picking the ship is the whole of an ocean crossing: the cargo goes with it,
// with nothing extra for the player to select or confirm.
func TestUnload_CargoSailsWithItsShip(t *testing.T) {
	session := powerSession(t, "UK", models.NoncombatMovePhase)
	transportID, cargo := loadedTransport(t, session, "North Sea", "Britain", 2)

	if err := session.Controller.PlanMove(transportID, "North Sea", "Canadian Atlantic"); err != nil {
		t.Fatalf("sailing the transport: %v", err)
	}
	if err := session.Controller.ExecuteNoncombatMoves(); err != nil {
		t.Fatalf("executing moves: %v", err)
	}

	g := session.Controller.Game
	if where := territoryNameOf(g, transportID); where != "Canadian Atlantic" {
		t.Fatalf("the transport is in %s, not Canadian Atlantic", where)
	}
	for _, id := range cargo {
		if g.GetTransportForPiece(id) != transportID {
			t.Errorf("piece %d fell out of the hold during the voyage", id)
		}
	}
	// And it can come ashore at the far end, from where the ship now is.
	dests, _ := transportOptions(session, cargo, "Canadian Atlantic")
	if len(dests) == 0 {
		t.Error("the cargo cannot land anywhere from its new sea zone")
	}
}

// The sea-zone picker offers hulls, and a hull carries its cargo with it. The
// cargo must never be offered a voyage of its own.
func TestUnload_CargoHasNoSeaDestinations(t *testing.T) {
	session := powerSession(t, "UK", models.NoncombatMovePhase)
	_, cargo := loadedTransport(t, session, "North Sea", "Britain", 2)

	dests, err := reachableForPiece(session, cargo[0], "North Sea")
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	for name := range dests {
		t.Errorf("loaded infantry was offered a destination of its own: %s", name)
	}

	// And the group-level answer for cargo is shores only.
	options, _ := transportOptions(session, cargo, "North Sea")
	g := session.Controller.Game
	for _, dest := range options {
		if g.Board[dest.Name].Terrain == models.Water {
			t.Errorf("cargo was offered a sea zone: %s", dest.Name)
		}
		if !dest.IsUnload {
			t.Errorf("a cargo destination that is not an unload: %+v", dest)
		}
	}
}
