# Axis & Allies Web Interface

## Overview

The web interface provides an accessible, browser-based GUI for playing Axis & Allies 1942. It features:

- **Clickable territory navigation** with search and filtering
- **Phase-by-phase guidance** with modal prompts for each turn phase
- **Accessible keyboard navigation** for screen readers and keyboard-only users
- **Responsive design** that works on different screen sizes
- **Real-time game state updates** while NPC players take their turns

## Quick Start

### 1. Build and Run the Web Server

```bash
# From the go-lang directory
cd /home/ccd/bg/go-lang

# Build the web server
go build -o webserver cmd/webserver/main.go

# Run the server (defaults to port 8080)
./webserver

# Or specify a custom port
./webserver -port 3000
```

### 2. Access the Game

Open your web browser and navigate to:
```
http://localhost:8080
```

### 3. Start a New Game

1. Enter the path to your game definition file (e.g., `../aaa.gdf`)
2. Select which nation you want to play
3. Click "Start Game"

## User Interface Guide

### Main Screen Layout

The game screen is divided into several sections:

#### Header (Top)
- **Turn Number**: Current game turn
- **Current Phase**: Which phase of the turn (Purchase, Combat Move, etc.)
- **Current Player**: Whose turn it is
- **Victory Cities**: Axis and Allies victory city counts

#### Left Sidebar: Territory List
- Searchable list of all territories
- Shows owner, unit count, and victory city status
- Click any territory to view details
- Use arrow keys to navigate

#### Center: Map Area
- Placeholder for the game map (to be enhanced with actual map image)
- Visual representation of the game board

#### Right Sidebar: Territory Details
- Shows detailed information about the selected territory
- Lists all units with their stats
- Shows connected territories
- Provides action buttons for units during your turn

#### Bottom Action Bar
- Phase-specific action buttons
- "Done" buttons to advance to next phase
- Status indicators (moves planned, battles pending, etc.)

## Gameplay Flow

### Turn Phases

Each player's turn consists of 6 phases:

#### 1. Purchase Units
- Click "Buy Units" to see available units and costs
- Purchase units using your IPCs
- Units will be placed later in the Mobilize phase
- Click "Done Purchasing" to proceed

#### 2. Combat Move
- Select territories with your units
- Click "Move" on units to see reachable destinations
- Plan moves by selecting destinations
- Planned moves are shown in the action bar
- Click "Execute Moves" to finalize and create battles

#### 3. Conduct Combat
- Resolve battles in attacked territories
- Click "Auto-Resolve All Battles" for automatic resolution
- Or resolve battles individually
- Cannot proceed until all battles are resolved

#### 4. Noncombat Move
- Reposition units that didn't attack
- Move to friendly territories only
- Land aircraft from battles
- Click "Execute Moves" when ready

#### 5. Mobilize New Units
- Place purchased units at industrial complexes
- Select territory with IC
- Place units (limited by production capacity)
- Click "Done Placing" when finished

#### 6. Collect Income
- Automatically collect IPCs from your territories
- Click "Collect Income & End Turn" to finish
- Game advances to next player

### Playing Against NPCs

When it's an NPC's turn:
1. The action bar will show "Watch NPC Turn"
2. Click the button to execute the NPC's full turn
3. The game will show a summary of what the NPC did
4. Game state automatically updates

## Accessibility Features

The web interface is designed to be fully accessible:

### Keyboard Navigation
- **Tab**: Navigate between interactive elements
- **Arrow Keys**: Navigate territory list
- **Enter/Space**: Activate buttons and select territories
- **Escape**: Close modals

### Screen Reader Support
- All interactive elements have proper ARIA labels
- Territory list is marked as navigation landmark
- Action sidebar is marked as complementary landmark
- Map is marked as region landmark
- Modals are announced when opened

### Visual Accessibility
- High contrast color scheme
- Clear focus indicators
- Large click targets
- Readable font sizes
- Color is not the only indicator of state

## API Integration

The frontend communicates with the backend via REST API. See `WEB_API_DESIGN.md` for full API documentation.

### Key API Flows

**Starting a Game:**
```
POST /api/game/new
→ Returns sessionId and initial game state
```

**Making a Move:**
```
1. POST /api/game/:sessionId/action/get-reachable
   → Get territories unit can reach

2. POST /api/game/:sessionId/action/plan-move
   → Plan the move

3. POST /api/game/:sessionId/action/advance-phase
   → Execute all planned moves
```

**Purchasing Units:**
```
1. POST /api/game/:sessionId/action/purchase
   → Buy units

2. [Later in Mobilize phase]
   POST /api/game/:sessionId/action/mobilize
   → Place units on board
```

## Files and Structure

### Backend
- `webserver/server.go` - Main HTTP server and routing
- `webserver/session.go` - Session management
- `webserver/dto.go` - Data Transfer Objects
- `webserver/actions.go` - Action handlers
- `cmd/webserver/main.go` - Entry point

### Frontend
- `webserver/static/index.html` - Main HTML template
- `webserver/static/css/style.css` - Stylesheet
- `webserver/static/js/api.js` - API client wrapper
- `webserver/static/js/app.js` - Vue.js application

## Customization

### Changing Port
```bash
./webserver -port 3000
```

### Modifying Colors
Edit `webserver/static/css/style.css`:
- `.owner-germany`, `.owner-japan`, etc. for nation colors
- `.axis-vc`, `.allies-vc` for victory city colors
- `.game-header` for header color

### Adding Custom Features
The Vue.js app is modular:
- Add new computed properties for derived state
- Add new methods for actions
- Modify the template in `index.html`

## Future Enhancements

### Planned Features
1. **Interactive Map**
   - SVG-based clickable map with hires_aaa_map.jpg as base
   - Visual territory highlighting
   - Click territories on map to select

2. **Enhanced Unit Movement**
   - Drag-and-drop unit movement
   - Visual path planning
   - Multi-step move visualization

3. **Battle Visualization**
   - Animated dice rolls
   - Step-by-step battle resolution
   - Casualty selection interface

4. **Game Persistence**
   - Save game state to database
   - Resume games later
   - Game history and replay

5. **Multiplayer Support**
   - WebSocket for real-time updates
   - Multiple human players
   - Spectator mode

## Troubleshooting

### Server Won't Start
```bash
# Check if port is already in use
lsof -i :8080

# Try a different port
./webserver -port 8081
```

### Can't Connect to Server
- Ensure the server is running
- Check firewall settings
- Verify the URL includes the correct port

### Game State Not Updating
- Check browser console for errors (F12)
- Verify API calls are succeeding
- Check server logs for errors

### Units Won't Move
- Ensure it's your turn (header shows "YOUR TURN")
- Verify you're in Combat Move or Noncombat Move phase
- Check unit hasn't already moved (canMove: false)

## Development

### Running Tests
```bash
# Backend tests
go test ./webserver -v

# Full game tests
go test ./... -v
```

### Building for Production
```bash
# Build optimized binary
go build -ldflags="-s -w" -o webserver_prod cmd/webserver/main.go
```

### Adding New API Endpoints
1. Define DTO in `webserver/dto.go`
2. Add handler in `webserver/actions.go`
3. Add route in `webserver/server.go`
4. Add client method in `webserver/static/js/api.js`
5. Add UI handler in `webserver/static/js/app.js`

## Support

For issues or questions:
- Check the main `README.md` for game rules
- Review `WEB_API_DESIGN.md` for API details
- Check browser console for errors
- Review server logs for backend issues

## Credits

- Game Engine: Complete Go implementation with NPC AI
- UI Framework: Vue.js 3
- HTTP Server: Go standard library
- Design: Custom accessible interface

---

**Note**: This web interface is designed to work alongside the existing terminal UI (`main.go`). Both interfaces use the same game engine and logic, ensuring consistent behavior.
