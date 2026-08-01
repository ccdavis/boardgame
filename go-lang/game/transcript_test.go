package game

import (
	"strings"
	"testing"
)

// Eight identical moves must read as one line, not eight.
func TestLogMove_AggregatesIdenticalMoves(t *testing.T) {
	transcript := NewGameTranscript("test")
	for i := 0; i < 8; i++ {
		transcript.LogMove("USA", "infantry", "Western US", "Mexico", "noncombat")
	}

	if len(transcript.Entries) != 1 {
		t.Fatalf("entries = %d, want 1 aggregated entry", len(transcript.Entries))
	}
	want := "Moving 8 infantry from Western US to Mexico (noncombat)"
	if got := transcript.Entries[0].Action; got != want {
		t.Errorf("action = %q, want %q", got, want)
	}
}

// Interleaved types within one run of moves still aggregate by type.
func TestLogMove_AggregatesAcrossInterleavedTypes(t *testing.T) {
	transcript := NewGameTranscript("test")
	transcript.LogMove("Germany", "infantry", "Berlin", "Poland", "combat")
	transcript.LogMove("Germany", "armor", "Berlin", "Poland", "combat")
	transcript.LogMove("Germany", "infantry", "Berlin", "Poland", "combat")
	transcript.LogMove("Germany", "fighter", "Berlin", "Poland", "combat")

	if len(transcript.Entries) != 3 {
		t.Fatalf("entries = %d, want 3 (infantry x2, armor, fighter)", len(transcript.Entries))
	}
	if got := transcript.Entries[0].Action; !strings.Contains(got, "2 infantry") {
		t.Errorf("first entry = %q, want it to count 2 infantry", got)
	}
	if got := transcript.Entries[2].Action; !strings.Contains(got, "1 fighter") {
		t.Errorf("third entry = %q, want a single fighter", got)
	}
}

// A non-move entry ends the run: later identical moves start a fresh count, so
// the report keeps its chronology around battles and phase changes.
func TestLogMove_RunEndsAtOtherEntries(t *testing.T) {
	transcript := NewGameTranscript("test")
	transcript.LogMove("Japan", "armor", "Manchuria", "China", "combat")
	transcript.LogAction("Japan", "something else happened")
	transcript.LogMove("Japan", "armor", "Manchuria", "China", "combat")

	if len(transcript.Entries) != 3 {
		t.Fatalf("entries = %d, want 3 (move, action, move)", len(transcript.Entries))
	}
}

// More than one of a pluralisable type reads naturally.
func TestLogMove_Pluralises(t *testing.T) {
	transcript := NewGameTranscript("test")
	transcript.LogMove("UK", "fighter", "London", "France", "combat")
	transcript.LogMove("UK", "fighter", "London", "France", "combat")

	want := "Moving 2 fighters from London to France (combat)"
	if got := transcript.Entries[0].Action; got != want {
		t.Errorf("action = %q, want %q", got, want)
	}
}
