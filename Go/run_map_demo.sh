#!/bin/bash

echo "Building and running the map server demo..."
echo

# Create a saved game with some changes
echo "Creating a demo saved game..."
go run cmd/persist_demo/main.go ../test_game.gdf demo_save.json

echo
echo "Starting map server..."
echo "Open your browser to:"
echo "  - http://localhost:8080 (basic map)"
echo "  - http://localhost:8080/enhanced (enhanced Axis & Allies style map)"
echo "  - http://localhost:8080/worldmap (realistic world map view)"
echo
echo "Press Ctrl+C to stop the server"
echo

# Start the map server with the saved game
go run cmd/mapserver/main.go demo_save.json