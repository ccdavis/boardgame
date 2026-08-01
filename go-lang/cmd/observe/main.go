// Command observe plays batches of all-computer games and reports what
// happened, per game and in aggregate. It exists to answer "is the game sane?"
// after a rules or AI change: fifty games say what one game cannot -- which
// side wins and how often, how long a war runs, what the computer players
// actually do with their navies and aircraft, and which incidents deserve a
// closer look.
//
//	go run ./cmd/observe -games 50 -board ../aaa.gdf -dir /tmp/observe
//
// Every game is seeded, so anything odd can be replayed exactly with
// GAME_SEED=<seed> go test -run TestFullGame -v .
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"boardgame/game"
	"boardgame/models"
	"boardgame/parser"
)

type stats struct {
	Seed        int64
	Rounds      int
	PlayerTurns int
	Winner      string
	AxisVC      int
	AlliesVC    int

	Battles      int
	SeaBattles   int
	Captured     int
	Held         int
	Broken       int
	Inconclusive int
	LongestFight int // rounds of the longest single battle

	Landings   int
	NewOps     int
	Leaks      int
	Abandoned  int
	Crashes    int
	Violations int // strict-neutral tolls paid

	Flips      map[string]int // territory -> ownership changes
	Eliminated []string
	FinalTerr  map[string]int
	FinalIPC   map[string]int
	Problems   []string // Validate() findings, if any ever appear

	// Trajectories, one sample per round boundary.
	VCByRound   [][2]int         // axis, allies
	TerrByRound []map[string]int // power -> territories held
}

var (
	battleRe = regexp.MustCompile(`⚔ Battle begins in (.+)`)
	resultRe = regexp.MustCompile(`after (\d+) rounds \(Att casualties: (\d+), Def casualties: (\d+)\)`)
)

func main() {
	games := flag.Int("games", 50, "how many games to play")
	baseSeed := flag.Int64("seed", 1000, "seed of the first game; game i uses seed+i")
	turns := flag.Int("turns", 40, "round cap per game")
	board := flag.String("board", "../aaa.gdf", "board file")
	dir := flag.String("dir", "observe-out", "output directory for transcripts and stats")
	flag.Parse()

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "output dir: %v\n", err)
		os.Exit(1)
	}

	all := make([]*stats, 0, *games)
	for i := 0; i < *games; i++ {
		seed := *baseSeed + int64(i)
		s, err := playOne(*board, seed, *turns, *dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "game seed %d: %v\n", seed, err)
			os.Exit(1)
		}
		all = append(all, s)
		fmt.Printf("%s\n", oneLine(s))
	}

	writeTSV(all, filepath.Join(*dir, "stats.tsv"))
	writeSeries(all, filepath.Join(*dir, "series.tsv"))
	fmt.Println()
	aggregate(all)
}

func playOne(board string, seed int64, maxTurns int, dir string) (*stats, error) {
	p, err := parser.NewParser(board)
	if err != nil {
		return nil, err
	}
	g, err := p.Parse()
	if err != nil {
		return nil, err
	}
	powers := g.TurnTakingPowers()

	runner := game.NewGameRunner(g, fmt.Sprintf("observed game, seed %d", seed))
	for i, name := range powers {
		runner.RegisterSeededNPC(name, "normal", seed+int64(i)*7919)
	}
	runner.SetMaxTurns(maxTurns)
	if err := runner.Controller.StartGame(); err != nil {
		return nil, err
	}

	s := &stats{
		Seed:      seed,
		Flips:     make(map[string]int),
		FinalTerr: make(map[string]int),
		FinalIPC:  make(map[string]int),
	}

	owners := ownerMap(g)
	crashesSeen := 0
	lastRound := 0
	for turn := 0; turn < maxTurns*len(powers); turn++ {
		power := g.CurrentPower
		npc, ok := runner.NPCPlayers[power]
		if !ok {
			return nil, fmt.Errorf("no AI for %q", power)
		}
		// Round boundary: record the score line so trajectories can be read
		// back out of the stats, not only the final state.
		if g.Turn != lastRound {
			lastRound = g.Turn
			axis, allies := g.CountVictoryCities()
			s.VCByRound = append(s.VCByRound, [2]int{axis, allies})
			terr := make(map[string]int, len(powers))
			for _, name := range powers {
				terr[name] = len(g.Players[name].Territories)
			}
			s.TerrByRound = append(s.TerrByRound, terr)
		}
		runner.Transcript.LogTurnStart(g.Turn, power)
		if err := npc.TakeTurn(runner.Controller, runner.Transcript); err != nil {
			return nil, fmt.Errorf("%s's turn: %w", power, err)
		}
		s.PlayerTurns++

		// Place aircraft losses in the record at the moment they happened.
		for ; crashesSeen < len(runner.Controller.CrashLog); crashesSeen++ {
			runner.Transcript.LogAction(power, "✈ "+runner.Controller.CrashLog[crashesSeen])
		}

		for _, problem := range g.Validate() {
			s.Problems = append(s.Problems,
				fmt.Sprintf("round %d after %s: %s", g.Turn, power, problem))
		}
		next := ownerMap(g)
		for name, owner := range next {
			if owners[name] != owner {
				s.Flips[name]++
			}
		}
		owners = next

		if winner, won, err := runner.Controller.CheckVictoryCondition(); err == nil && won {
			s.Winner = winner
			break
		}
		if g.Turn > maxTurns {
			break
		}
	}

	s.Rounds = g.Turn
	s.AxisVC, s.AlliesVC = g.CountVictoryCities()
	s.Crashes = len(runner.Controller.CrashLog)
	for _, name := range powers {
		player := g.Players[name]
		s.FinalTerr[name] = len(player.Territories)
		s.FinalIPC[name] = player.IPCs
		if len(player.Territories) == 0 {
			s.Eliminated = append(s.Eliminated, name)
		}
	}
	harvestTranscript(g, runner.Transcript, s)

	path := filepath.Join(dir, fmt.Sprintf("game-%d.txt", seed))
	if err := runner.Transcript.SaveToFile(path); err != nil {
		return nil, err
	}
	if len(runner.Controller.CrashLog) > 0 {
		crashes := strings.Join(runner.Controller.CrashLog, "\n") + "\n"
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("crashes-%d.txt", seed)), []byte(crashes), 0o644)
	}
	return s, nil
}

func ownerMap(g *models.Game) map[string]string {
	owners := make(map[string]string, len(g.Board))
	for name, territory := range g.Board {
		if territory.Owner != nil {
			owners[name] = territory.Owner.Name
		}
	}
	return owners
}

func harvestTranscript(g *models.Game, transcript *game.GameTranscript, s *stats) {
	for _, entry := range transcript.Entries {
		text := entry.Action
		switch {
		case strings.Contains(text, "Battle begins in"):
			s.Battles++
			if m := battleRe.FindStringSubmatch(text); m != nil {
				if territory, ok := g.Board[strings.TrimSpace(m[1])]; ok &&
					territory.Terrain == models.Water {
					s.SeaBattles++
				}
			}
		case resultRe.MatchString(text):
			m := resultRe.FindStringSubmatch(text)
			rounds := atoi(m[1])
			if rounds > s.LongestFight {
				s.LongestFight = rounds
			}
			switch {
			case strings.Contains(text, "captured"):
				s.Captured++
			case strings.Contains(text, "held"):
				s.Held++
			case strings.Contains(text, "broken off"):
				s.Broken++
			default:
				s.Inconclusive++
			}
		case strings.Contains(text, "troops landed in"):
			s.Landings++
		case strings.HasPrefix(text, "new Operation"):
			s.NewOps++
		case strings.Contains(text, "Intelligence leak"):
			s.Leaks++
		case strings.Contains(text, "(abandoned;") || strings.Contains(text, "; abandoned"):
			s.Abandoned++
		case strings.Contains(text, "violates") && strings.Contains(text, "neutral"):
			s.Violations++
		}
	}
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

func oneLine(s *stats) string {
	winner := s.Winner
	if winner == "" {
		winner = "-"
	}
	hot := topFlips(s.Flips, 1)
	return fmt.Sprintf(
		"seed %d: %2d rounds, winner %-8s VC %d-%d, battles %3d (%2d sea), cap %3d, landings %2d, ops %2d, leaks %d, crashes %2d, longest fight %2d, hottest %s",
		s.Seed, s.Rounds, winner, s.AxisVC, s.AlliesVC, s.Battles, s.SeaBattles,
		s.Captured, s.Landings, s.NewOps, s.Leaks, s.Crashes, s.LongestFight, hot)
}

func topFlips(flips map[string]int, n int) string {
	type kv struct {
		name  string
		count int
	}
	list := make([]kv, 0, len(flips))
	for name, count := range flips {
		list = append(list, kv{name, count})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].count != list[j].count {
			return list[i].count > list[j].count
		}
		return list[i].name < list[j].name
	})
	parts := make([]string, 0, n)
	for i := 0; i < n && i < len(list); i++ {
		parts = append(parts, fmt.Sprintf("%s x%d", list[i].name, list[i].count))
	}
	return strings.Join(parts, ", ")
}

func writeTSV(all []*stats, path string) {
	var b strings.Builder
	b.WriteString("seed\trounds\twinner\taxisVC\talliesVC\tbattles\tsea\tcaptured\theld\tbroken\tlandings\tops\tleaks\tabandoned\tcrashes\tlongest\teliminated\tproblems\n")
	for _, s := range all {
		fmt.Fprintf(&b, "%d\t%d\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\t%d\n",
			s.Seed, s.Rounds, s.Winner, s.AxisVC, s.AlliesVC, s.Battles, s.SeaBattles,
			s.Captured, s.Held, s.Broken, s.Landings, s.NewOps, s.Leaks, s.Abandoned,
			s.Crashes, s.LongestFight, strings.Join(s.Eliminated, ","), len(s.Problems))
	}
	os.WriteFile(path, []byte(b.String()), 0o644)
}

// writeSeries dumps each game's round-by-round score and holdings, one row
// per round, so trajectories can be analysed after the fact.
func writeSeries(all []*stats, path string) {
	if len(all) == 0 || len(all[0].TerrByRound) == 0 {
		return
	}
	powers := make([]string, 0)
	for name := range all[0].TerrByRound[0] {
		powers = append(powers, name)
	}
	sort.Strings(powers)

	var b strings.Builder
	b.WriteString("seed\tround\taxisVC\talliesVC")
	for _, name := range powers {
		b.WriteString("\t" + name)
	}
	b.WriteString("\n")
	for _, s := range all {
		for i, vc := range s.VCByRound {
			fmt.Fprintf(&b, "%d\t%d\t%d\t%d", s.Seed, i+1, vc[0], vc[1])
			for _, name := range powers {
				fmt.Fprintf(&b, "\t%d", s.TerrByRound[i][name])
			}
			b.WriteString("\n")
		}
	}
	os.WriteFile(path, []byte(b.String()), 0o644)
}

func aggregate(all []*stats) {
	wins := make(map[string]int)
	var rounds, battles, sea, landings, leaks, crashes, ops []int
	flipTotals := make(map[string]int)
	problems := 0
	for _, s := range all {
		winner := s.Winner
		if winner == "" {
			winner = "(undecided)"
		}
		wins[winner]++
		rounds = append(rounds, s.Rounds)
		battles = append(battles, s.Battles)
		sea = append(sea, s.SeaBattles)
		landings = append(landings, s.Landings)
		leaks = append(leaks, s.Leaks)
		crashes = append(crashes, s.Crashes)
		ops = append(ops, s.NewOps)
		problems += len(s.Problems)
		for name, count := range s.Flips {
			flipTotals[name] += count
		}
	}

	fmt.Printf("=== %d games ===\n", len(all))
	names := make([]string, 0, len(wins))
	for name := range wins {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return wins[names[i]] > wins[names[j]] })
	for _, name := range names {
		fmt.Printf("  wins: %-12s %d\n", name, wins[name])
	}
	fmt.Printf("  rounds:    %s\n", spread(rounds))
	fmt.Printf("  battles:   %s\n", spread(battles))
	fmt.Printf("  sea:       %s\n", spread(sea))
	fmt.Printf("  landings:  %s\n", spread(landings))
	fmt.Printf("  new ops:   %s\n", spread(ops))
	fmt.Printf("  leaks:     %s\n", spread(leaks))
	fmt.Printf("  crashes:   %s\n", spread(crashes))
	fmt.Printf("  state problems across all games: %d\n", problems)
	fmt.Printf("  most contested: %s\n", topFlipsTotal(flipTotals, 8))
}

func spread(values []int) string {
	if len(values) == 0 {
		return "-"
	}
	sorted := append([]int{}, values...)
	sort.Ints(sorted)
	total := 0
	for _, v := range sorted {
		total += v
	}
	return fmt.Sprintf("min %d, median %d, mean %.1f, max %d",
		sorted[0], sorted[len(sorted)/2], float64(total)/float64(len(sorted)), sorted[len(sorted)-1])
}

func topFlipsTotal(flips map[string]int, n int) string {
	return topFlips(flips, n)
}
