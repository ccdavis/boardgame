package models

import (
	"fmt"
	"sort"
)

// GraphProblemKind classifies a defect in the board adjacency graph.
type GraphProblemKind string

const (
	// GraphAsymmetric means A lists B as a neighbour but B does not list A.
	GraphAsymmetric GraphProblemKind = "ASYMMETRIC"
	// GraphSelfLoop means a territory lists itself as its own neighbour.
	GraphSelfLoop GraphProblemKind = "SELF_LOOP"
	// GraphDuplicate means a territory lists the same neighbour more than once.
	GraphDuplicate GraphProblemKind = "DUPLICATE"
	// GraphIsolated means a territory has no neighbours at all.
	GraphIsolated GraphProblemKind = "ISOLATED"
	// GraphDisconnected means a territory is unreachable from the rest of the board.
	GraphDisconnected GraphProblemKind = "DISCONNECTED"
	// GraphDanglingOwner means a territory's owner is not a registered player.
	GraphDanglingOwner GraphProblemKind = "DANGLING_OWNER"
)

// GraphProblem is a single defect found by ValidateGraph.
type GraphProblem struct {
	Kind   GraphProblemKind
	A      string // the territory the problem is reported against
	B      string // the other endpoint, where the problem concerns an edge
	Detail string
}

func (p GraphProblem) String() string {
	if p.B != "" {
		return fmt.Sprintf("%-13s %q -- %q  %s", p.Kind, p.A, p.B, p.Detail)
	}
	return fmt.Sprintf("%-13s %q  %s", p.Kind, p.A, p.Detail)
}

// ValidateGraph checks the board's adjacency graph for structural defects.
//
// The board is a symmetric, connected, simple graph: adjacency means two regions
// touch, which is inherently mutual. The .gdf Map section declares edges from
// each endpoint independently and is free to declare an edge from only one side,
// so this catches data that says something the geometry cannot.
//
// Problems are returned sorted, so output is stable across runs and diffable.
func (g *Game) ValidateGraph() []GraphProblem {
	var problems []GraphProblem

	names := make([]string, 0, len(g.Board))
	for name := range g.Board {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		terr := g.Board[name]

		if terr.Owner != nil {
			if _, ok := g.Players[terr.Owner.Name]; !ok {
				problems = append(problems, GraphProblem{
					Kind:   GraphDanglingOwner,
					A:      name,
					Detail: fmt.Sprintf("owner %q is not a registered player", terr.Owner.Name),
				})
			}
		}

		if len(terr.ConnectedTo) == 0 {
			problems = append(problems, GraphProblem{
				Kind: GraphIsolated, A: name,
				Detail: "no neighbours; it cannot be reached or left",
			})
			continue
		}

		seen := make(map[string]bool, len(terr.ConnectedTo))
		for _, nb := range terr.ConnectedTo {
			if nb == terr {
				problems = append(problems, GraphProblem{
					Kind: GraphSelfLoop, A: name, Detail: "lists itself as a neighbour",
				})
				continue
			}
			if seen[nb.Name] {
				problems = append(problems, GraphProblem{
					Kind: GraphDuplicate, A: name, B: nb.Name,
					Detail: "listed more than once",
				})
				continue
			}
			seen[nb.Name] = true

			if !adjacentTo(nb, terr) {
				problems = append(problems, GraphProblem{
					Kind: GraphAsymmetric, A: name, B: nb.Name,
					Detail: fmt.Sprintf("%q does not list %q back", nb.Name, name),
				})
			}
		}
	}

	problems = append(problems, g.findDisconnected(names)...)
	return problems
}

// findDisconnected reports every territory not reachable from the first one.
func (g *Game) findDisconnected(sortedNames []string) []GraphProblem {
	if len(sortedNames) == 0 {
		return nil
	}

	root := sortedNames[0]
	reached := map[string]bool{root: true}
	queue := []string{root}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nb := range g.Board[cur].ConnectedTo {
			if !reached[nb.Name] {
				reached[nb.Name] = true
				queue = append(queue, nb.Name)
			}
		}
	}

	var problems []GraphProblem
	for _, name := range sortedNames {
		if !reached[name] {
			problems = append(problems, GraphProblem{
				Kind: GraphDisconnected, A: name,
				Detail: fmt.Sprintf("not reachable from %q", root),
			})
		}
	}
	return problems
}

func adjacentTo(from, to *Territory) bool {
	for _, t := range from.ConnectedTo {
		if t == to {
			return true
		}
	}
	return false
}
