package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/parsing"
)

type LoggingReader struct {
	r        io.Reader
	position int
	buffer   []byte
}

func NewLoggingReader(r io.Reader) *LoggingReader {
	content, _ := io.ReadAll(r)
	return &LoggingReader{
		r:      strings.NewReader(string(content)),
		buffer: content,
	}
}

func (lr *LoggingReader) Read(p []byte) (n int, err error) {
	n, err = lr.r.Read(p)
	if n > 0 {
		lr.position += n
		
		// Find what section we're in
		section := "unknown"
		beforePos := string(lr.buffer[:lr.position])
		if strings.Contains(beforePos, "Placement") {
			section = "Placement"
		} else if strings.Contains(beforePos, "Containers") {
			section = "Containers"  
		} else if strings.Contains(beforePos, "Units") {
			section = "Units"
		} else if strings.Contains(beforePos, "Map") {
			section = "Map"
		} else if strings.Contains(beforePos, "Territories") {
			section = "Territories"
		} else if strings.Contains(beforePos, "Turn") {
			section = "Turn"
		} else if strings.Contains(beforePos, "Players") {
			section = "Players"
		}
		
		// Log every 1000 characters
		if lr.position % 1000 == 0 {
			fmt.Printf("Read position %d (section: %s)\n", lr.position, section)
		}
	}
	return n, err
}

func main() {
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	loggingReader := NewLoggingReader(file)
	parser := parsing.NewGameParser(loggingReader)
	
	fmt.Println("Starting parse...")
	gs, err := parser.Load()
	
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Printf("Last position: %d\n", loggingReader.position)
	} else {
		fmt.Printf("Success! Placement entries: %d\n", len(gs.Placement))
	}
}