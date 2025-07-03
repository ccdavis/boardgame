package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/boardgame/Go/parsing"
)

func testSection(content string, sectionName string) {
	fmt.Printf("\nTesting with content up to %s...\n", sectionName)
	
	done := make(chan bool)
	go func() {
		parser := parsing.NewGameParser(strings.NewReader(content))
		_, err := parser.Load()
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else {
			fmt.Printf("  Success!\n")
		}
		done <- true
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		fmt.Printf("  TIMEOUT - parser is hanging!\n")
	}
}

func main() {
	content, err := os.ReadFile("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	
	fullContent := string(content)
	
	// Test progressively larger portions
	sections := []string{"Turn", "Territories", "Map", "Units", "Containers", "Placement"}
	
	for _, section := range sections {
		idx := strings.Index(fullContent, "\n" + section)
		if idx == -1 {
			fmt.Printf("Could not find section %s\n", section)
			continue
		}
		
		testContent := fullContent[:idx]
		testSection(testContent, section)
	}
}