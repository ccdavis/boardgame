package main

import (
	"fmt"
	"log"
	"os"

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

	scanner := parsing.NewScanner(file)
	
	// Read and print all tokens
	for {
		token := scanner.NextToken()
		fmt.Printf("Line %d: %s\n", token.Line, token.String())
		if token.Type == parsing.END_OF_FILE {
			break
		}
	}
}