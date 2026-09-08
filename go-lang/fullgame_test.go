package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"boardgame/game"
	"boardgame/models"
	"boardgame/parser"
)

// A whole game, played by computer players on the real board, with the state
// checked after every single turn.
//
// The value here is not the final result -- it is the thousands of rule
// interactions along the way that no unit test sets up. A unit test asks
// whether a retreat unwinds correctly; this asks whether four hundred turns of
// purchases, movement, combat, capture and income leave the board consistent.
//
// Failures are reproducible: the seed is logged, and setting GAME_SEED replays
// that exact game.
//
//	go test -run TestFullGame -v .
//	GAME_SEED=12345 go test -run TestFullGame -v .
//
// TRANSCRIPT_DIR puts the transcript somewhere durable instead of a temp dir.
const (
	defaultSeed = 20260731
	// Batch observation put the median decided game at round 36, so a shorter
	// cap truncated real endings; this horizon lets sustained victories land.
	maxTurns = 40
)

func gameSeed() int64 {
	if raw := os.Getenv("GAME_SEED"); raw != "" {
		if seed, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return seed
		}
	}
	return defaultSeed
}

// turnSnapshot is what we compare across a turn boundary.
type turnSnapshot struct {
	turn      int
	power     string
	pieces    int
	territory map[string]string // territory -> owner
	ipcs      map[string]int
}

func takeSnapshot(g *models.Game) turnSnapshot {
	snap := turnSnapshot{
		turn:      g.Turn,
		power:     g.CurrentPower,
		pieces:    len(g.Pieces),
		territory: make(map[string]string, len(g.Board)),
		ipcs:      make(map[string]int, len(g.Players)),
	}
	for name, territory := range g.Board {
		if territory.Owner != nil {
			snap.territory[name] = territory.Owner.Name
		}
	}
	for name, player := range g.Players {
		snap.ipcs[name] = player.IPCs
	}
	return snap
}

func TestFullGame(t *testing.T) {
	seed := gameSeed()
	t.Logf("seed %d -- replay with: GAME_SEED=%d go test -run TestFullGame -v .", seed, seed)

	p, err := parser.NewParser("../aaa.gdf")
	if err != nil {
		t.Fatalf("opening board: %v", err)
	}
	g, err := p.Parse()
	if err != nil {
		t.Fatalf("parsing board: %v", err)
	}

	powers := g.TurnTakingPowers()
	if len(powers) < 2 {
		t.Fatalf("need at least two playing powers, got %v", powers)
	}

	runner := game.NewGameRunner(g, "All-computer game")
	// Each power gets a distinct seed derived from the run's seed, so they do
	// not all roll the same dice in lockstep.
	for i, name := range powers {
		runner.RegisterSeededNPC(name, "normal", seed+int64(i)*7919)
	}
	runner.SetMaxTurns(maxTurns)

	if err := runner.Controller.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}

	previous := takeSnapshot(g)
	turnsPlayed := 0
	battlesSeen := 0

	for turn := 0; turn < maxTurns*len(powers); turn++ {
		power := g.CurrentPower
		npc, ok := runner.NPCPlayers[power]
		if !ok {
			t.Fatalf("turn %d: no AI registered for %q -- is a non-playing power taking turns?",
				g.Turn, power)
		}

		before := len(g.Pieces)
		if err := npc.TakeTurn(runner.Controller, runner.Transcript); err != nil {
			t.Fatalf("turn %d, %s: %v", g.Turn, power, err)
		}
		turnsPlayed++
		battlesSeen += len(runner.Controller.PendingBattles)

		// The board must be internally consistent after every single turn.
		// Checking only at the end would tell us something broke without
		// saying when, and a leaked piece is invisible until you look for it.
		if problems := g.Validate(); len(problems) > 0 {
			t.Errorf("turn %d, after %s played, the game state is inconsistent:", g.Turn, power)
			for i, problem := range problems {
				if i >= 8 {
					t.Errorf("  ... and %d more", len(problems)-8)
					break
				}
				t.Errorf("  %s", problem)
			}
			// Dump what happened before giving up, or the failure has no
			// context to debug from.
			writeTranscript(t, runner, seed)
			t.FailNow()
		}

		checkTurnInvariants(t, g, previous, power, before)
		previous = takeSnapshot(g)

		if winner, won, err := runner.Controller.CheckVictoryCondition(); err == nil && won {
			t.Logf("%s wins on turn %d", winner, g.Turn)
			break
		}
		if g.Turn > maxTurns {
			break
		}
	}

	writeTranscript(t, runner, seed)
	summarise(t, g, runner, turnsPlayed)
}

// checkTurnInvariants asserts the things that must hold across any single turn,
// whatever the AI decided to do.
func checkTurnInvariants(t *testing.T, g *models.Game, before turnSnapshot, power string, piecesBefore int) {
	t.Helper()

	// Nobody may spend money they do not have.
	for name, player := range g.Players {
		if player.IPCs < 0 {
			t.Errorf("turn %d: %s has %d IPCs", g.Turn, name, player.IPCs)
		}
	}

	// Only the power that just played may gain or lose territory -- or, by
	// liberation, an ally of it: a province retaken from the enemy returns
	// to its original owner. If anyone else's holdings changed, ownership
	// was written by the wrong actor.
	sameSide := func(a, b string) bool {
		pa, pb := g.Players[a], g.Players[b]
		return pa != nil && pb != nil && pa.Side != "" && pa.Side == pb.Side
	}
	for territory, owner := range takeSnapshot(g).territory {
		was := before.territory[territory]
		if was == owner {
			continue
		}
		if owner != power && was != power && !(sameSide(owner, power) && !sameSide(was, power)) {
			t.Errorf("turn %d: %s changed hands from %s to %s while %s was playing",
				g.Turn, territory, was, owner, power)
		}
	}

	// Units do not appear from nowhere. Mobilised units are bought and paid for,
	// so a jump far beyond what any treasury could fund means something is
	// duplicating pieces.
	if grew := len(g.Pieces) - piecesBefore; grew > 60 {
		t.Errorf("turn %d: %s added %d pieces in one turn", g.Turn, power, grew)
	}
}

func writeTranscript(t *testing.T, runner *game.GameRunner, seed int64) {
	t.Helper()

	dir := os.Getenv("TRANSCRIPT_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	path := filepath.Join(dir, fmt.Sprintf("game-seed-%d.txt", seed))

	if err := runner.Transcript.SaveToFile(path); err != nil {
		t.Errorf("saving transcript: %v", err)
		return
	}
	t.Logf("transcript written to %s", path)
}

func summarise(t *testing.T, g *models.Game, runner *game.GameRunner, turnsPlayed int) {
	t.Helper()

	transcript := runner.GetTranscriptString()
	count := func(needle string) int { return strings.Count(transcript, needle) }

	moves := count("Moving ")
	battles := count("Battle begins")
	captures := count("captured")

	t.Logf("played %d turns over %d rounds", turnsPlayed, g.Turn)
	t.Logf("  moves %d, battles %d, captures %d, purchases %d, mobilisations %d",
		moves, battles, captures, count("Purchased"), count("Mobilized"))

	axis, allies := g.CountVictoryCities()
	t.Logf("  victory cities: Axis %d, Allies %d", axis, allies)
	for _, name := range g.TurnTakingPowers() {
		player := g.Players[name]
		t.Logf("  %-8s %2d territories, %3d IPCs", name, len(player.Territories), player.IPCs)
	}

	// A game where nothing happens would pass every invariant above while
	// telling us nothing, so assert the game was actually played.
	if moves == 0 {
		t.Error("no units moved in the entire game")
	}
	if battles == 0 {
		t.Error("no battles were fought in the entire game")
	}
	if turnsPlayed < 2 {
		t.Errorf("only %d turns were played", turnsPlayed)
	}
}
