package main

import (
	"boardgame/webserver"
	"flag"
	"fmt"
	"os"
)

func main() {
	port := flag.Int("port", 8080, "Port to run the web server on")
	flag.Parse()

	fmt.Printf("Starting Axis & Allies Web Server on port %d\n", *port)
	fmt.Printf("Access the game at: http://localhost:%d\n", *port)

	server := webserver.NewServer(*port)
	if err := server.Start(); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
