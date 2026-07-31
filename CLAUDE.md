# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a complete Axis & Allies strategy board game implementation in Go, featuring:
- Board derived from Axis & Allies Classic (Milton Bradley 1984/86), extended
  with Italy as a sixth power -- see "Board data" below
- Interactive Terminal User Interface (TUI) for human players
- Strategic NPC AI that can play complete games autonomously
- Six playing powers (Germany, USSR, UK, Japan, USA, Italy) plus a non-playing
  Neutral that owns unclaimed territory
- All combat mechanics: land, sea, air, amphibious assaults, strategic bombing
- Neutral territory rules with proper strict/pro-Allied/pro-Axis classifications
- Victory city tracking and win condition detection

## Build and Run

**Build the main game:**
```bash
cd go-lang
go build -o boardgame main.go
```

**Run interactive game with TUI:**
```bash
./boardgame ../aaa.gdf
```

**Build and run NPC-only demo:**
```bash
go build -o npc_game_demo cmd/npc_game_demo.go
./npc_game_demo
```

**Run tests:**
```bash
go test ./... -v                    # All tests
go test ./game -v                   # Game logic tests
go test ./models -v                 # Model tests
go test -run "TestNPC" ./game -v    # NPC AI tests
```

## Architecture

### Package Structure

- **models/** - Core game data structures
  - `Game`: Top-level container (board, players, pieces, turn state)
  - `Player`: Player state (IPCs, territories, units, side affiliation)
  - `Territory`: Board locations (owner, pieces, connections, victory city status, neutral type)
  - `Piece`: Game units (stats, cargo, hits taken)
  - `Phase`: Turn phases (Purchase, Combat Move, Conduct Combat, Noncombat Move, Mobilize, Collect Income)
  - `NeutralType`: Territory neutrality (NotNeutral, StrictNeutral, ProAlliedNeutral, ProAxisNeutral)

- **scanner/** - Lexical analyzer
  - Tokenizes .gdf files into IDENTIFIER, NUMBER, COLON, SEMICOLON, COMMA tokens

- **parser/** - Recursive descent parser
  - Parses .gdf files into game state
  - Handles multi-word territory names (e.g., "South West Indian Ocean")
  - Sections: Players, Turn, Territories, Map, Units, Containers, Placement

- **game/** - Game logic and systems
  - `controller.go`: Turn sequence management, phase advancement
  - `combat.go`: Battle resolution with dice rolling, casualties, retreat logic
  - `movement.go`: Movement validation, path finding, transport loading/unloading
  - `blitz.go`: Tank blitzing through unoccupied territories
  - `npc_ai.go`: NPC AI decision-making with strategic victory city focus
  - `game_runner.go`: Orchestrates complete games to victory
  - `transcript.go`: Records all game actions in human-readable format

- **ui/** - Terminal User Interface
  - `terminal.go`: TUI for human players (bubbletea framework)
  - `display.go`: Rendering game state, territory info, combat results
  - `ai.go`: NPC turn execution within UI context

## Key Game Mechanics

### Turn Structure (6 Phases)

Each player's turn follows this sequence:

1. **Purchase Units** - Spend IPCs to buy units (placed later in Mobilize phase)
2. **Combat Move** - Move units to attack enemy territories or activate friendly neutrals
3. **Conduct Combat** - Resolve all battles, including strategic bombing raids
4. **Noncombat Move** - Move units to friendly territories, land aircraft, activate pro-side neutrals
5. **Mobilize New Units** - Place purchased units at territories with industrial complexes
6. **Collect Income** - Gain IPCs based on controlled territory production values

### Combat System

Located in `game/combat.go`:
- Simultaneous dice rolling (attackers roll attack values, defenders roll defense values)
- Hits applied after each round
- Attacker chooses casualties first, then defender
- Multi-hit units (battleships) can take 2 hits
- Submarines: First strike, can submerge, ignored by aircraft
- Destroyers: Cancel submarine abilities
- Anti-aircraft artillery: One pre-combat roll per attacking aircraft
- Artillery support: Infantry attack at 2 when paired with artillery (the
  mechanic is implemented and tested, but `aaa.gdf` declares no artillery
  unit, so it is inert on the real board)
- Strategic bombing: Target industrial complexes to cause damage

### Movement Rules

Located in `game/movement.go`:
- Validates movement distance based on unit type
- Path finding for multi-space moves
- Blitzing: Tanks can move through friendly unoccupied territories
- Amphibious assault: Land units load on transports, move, then attack coastal territory
- Air units overfly enemy territory and units freely; what limits them is range
- Air unit landing: a noncombat air move must end in friendly territory or on a
  friendly carrier with room (checked per carrier slot at planning time; two
  aircraft planned onto the same last slot in one phase are not yet caught)

### Neutral Territory Rules

Located in `models/models.go` and `game/movement.go`:
- **Strict Neutrals** (Turkey, Afghanistan, Syria, Mongolia): Cannot be attacked. If any strict neutral is attacked, all become hostile with defending infantry.
- **Pro-Allied Neutrals** (South America, some Middle East): Allied powers can peacefully activate during noncombat move, gaining free infantry. Axis can attack.
- **Pro-Axis Neutrals**: Axis powers can peacefully activate during noncombat move. Allies can attack.
- **Water Territories**: Owned by "Neutral" but freely traversable (NotNeutral type)

### NPC AI Strategy

Located in `game/npc_ai.go`:
- **Victory City Focus**: Prioritizes capturing victory cities (+15 score bonus)
- **Defensive Awareness**: Evaluates territory threats, won't leave victory cities vulnerable
- **Strategic Positioning**: Moves units toward staging areas adjacent to enemy victory cities
- **Intelligent Purchases**: Balanced force composition (infantry, armor, fighters)
- **Three-tier Noncombat Priority**:
  1. Defend threatened territories (especially victory cities)
  2. Position for victory city attacks
  3. General border consolidation
- **Vulnerability Checks**: Won't move units if it leaves source territory exposed to counterattack

### Special Unit Abilities

- **Submarines** (`game/submarine_test.go`): First strike, can submerge, ignored by aircraft
- **Destroyers** (`game/combat.go`): Cancel submarine special abilities
- **Carriers** (`game/movement.go`): Carry up to 2 fighters; a fighter may only
  end a noncombat move on friendly ground or a friendly carrier with room
- **Transports** (`game/transport_test.go`): Carry infantry/artillery, enable amphibious assaults
- **Battleships** (`game/battleship_test.go`): 2 hits to destroy. Shore
  bombardment code exists (`RollBombardment`) but the amphibious landing flow
  does not yet call it
- **Artillery** (`game/artillery_test.go`): Boost infantry attack from 1 to 2
- **Anti-Aircraft Artillery** (`game/combat.go`): Pre-combat roll against aircraft
- **Bombers** (`game/bombing_test.go`): Strategic bombing raids on industrial complexes

## Board data

`aaa.gdf` describes an **Axis & Allies Classic** board (Milton Bradley 1984/86),
not the 1942 Second Edition. It is closest to TripleA's `world_war_ii_classic`:
both merge Syria with Jordan and neither has the later edition's territory
breakdown. It has since been extended -- Norway and Finland split apart, and
Sweden, Switzerland, Morocco, Byelorussia and Italy added -- so it is now a
variant rather than a faithful reproduction of any single printed board.

The board is the authority. Where the geometry and the graph disagree, the graph
wins and the map is adjusted to match it.

### Sections beyond the original format

- `Sides` -- which powers fight together. Membership makes a power *playable*:
  it sets `Player.Side` and `Player.TakesTurns`. `Neutral` is deliberately
  absent, so it owns territory without taking a turn.
- `Capitals` -- capturing one transfers the defender's treasury.
- `VictoryCities` -- always present in the data; `Game.VictoryCitiesEnabled`
  controls whether holding them ends the game.
- `Neutrality` -- `strict`, `proallied` or `proaxis` per territory.

Two constraints on anyone extending the grammar further: the scanner matches
reserved words case-insensitively, so `neutral` must never become a keyword (it
is an owner name throughout `Territories`); and section parsers terminate on any
keyword via `atSectionKeyword`, so sections may appear in any order.

## Map geometry

`aaa.gdf` pairs with `aaa.layout.json`, generated by `tools/mapgen/`. The two are
checked against each other when a game is created, and the server refuses to
start a game if the territory names disagree.

Each territory is one SVG `<path>` that is simultaneously fill, border and click
target, so the visual and the hit region cannot drift apart. That is the fix for
the original problem: the previous implementation kept hand-authored circle
coordinates in a separate file from the map image, and they were never in
agreement.

```bash
# Regenerate the geometry (only needed when the board changes)
uv run --with shapely --with topojson --with numpy --with scipy --with geopandas \
    python tools/mapgen/build.py

# Check geometry against the board graph
cd go-lang && go run ./cmd/layoutcheck ../aaa.gdf

# Verify in a real browser: every territory's anchor must select that territory
cd go-lang && RUN_BROWSER_TESTS=1 go test ./webserver -run TestMap_ -v
```

`links` in the layout records adjacencies with no shared border -- the map seam
where the board splits North America across both edges, and places where the
graph abstracts away real coastline. `layoutcheck` warnings list regions that
touch on the map but are not connected in the `.gdf`; those are a board-design
decision, not a rendering fault.

## Game Definition File Format (.gdf)

The .gdf format is a custom, human-readable format:

```
Players
  Germany, USSR, UK, Japan, USA, Italy;

Turn 1;

Territories
  Berlin :land, Germany, 10;
  Moscow :land, USSR, 8;
  ...

Map
  Berlin: Poland, East Prussia;
  Moscow: Leningrad, Ukraine;
  ...

Units
  infantry: land, 1 movement, 1 attack, 2 defend, 3 cost;
  armor: land, 2 movement, 3 attack, 3 defend, 5 cost;
  ...

Containers
  transport: 2, infantry, artillery;
  carrier: 2, fighter;
  ...

Placement
  Berlin: 3 infantry, 2 armor, 1 fighter;
  Moscow: 5 infantry, 1 armor;
  ...
```

## Testing Coverage

The codebase has comprehensive test coverage:

- **Unit tests**: All core systems (combat, movement, carriers, blitz, artillery, submarines, etc.)
- **Integration tests**: Full turn sequences, multi-player games
- **NPC tests**: AI decision-making, complete games to victory
- **Rule compliance tests**: Neutral territories, amphibious assaults, strategic bombing

Key test files:
- `game/combat_test.go` - Battle resolution
- `game/movement_test.go` - Movement validation
- `game/carrier_test.go` - Carrier operations
- `game/npc_game_test.go` - NPC AI behavior
- `game/neutral_test.go` - Neutral territory rules
- `game/full_turn_integration_test.go` - Complete turn sequences

## Key Features

- ✅ Axis & Allies Classic board, extended with Italy
- ✅ 6-player support with turn order
- ✅ All combat mechanics (land, sea, air, combined)
- ✅ Amphibious assaults with transport coordination
- ✅ Strategic bombing raids with industrial complex damage
- ✅ Carrier operations with fighter dependency tracking
- ✅ Submarine first strike and submerge abilities
- ✅ Neutral territory rules (strict, pro-Allied, pro-Axis)
- ✅ Victory city tracking and win conditions
- ✅ Industrial complex placement and damage
- ✅ Strategic NPC AI with victory city focus
- ✅ Game transcript generation
- ✅ Interactive TUI for human players
- ✅ Complete game runner for NPC-only games

## Documentation Files

The `go-lang/` directory contains detailed implementation documentation:
- `README.md` - Main project overview
- `NPC_SYSTEM_README.md` - NPC AI system documentation
- `NPC_STRATEGIC_IMPROVEMENTS.md` - Recent AI enhancements (victory city focus, defensive awareness)
- `NEUTRAL_TERRITORIES_IMPLEMENTATION.md` - Neutral territory rules details
- `MOVEMENT_FIX_SUMMARY.md` - Movement system improvements
- `FACTORY_FIX_SUMMARY.md` - Industrial complex mechanics

## Recent Major Improvements

1. **Strategic NPC AI** (commit 2641060): Victory city prioritization, defensive vulnerability checks, three-tier noncombat movement strategy
2. **Neutral Territory Rules** (see NEUTRAL_TERRITORIES_IMPLEMENTATION.md): Proper strict/pro-Allied/pro-Axis classifications
3. **Complete Rule Compliance**: Multiple iterations improving rule adherence to official rulebook
4. **Interactive TUI**: Full terminal interface for human gameplay

## Development Notes

- Uses Go 1.21 (`go.mod`)
- Module name: `boardgame`
- No external dependencies for core game logic
- UI uses bubbletea framework (terminal UI library)
- Extensive test coverage (run `go test ./... -v`)
- Parser provides detailed error messages with line numbers for .gdf files
