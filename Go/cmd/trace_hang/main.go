package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/boardgame/Go/parsing"
)

func getStackTrace() string {
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, true)
	return string(buf)
}

func main() {
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	
	done := make(chan bool)
	go func() {
		_, err := parser.Load()
		if err != nil {
			fmt.Printf("Parser error: %v\n", err)
		} else {
			fmt.Println("Parser completed successfully")
		}
		done <- true
	}()
	
	// Wait for completion or timeout
	select {
	case <-done:
		fmt.Println("Parser finished")
	case <-time.After(3 * time.Second):
		fmt.Println("Parser is hanging. Stack trace:")
		fmt.Println(getStackTrace())
	}
}