// gamebatch plays many seeded all-NPC games and reports what happened in each:
// winner, length, the final position, and anything that looks wrong -- state
// validation failures, turn errors, powers wiped out, mobilisation backlogs.
//
// Output is one TSV row per game on stdout (aggregate with anything), with
// oddities flagged on stderr. Transcripts for flagged games are written to the
// directory named by -transcripts.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"boardgame/game"
	"boardgame/models"
	"boardgame/parser"
)

func main() {
	games := flag.Int("n", 100, "number of games to play")
	maxRounds := flag.Int("rounds", 26, "round cap per game")
	firstSeed := flag.Int64("seed", 1, "first seed; games use seed, seed+1, ...")
	gdf := flag.String("board", "../aaa.gdf", "board file")
	transcriptDir := flag.String("transcripts", "", "directory for flagged games' transcripts")
	flag.Parse()
	cap := *maxRounds

	fmt.Println("seed\trounds\twinner\taxisVC\talliesVC\tbattles\tcaptures\tlandings\tviolations\tflags\t" +
		"powers(territories/ipcs/units)")

	for i := 0; i < *games; i++ {
		seed := *firstSeed + int64(i)
		playOne(seed, *gdf, *transcriptDir, cap)
	}
}

func playOne(seed int64, gdf, transcriptDir string, maxRounds int) {
	p, err := parser.NewParser(gdf)
	if err != nil {
		panic(err)
	}
	g, err := p.Parse()
	if err != nil {
		panic(err)
	}

	powers := g.TurnTakingPowers()
	runner := game.NewGameRunner(g, fmt.Sprintf("batch seed %d", seed))
	for i, name := range powers {
		runner.RegisterSeededNPC(name, "normal", seed+int64(i)*7919)
	}
	if err := runner.Controller.StartGame(); err != nil {
		panic(err)
	}

	var flags []string
	winner := "-"
	rounds := 0

	for turn := 0; turn < maxRounds*len(powers); turn++ {
		power := g.CurrentPower
		npc, ok := runner.NPCPlayers[power]
		if !ok {
			flags = append(flags, fmt.Sprintf("no-ai:%s", power))
			break
		}
		if err := npc.TakeTurn(runner.Controller, runner.Transcript); err != nil {
			flags = append(flags, fmt.Sprintf("turn-error:%s:%v", power, err))
			break
		}
		if problems := g.Validate(); len(problems) > 0 {
			flags = append(flags, fmt.Sprintf("invalid-state:%s:%s", power, problems[0]))
			break
		}
		for name, player := range g.Players {
			if player.IPCs < 0 {
				flags = append(flags, fmt.Sprintf("negative-ipcs:%s", name))
			}
		}
		rounds = g.Turn
		if who, won, _ := runner.Controller.CheckVictoryCondition(); won {
			winner = who
			break
		}
	}

	// A game that ends at the cap while a side is at its sustained threshold
	// is worth flagging: with more rounds it would likely have been decided.
	if winner == "-" && g.VictoryHoldSide != "" {
		flags = append(flags, fmt.Sprintf("hold-at-cap:%s:%d", g.VictoryHoldSide, g.VictoryHoldRounds))
	}

	// Post-game oddity sweep.
	for _, name := range powers {
		player := g.Players[name]
		if len(player.Territories) == 0 {
			flags = append(flags, "wiped-out:"+name)
		}
		if backlog := len(g.PurchasedUnits[name]); backlog > 0 {
			flags = append(flags, fmt.Sprintf("backlog:%s:%d", name, backlog))
		}
	}
	transcript := runner.GetTranscriptString()
	count := func(needle string) int { return strings.Count(transcript, needle) }
	if n := count("Could not place"); n > 0 {
		flags = append(flags, fmt.Sprintf("could-not-place:%d", n))
	}
	if n := count("could not launch"); n > 0 {
		flags = append(flags, fmt.Sprintf("launch-failed:%d", n))
	}
	if n := count("failed"); n > 0 {
		flags = append(flags, fmt.Sprintf("failures:%d", n))
	}

	axis, allies := g.CountVictoryCities()

	var standing []string
	for _, name := range powers {
		player := g.Players[name]
		units := 0
		for _, piece := range g.Pieces {
			if piece.Owner == player {
				units++
			}
		}
		standing = append(standing, fmt.Sprintf("%s:%d/%d/%d", name, len(player.Territories), player.IPCs, units))
	}

	flagText := "-"
	if len(flags) > 0 {
		flagText = strings.Join(flags, ",")
		if transcriptDir != "" {
			path := filepath.Join(transcriptDir, fmt.Sprintf("flagged-seed-%d.txt", seed))
			if err := runner.Transcript.SaveToFile(path); err == nil {
				fmt.Fprintf(os.Stderr, "seed %d flagged (%s): transcript at %s\n", seed, flagText, path)
			}
		}
	}

	fmt.Printf("%d\t%d\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%s\t%s\n",
		seed, rounds, winner, axis, allies,
		count("Battle begins"), count("captured -"), count("troops landed"),
		countViolations(g, transcript), flagText, strings.Join(standing, " "))
}

// countViolations counts strict-neutral battles as a proxy for violations.
func countViolations(g *models.Game, transcript string) int {
	n := 0
	for _, name := range []string{"Turkey", "Switzerland", "Afghanistan", "Mongolia"} {
		n += strings.Count(transcript, "Battle begins in "+name)
	}
	return n
}
