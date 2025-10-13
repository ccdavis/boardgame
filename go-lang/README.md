# Boardgame - Go Implementation

A Go port of the boardgame parser that reads and writes Game Definition Files (.gdf format).

## Overview

This is a complete Go implementation of a parser for strategy board game definitions (similar to Axis and Allies). It can load game state from .gdf files, manipulate the game state in memory, and save it back to human-readable .gdf format.

## Building

```bash
go build -o boardgame
```

## Usage

**Load and display game statistics:**
```bash
./boardgame <input.gdf>
```

**Load a game and save it to a new file:**
```bash
./boardgame <input.gdf> <output.gdf>
```

**Example:**
```bash
./boardgame ../aaa.gdf                    # Load and display stats
./boardgame ../aaa.gdf saved_game.gdf     # Load and save to new file
```

## Architecture

The Go implementation follows the same architecture as the C++ version:

### Packages

- **models** - Core game data structures (Game, Player, Territory, Piece)
- **scanner** - Lexical analyzer that tokenizes .gdf files
- **parser** - Recursive descent parser that builds game state

### Key Design Decisions

1. **Single-word tokenization**: The scanner returns single words as IDENTIFIER tokens, just like the C++ version
2. **Multi-word names in parser**: Territory names with spaces (e.g., "South West Indian Ocean") are assembled by the parser's `parseTerritoryName()` method
3. **Section-based parsing**: Section keywords (Players, Turn, Territories, Map, Units, Containers, Placement) act as delimiters between sections
4. **Terrain types**: Supports land, water, air, and both terrain types

## Game Definition File Format

The .gdf format consists of several sections:

```
Players
  name1, name2, name3;

Turn 1;

Territories
  Name :terrain, owner, production;
  ...

Map
  Territory: connected1, connected2;
  ...

Units
  name: terrain, X movement, Y attack, Z defend, W cost;
  ...

Containers
  transport: capacity, unit_type1, unit_type2;
  ...

Placement
  Territory: count unit_type, count unit_type;
  ...
```

## Statistics from aaa.gdf

The included `aaa.gdf` file contains:
- 6 players (Germany, USA, USSR, UK, Japan, Neutral)
- 127 territories (69 land, 58 water)
- 10 unit types (infantry, armor, fighter, bomber, etc.)
- 298 total pieces placed on the board

## Game Systems

The implementation includes game logic systems:

- **ChangeOwnership(territory, newOwner)** - Transfer territory ownership between players
- **MovePiece(pieceID, from, to)** - Move a piece between territories
- **GetTerritoriesByOwner(playerName)** - Get all territories owned by a player
- **GetPiecesInTerritory(territoryName)** - Get all pieces in a territory
- **CountPiecesByType()** - Count how many of each piece type exist

## Testing

Run unit tests:
```bash
go test ./models -v
```

Run integration tests (requires aaa.gdf):
```bash
go test -v
```

All tests verify:
- Game file loading (127 territories, 298 pieces, 6 players)
- Territory ownership transfer
- Piece placement and movement
- Territory connections
- Data integrity

## Differences from C++ Version

- Uses Go idioms (error handling, slices, maps)
- More explicit error messages with line numbers
- Territory connections use pointers instead of reference_wrapper
- Player order tracked separately for consistent output
- Single-pass parsing (parser builds game directly vs. C++ two-stage: GameState → Game)
