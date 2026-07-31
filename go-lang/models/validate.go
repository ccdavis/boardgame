package models

import (
	"fmt"
	"sort"
)

// Problem is a defect in a game's state.
type Problem struct {
	Kind   string
	Detail string
}

func (p Problem) String() string { return fmt.Sprintf("%-18s %s", p.Kind, p.Detail) }

// Validate checks that a game holds together.
//
// This exists because the test suite had a specific blind spot: fixtures
// hand-built state that the parser could not produce, in both directions. Some
// set Player.Side, which no board could supply until the grammar learned about
// sides. The most-used fixture omitted it entirely, which matched production's
// broken behaviour by accident and so could never catch the bug.
//
// Checking invariants rather than construction closes that gap. It does not
// matter how a game was built, only that it satisfies what a parsed game
// satisfies.
func (g *Game) Validate() []Problem {
	var problems []Problem

	add := func(kind, format string, args ...any) {
		problems = append(problems, Problem{Kind: kind, Detail: fmt.Sprintf(format, args...)})
	}

	// Turn order must name real powers.
	for _, name := range g.PlayerOrder {
		if _, ok := g.Players[name]; !ok {
			add("UNKNOWN_POWER", "turn order names %q, which is not a player", name)
		}
	}

	// Every power that plays needs a side and a real capital. Without these the
	// alliance checks, the neutral rules and the victory condition are all inert
	// -- allied powers attack each other and the game can never end.
	for _, name := range sortedKeys(g.Players) {
		player := g.Players[name]
		if !player.TakesTurns {
			continue
		}
		if player.Side == "" {
			add("NO_SIDE", "%q takes turns but has no side, so it has no allies and no enemies", name)
		}
		if player.Capital == "" {
			add("NO_CAPITAL", "%q takes turns but has no capital", name)
		} else if _, ok := g.Board[player.Capital]; !ok {
			add("BAD_CAPITAL", "%q has capital %q, which is not a territory", name, player.Capital)
		}
	}

	problems = append(problems, g.validateOwnership()...)
	problems = append(problems, g.validatePieces()...)
	return problems
}

// validateOwnership checks the two directions of the owner index agree.
//
// Territory.Owner and Player.Territories are a hand-maintained bidirectional
// index. When they disagree, income is computed differently depending on which
// side the caller happened to walk.
func (g *Game) validateOwnership() []Problem {
	var problems []Problem

	for _, name := range sortedKeys(g.Board) {
		territory := g.Board[name]
		if territory.Owner == nil {
			problems = append(problems, Problem{"NO_OWNER",
				fmt.Sprintf("territory %q has no owner", name)})
			continue
		}
		if _, ok := g.Players[territory.Owner.Name]; !ok {
			problems = append(problems, Problem{"UNKNOWN_OWNER",
				fmt.Sprintf("territory %q is owned by %q, which is not a player",
					name, territory.Owner.Name)})
			continue
		}

		listed := false
		for _, held := range territory.Owner.Territories {
			if held == territory {
				listed = true
				break
			}
		}
		if !listed {
			problems = append(problems, Problem{"OWNER_DESYNC",
				fmt.Sprintf("territory %q says it belongs to %q, but %q does not list it",
					name, territory.Owner.Name, territory.Owner.Name)})
		}
	}

	for _, name := range sortedKeys(g.Players) {
		for _, territory := range g.Players[name].Territories {
			if territory.Owner == nil || territory.Owner.Name != name {
				owner := "nobody"
				if territory.Owner != nil {
					owner = territory.Owner.Name
				}
				problems = append(problems, Problem{"OWNER_DESYNC",
					fmt.Sprintf("%q claims %q, but that territory says it belongs to %s",
						name, territory.Name, owner)})
			}
		}
	}
	return problems
}

// validatePieces checks every piece is somewhere exactly once.
//
// A piece in no territory and no hold is leaked: it stays in Game.Pieces
// forever, invisible on the board but still counted by unit tallies. That is
// what happens to a transport's cargo when the transport is destroyed, because
// cargo lives only in Piece.Holding and is absent from Territory.Pieces.
func (g *Game) validatePieces() []Problem {
	var problems []Problem

	location := make(map[int]string, len(g.Pieces))
	note := func(id int, where string) {
		if previous, seen := location[id]; seen {
			problems = append(problems, Problem{"DUPLICATE_PIECE",
				fmt.Sprintf("piece %d is in both %s and %s", id, previous, where)})
			return
		}
		location[id] = where
	}

	for _, name := range sortedKeys(g.Board) {
		for _, id := range g.Board[name].Pieces {
			if _, ok := g.Pieces[id]; !ok {
				problems = append(problems, Problem{"MISSING_PIECE",
					fmt.Sprintf("territory %q lists piece %d, which does not exist", name, id)})
				continue
			}
			note(id, fmt.Sprintf("territory %q", name))
		}
	}

	for _, id := range sortedIntKeys(g.Pieces) {
		for _, cargoID := range g.Pieces[id].Holding {
			if _, ok := g.Pieces[cargoID]; !ok {
				problems = append(problems, Problem{"MISSING_PIECE",
					fmt.Sprintf("piece %d carries piece %d, which does not exist", id, cargoID)})
				continue
			}
			note(cargoID, fmt.Sprintf("the hold of piece %d", id))
		}
	}

	for _, id := range sortedIntKeys(g.Pieces) {
		if _, placed := location[id]; !placed {
			problems = append(problems, Problem{"ORPHANED_PIECE",
				fmt.Sprintf("piece %d (%s) is in no territory and is carried by nothing",
					id, g.Pieces[id].Name)})
		}
	}
	return problems
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func sortedIntKeys[V any](m map[int]V) []int {
	out := make([]int, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Ints(out)
	return out
}
