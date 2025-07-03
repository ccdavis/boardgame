# Board Game Engine - Go Implementation

A Go implementation of a strategy board game engine similar to "Axis and Allies" using an Entity Component System (ECS) design pattern. This implementation includes game state persistence and a web-based map visualization.

## Features

- **Game Parser**: Loads game definitions from GDF (Game Definition Format) files
- **ECS Architecture**: Clean separation of entities, components, and systems
- **Game Persistence**: Save and load game states to/from JSON
- **Web Map Visualization**: Interactive browser-based map display
- **Flexible Game Rules**: Support for various unit types, territories, and player configurations

## Project Structure

```
Go/
├── game/               # Core game logic
│   ├── components.go   # Game entities (Player, Territory, Piece)
│   ├── systems.go      # Game logic (e.g., ChangeOwnership)
│   ├── persist.go      # Save/load functionality
│   └── territories.go  # Map coordinates and player colors
├── parsing/            # GDF file parser
│   ├── game_parser.go  # Game-specific parser
│   ├── parser.go       # Base parser functionality
│   ├── scanner.go      # Lexical scanner
│   └── token.go        # Token definitions
├── cmd/                # Executable programs
│   ├── parser/         # GDF to JSON converter
│   ├── tester/         # Test runner
│   ├── persist_demo/   # Persistence demo
│   └── mapserver/      # Web map server
└── test files...       # .gdf game definitions
```

## Installation

```bash
# From the Go directory
go mod init github.com/boardgame/Go
go mod tidy
```

## Usage

### 1. Parsing Game Files

Convert GDF files to JSON:

```bash
go run cmd/parser/main.go ../test_game.gdf
```

### 2. Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./game -v
```

### 3. Game Persistence

The game supports saving and loading game states:

```go
// Save game to file
err := game.SaveToFile("savegame.json")

// Load game from file
loadedGame, err := game.LoadGameFromFile("savegame.json")

// Save to JSON string
jsonData, err := game.SaveToJSON()

// Load from JSON string
game, err := game.LoadGameFromJSON(jsonData)
```

Demo program:

```bash
# First run creates a save file
go run cmd/persist_demo/main.go ../test_game.gdf

# Second run loads the saved game
go run cmd/persist_demo/main.go ../test_game.gdf
```

### 4. Web Map Visualization

The map server provides an interactive web interface to view the game board:

```bash
# Run with a game definition file
go run cmd/mapserver/main.go ../test_game.gdf

# Run with a saved game file
go run cmd/mapserver/main.go game_save.json

# Specify custom port (default is 8080)
go run cmd/mapserver/main.go ../test_game.gdf 3000
```

Then open your browser to:
- http://localhost:8080 - Basic map view
- http://localhost:8080/enhanced - Enhanced Axis & Allies style map

#### Map Features

- **Interactive Territories**: Click any territory to see details
- **Player Colors**: Each player has a distinct color
- **Unit Counts**: Shows total units on each territory
- **Connections**: Dotted lines show territory connections
- **Zoom Controls**: Use buttons or keyboard (+/-) to zoom
- **Territory Types**: Visual distinction between land and sea zones

#### Keyboard Shortcuts

- `c` - Toggle connection lines
- `+`/`=` - Zoom in
- `-`/`_` - Zoom out

## API Reference

### Game Structure

```go
type Game struct {
    Board                []*Territory
    Players              []*Player
    Pieces               map[PieceID]*Piece
    GlobalPieceTemplates map[string]*Piece
}

type Territory struct {
    Name        string
    Owner       *Player
    Pieces      []PieceID
    Terrain     TerrainType
    Production  int
    ConnectedTo []*Territory
}

type Player struct {
    Name           string
    NPC            bool
    Active         bool
    Territories    []*Territory
    PieceTemplates map[string]*Piece
}

type Piece struct {
    Capacity  int16
    Cost      int16
    Attack    int16
    Defend    int16
    Movement  int16
    Terrain   TerrainType
    Name      string
    CanCarry  []string
    Holding   []PieceID
}
```

### Core Functions

```go
// Create a new game from parsed GDF
func NewGame(loadedGame *parsing.GameState) (*Game, error)

// Change territory ownership
func ChangeOwnership(territory *Territory, to *Player)

// Save game to file
func (g *Game) SaveToFile(filename string) error

// Load game from file
func LoadGameFromFile(filename string) (*Game, error)
```

## GDF File Format

The Game Definition Format (GDF) is a custom format for defining game states:

```
players: USA USSR Germany Japan UK

territories:
    Eastern_United_States: owner=USA, type=land, production=12
    North_Atlantic: owner=UK, type=water, production=0

map:
    Eastern_United_States: [Western_United_States, North_Atlantic]

units:
    infantry: cost=3, attack=1, defend=2, movement=1, land
    battleship: cost=20, attack=4, defend=4, movement=2, water

placement:
    Eastern_United_States: infantry=10, armor=5
```

## Examples

### Creating and Saving a Game

```go
// Load game from GDF
file, _ := os.Open("game.gdf")
parser := parsing.NewGameParser(file)
gameState, _ := parser.Load()

// Create game
game, _ := game.NewGame(gameState)

// Make changes
game.ChangeOwnership(game.Board[0], game.Players[1])

// Save game
game.SaveToFile("savegame.json")
```

### Running a Map Server Demo

```bash
# Use the provided demo script
chmod +x run_map_demo.sh
./run_map_demo.sh
```

## Development

### Adding New Features

1. **New Unit Types**: Add to the `units` section in GDF files
2. **New Territories**: Add to `territories.go` with coordinates
3. **New Game Rules**: Implement in `systems.go`
4. **New Player Colors**: Add to `PlayerColors` map in `territories.go`

### Running Linters

No linting tools are currently configured. Follow the existing Go code style conventions.

## Future Enhancements

- [ ] Combat resolution system
- [ ] Movement validation
- [ ] Production and purchasing
- [ ] Turn management
- [ ] AI players
- [ ] Multiplayer support
- [ ] Save game history/replay
- [ ] Mobile-responsive map interface

## License

This project is part of a programming exploration and learning exercise. See the parent directory README for more information about the project's history and goals.