package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/parsing"
)

type TracingParser struct {
	*parsing.Parser
	sectionName string
}

func (tp *TracingParser) NextToken() *parsing.Token {
	tok := tp.Parser.NextToken()
	if tp.sectionName == "placement" && (tok.Type == parsing.END_OF_FILE || tok.Type == parsing.SEMICOLON) {
		fmt.Printf("[%s] NextToken() = %v\n", tp.sectionName, tok)
	}
	return tok
}

func (tp *TracingParser) Skip() {
	tok := tp.NextToken()
	if tp.sectionName == "placement" && (tok.Type == parsing.END_OF_FILE || tok.Type == parsing.SEMICOLON) {
		fmt.Printf("[%s] Skip() consuming %v\n", tp.sectionName, tok)
	}
	tp.Parser.Skip()
}

func (tp *TracingParser) tryParseTerritoryName() string {
	if tp.sectionName == "placement" {
		fmt.Printf("[%s] tryParseTerritoryName() starting\n", tp.sectionName)
	}
	
	var parts []string
	for tp.NextToken().Type == parsing.IDENTIFIER {
		tp.Skip()
		parts = append(parts, tp.LastTokenAsString())
	}

	result := strings.Join(parts, " ")
	if tp.sectionName == "placement" {
		fmt.Printf("[%s] tryParseTerritoryName() = %q\n", tp.sectionName, result)
	}
	return result
}

func main() {
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	baseParser := parsing.NewParser(file)
	parser := &TracingParser{Parser: baseParser}
	
	// Create a game parser that uses our tracing parser
	// Since we can't easily wrap the GameParser, let's just trace the placement section
	gp := parsing.NewGameParser(file)
	gs, err := gp.Load()
	
	if err != nil {
		fmt.Printf("Parser failed with error: %v\n", err)
	} else {
		fmt.Printf("Parser succeeded! Placement entries: %d\n", len(gs.Placement))
	}
}