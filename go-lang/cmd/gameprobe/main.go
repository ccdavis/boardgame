// Probe: play a seeded all-NPC game and dump where the pieces ended up.
package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"boardgame/game"
	"boardgame/parser"
)

func main() {
	seed := int64(7)
	if len(os.Args) > 1 {
		if s, err := strconv.ParseInt(os.Args[1], 10, 64); err == nil {
			seed = s
		}
	}
	rounds := 26

	p, err := parser.NewParser("../aaa.gdf")
	if err != nil {
		panic(err)
	}
	g, err := p.Parse()
	if err != nil {
		panic(err)
	}
	powers := g.TurnTakingPowers()
	runner := game.NewGameRunner(g, "probe")
	for i, name := range powers {
		runner.RegisterSeededNPC(name, "normal", seed+int64(i)*7919)
	}
	runner.Controller.StartGame()
	for turn := 0; turn < rounds*len(powers); turn++ {
		npc := runner.NPCPlayers[g.CurrentPower]
		if err := npc.TakeTurn(runner.Controller, runner.Transcript); err != nil {
			panic(err)
		}
	}

	fmt.Printf("=== final state, seed %d, %d rounds ===\n", seed, rounds)
	fmt.Println("\n--- unplaced purchases (mobilisation backlog) ---")
	for _, name := range powers {
		pending := g.PurchasedUnits[name]
		byType := map[string]int{}
		for _, u := range pending {
			byType[u.Type]++
		}
		fmt.Printf("  %-8s %3d unplaced  %v\n", name, len(pending), byType)
	}
	for _, name := range powers {
		player := g.Players[name]
		type spot struct {
			name  string
			count int
		}
		var spots []spot
		total := 0
		for _, t := range player.Territories {
			n := 0
			for _, id := range t.Pieces {
				if piece := g.Pieces[id]; piece != nil && piece.Owner == player {
					n++
				}
			}
			if n > 0 {
				spots = append(spots, spot{t.Name, n})
				total += n
			}
		}
		sort.Slice(spots, func(i, j int) bool { return spots[i].count > spots[j].count })
		fmt.Printf("\n%s: %d units on own soil, treasury %d, income %d territories\n",
			name, total, player.IPCs, len(player.Territories))
		for i, s := range spots {
			if i >= 6 {
				fmt.Printf("  ... and %d more territories\n", len(spots)-6)
				break
			}
			fmt.Printf("  %-28s %4d\n", s.name, s.count)
		}
	}
}
