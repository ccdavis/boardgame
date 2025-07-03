package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <game.gdf>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Failed to open file %s: %v", filename, err)
	}
	defer file.Close()

	fmt.Println("Starting parse...")
	
	// Test just the map section
	input := `Map
Germany : Norway;
Norway : Germany;`
	
	p2 := parsing.NewGameParser(strings.NewReader(input))
	
	fmt.Println("Parsing map section...")
	if err := p2.Match(parsing.MAP); err != nil {
		log.Fatal(err)
	}
	
	fmt.Println("MAP token matched")
	
	// Parse one entry manually
	fmt.Printf("Next token: %v\n", p2.NextToken())
}