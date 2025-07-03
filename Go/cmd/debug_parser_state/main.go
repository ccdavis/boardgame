package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gs := parsing.NewGameState()

	// Parse everything up to placement
	fmt.Println("Parsing players...")
	parser.ParsePlayers(gs)
	fmt.Println("Parsing turn...")
	parser.ParseTurn(gs)
	fmt.Println("Parsing territories...")
	parser.ParseTerritories(gs)
	fmt.Println("Parsing map...")
	parser.ParseMap(gs)
	fmt.Println("Parsing units...")
	parser.ParseUnits(gs)
	fmt.Println("Parsing containers...")
	parser.ParseContainers(gs)
	
	// Now check what token we're at
	fmt.Printf("\nAbout to parse placement. Next token: %v\n", parser.NextToken())
	
	// Try to parse placement with debugging
	fmt.Println("\nStarting placement parse...")
	if parser.NextToken().Type != parsing.PLACEMENT {
		fmt.Printf("ERROR: Expected PLACEMENT token, got %v\n", parser.NextToken())
		return
	}
	
	parser.Match(parsing.PLACEMENT)
	fmt.Println("Matched PLACEMENT token")
	
	// Try one iteration manually
	fmt.Printf("Next token after PLACEMENT: %v\n", parser.NextToken())
	if parser.NextToken().Type == parsing.END_OF_FILE {
		fmt.Println("Already at EOF!")
		return
	}
	
	// Try to get territory name
	var parts []string
	for parser.NextToken().Type == parsing.IDENTIFIER {
		parser.Skip()
		parts = append(parts, parser.LastTokenAsString())
		fmt.Printf("  Got part: %q\n", parser.LastTokenAsString())
	}
	
	if len(parts) == 0 {
		fmt.Printf("No identifier tokens found. Next token: %v\n", parser.NextToken())
	} else {
		fmt.Printf("Got territory: %q\n", strings.Join(parts, " "))
	}
}