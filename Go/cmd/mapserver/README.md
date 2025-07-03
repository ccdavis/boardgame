# Map Server

This is a web-based map viewer for the board game engine.

## Usage

```bash
# Run with a game definition file
go run cmd/mapserver/main.go test_game.gdf

# Run with a saved game file
go run cmd/mapserver/main.go game_save.json

# Specify custom port (default is 8080)
go run cmd/mapserver/main.go test_game.gdf 3000
```

Then open your browser to http://localhost:8080

## Features

- Interactive world map showing territories
- Color-coded by player ownership
- Shows piece counts on each territory
- Click territories to see detailed information
- Displays connections between territories
- Distinguishes between land and sea territories

## Map Layout

The map uses a coordinate system where:
- X: 0-100 (west to east)
- Y: 0-80 (north to south)

Territory positions are defined in `game/territories.go`