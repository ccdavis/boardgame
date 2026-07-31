package main

import (
	"strings"
	"testing"

	"boardgame/game"
	"boardgame/parser"
)

// playSeededRounds plays a few full rounds on the real board and returns the
// transcript.
func playSeededRounds(t *testing.T, seed int64, rounds int) string {
	t.Helper()

	p, err := parser.NewParser("../aaa.gdf")
	if err != nil {
		t.Fatalf("opening board: %v", err)
	}
	g, err := p.Parse()
	if err != nil {
		t.Fatalf("parsing board: %v", err)
	}

	powers := g.TurnTakingPowers()
	runner := game.NewGameRunner(g, "replay check")
	for i, name := range powers {
		runner.RegisterSeededNPC(name, "normal", seed+int64(i)*7919)
	}
	if err := runner.Controller.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}

	for turn := 0; turn < rounds*len(powers); turn++ {
		power := g.CurrentPower
		npc, ok := runner.NPCPlayers[power]
		if !ok {
			t.Fatalf("no AI for %q", power)
		}
		if err := npc.TakeTurn(runner.Controller, runner.Transcript); err != nil {
			t.Fatalf("turn %d, %s: %v", g.Turn, power, err)
		}
	}
	return runner.GetTranscriptString()
}

// The same seed must produce the same game.
//
// This held only on paper for a long time: the dice were seeded, but battles
// were resolved and units mobilised in Go map-iteration order, which differs on
// every run -- so each battle consumed different rolls and "replay with
// GAME_SEED=N" replayed a different game. Go randomises map iteration per range
// statement, so two games in one process are enough to catch a regression.
func TestSameSeedSameGame(t *testing.T) {
	stripClock := func(s string) string {
		// The header and footer carry wall-clock times; everything else must match.
		lines := strings.Split(s, "\n")
		kept := lines[:0]
		for _, line := range lines {
			if strings.Contains(line, "Started: ") || strings.Contains(line, "Game Duration") {
				continue
			}
			kept = append(kept, line)
		}
		return strings.Join(kept, "\n")
	}

	first := stripClock(playSeededRounds(t, 4242, 4))
	second := stripClock(playSeededRounds(t, 4242, 4))

	if first == second {
		return
	}

	// Show the first divergence, which names the nondeterministic decision.
	a, b := strings.Split(first, "\n"), strings.Split(second, "\n")
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			t.Fatalf("transcripts diverge at line %d:\n  run 1: %s\n  run 2: %s", i+1, a[i], b[i])
		}
	}
	t.Fatalf("transcripts differ in length: %d vs %d lines", len(a), len(b))
}
