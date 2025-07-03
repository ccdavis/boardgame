package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/boardgame/Go/parsing"
)

func main() {
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	
	// Start parser in goroutine
	done := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Parser panicked: %v\n", r)
			}
		}()
		
		_, err := parser.Load()
		if err != nil {
			fmt.Printf("Parser error: %v\n", err)
		}
		done <- true
	}()

	// Monitor goroutines
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	timeout := time.After(5 * time.Second)
	count := 0
	
	for {
		select {
		case <-done:
			fmt.Println("Parser completed successfully!")
			return
		case <-ticker.C:
			count++
			fmt.Printf("Still running after %d seconds... goroutines: %d\n", count, runtime.NumGoroutine())
			
			// After 3 seconds, show stack trace
			if count == 3 {
				fmt.Println("\nGoroutine stack traces:")
				buf := make([]byte, 1<<16)
				runtime.Stack(buf, true)
				fmt.Printf("%s\n", buf)
			}
		case <-timeout:
			fmt.Println("\nTimeout! Parser is stuck.")
			return
		}
	}
}