package main

import (
	"fmt"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	// Test with the exact end of the file
	content := "Hawaian Sea: 1 sub;"
	
	scanner := parsing.NewScanner(strings.NewReader(content))
	
	tokens := []string{}
	for i := 0; i < 20; i++ {
		tok := scanner.NextToken()
		tokens = append(tokens, fmt.Sprintf("%v", tok))
		if tok.Type == parsing.END_OF_FILE {
			break
		}
	}
	
	fmt.Println("Tokens:")
	for i, tok := range tokens {
		fmt.Printf("%d: %s\n", i, tok)
	}
}