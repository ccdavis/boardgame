// Command layoutcheck validates a board's map geometry against its .gdf graph.
//
//	go run ./cmd/layoutcheck ../aaa.gdf
//
// Exists as a CLI, not only a test, so the authoring loop is a couple of
// seconds: edit the layout, run this, see what moved.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"boardgame/layout"
	"boardgame/parser"
)

func main() {
	quick := flag.Bool("quick", false, "skip the adjacency comparison")
	limit := flag.Int("limit", 40, "maximum problems to print (0 for all)")
	flag.Parse()

	gdfPath := flag.Arg(0)
	if gdfPath == "" {
		fmt.Fprintln(os.Stderr, "usage: layoutcheck [-quick] [-limit N] <board.gdf>")
		os.Exit(2)
	}

	p, err := parser.NewParser(gdfPath)
	if err != nil {
		fail("opening %s: %v", gdfPath, err)
	}
	game, err := p.Parse()
	if err != nil {
		fail("parsing %s: %v", gdfPath, err)
	}

	layoutPath := layout.PathFor(gdfPath)
	l, raw, err := layout.Load(layoutPath)
	if err != nil {
		fail("%v", err)
	}

	fmt.Printf("board  %s: %d territories\n", gdfPath, len(game.Board))
	fmt.Printf("layout %s: %d territories, %d KB, %d links\n",
		layoutPath, len(l.Territories), len(raw)/1024, len(l.Links))

	problems := l.Validate(game, layout.Options{SkipAdjacency: *quick})

	var errs, warns []layout.Problem
	for _, pr := range problems {
		if pr.IsError() {
			errs = append(errs, pr)
		} else {
			warns = append(warns, pr)
		}
	}

	report := func(label string, group []layout.Problem) {
		if len(group) == 0 {
			return
		}
		byKind := map[layout.ProblemKind]int{}
		for _, pr := range group {
			byKind[pr.Kind]++
		}
		kinds := make([]string, 0, len(byKind))
		for k := range byKind {
			kinds = append(kinds, string(k))
		}
		sort.Strings(kinds)

		fmt.Printf("\n%d %s:\n", len(group), label)
		for _, k := range kinds {
			fmt.Printf("  %-17s %d\n", k, byKind[layout.ProblemKind(k)])
		}
		fmt.Println()

		shown := group
		if *limit > 0 && len(shown) > *limit {
			shown = shown[:*limit]
		}
		for _, pr := range shown {
			fmt.Println("  " + pr.String())
		}
		if len(shown) < len(group) {
			fmt.Printf("  ... and %d more\n", len(group)-len(shown))
		}
	}

	report("error(s)", errs)
	report("warning(s)", warns)

	if len(errs) == 0 {
		fmt.Printf("\nOK: no errors")
		if len(warns) > 0 {
			fmt.Printf(" (%d warning(s): regions that touch on the map but are not "+
				"connected in the .gdf)", len(warns))
		}
		fmt.Println()
		return
	}
	os.Exit(1)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "layoutcheck: "+format+"\n", args...)
	os.Exit(1)
}
