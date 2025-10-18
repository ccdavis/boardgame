# Quick Start: Axis & Allies Web Interface

This guide will get you up and running with the web interface in under 5 minutes.

## Prerequisites

- Go 1.21 or higher
- Modern web browser (Chrome, Firefox, Safari, or Edge)
- The `aaa.gdf` game definition file

## Step-by-Step Setup

### 1. Build the Web Server

```bash
cd /home/ccd/bg/go-lang
go build -o webserver cmd/webserver/main.go
```

Expected output:
```
(builds silently, creates 'webserver' binary)
```

### 2. Start the Server

```bash
./webserver
```

Expected output:
```
Starting Axis & Allies Web Server on port 8080
Access the game at: http://localhost:8080
```

### 3. Open Your Browser

Navigate to:
```
http://localhost:8080
```

You should see the Axis & Allies setup screen with a purple gradient background.

### 4. Configure Your Game

1. **Game File Path**: Enter `../aaa.gdf` (or the path to your game file)
2. **Select Your Nation**: Choose which country you want to play (e.g., Germany)
3. Click **"Start Game"**

### 5. Play!

You'll now see the main game interface with:

- **Header** showing turn, phase, and current player
- **Left sidebar** with searchable territory list
- **Center area** for the map (placeholder for now)
- **Right sidebar** showing territory details
- **Bottom action bar** with phase-specific buttons

## Your First Turn (Germany)

### Purchase Phase

1. You'll see a modal explaining the Purchase Phase - click "Got it!"
2. Click **"Buy Units"** in the action bar
3. Purchase some units (e.g., 5 infantry = 15 IPCs)
4. Click **"Done Purchasing"** to advance

### Combat Move Phase

1. Read the phase guidance modal
2. Select "Berlin" from the territory list
3. Click **"Move"** on a unit in the right sidebar
4. Select a destination territory
5. Click **"Execute Moves"** when ready

### Conduct Combat Phase

1. If you created battles, click **"Auto-Resolve All Battles"**
2. View the battle results
3. Click **"Done with Combat"**

### Noncombat Move Phase

1. Reposition any units that didn't attack
2. Click **"Execute Moves"**

### Mobilize Phase

1. Select a territory with an industrial complex (e.g., Berlin)
2. Place your purchased units
3. Click **"Done Placing"**

### Collect Income Phase

1. Click **"Collect Income & End Turn"**
2. You've completed your first turn!

### NPC Turns

When it's an NPC's turn:
1. The action bar shows "Watch NPC Turn"
2. Click the button to execute the NPC's turn
3. The game will show what the NPC did
4. Continue to your next turn

## Keyboard Navigation

For accessibility or preference:

- **Tab**: Navigate between elements
- **Arrow Keys**: Navigate territory list
- **Enter/Space**: Activate buttons
- **Escape**: Close modals
- **Search box**: Type to filter territories

## Common Actions

### Viewing Territory Details
- Click any territory in the left sidebar
- Details appear in the right sidebar
- Shows owner, units, production, connections

### Searching Territories
- Use the search box at the top of the territory list
- Type territory name or owner
- List filters in real-time

### Making Purchases
- Only during Purchase Phase
- Click "Buy Units" button
- Select units to purchase
- IPCs automatically deducted

### Planning Moves
- During Combat Move or Noncombat Move phases
- Select territory with your units
- Click "Move" on a unit
- Choose destination
- Repeat for all units
- Click "Execute Moves" when done

### Resolving Battles
- During Conduct Combat Phase
- Click "Auto-Resolve All Battles" (recommended)
- Or resolve individually by clicking territories

## Troubleshooting

### Server Won't Start

**Problem**: `address already in use`
```bash
# Try a different port
./webserver -port 8081
# Then visit http://localhost:8081
```

**Problem**: `permission denied`
```bash
chmod +x webserver
./webserver
```

### Can't Load Game File

**Problem**: `Failed to load game file: open ../aaa.gdf: no such file or directory`

**Solution**: Make sure the game file path is correct relative to where you run the server.
```bash
# If aaa.gdf is in parent directory:
../aaa.gdf

# If it's in the same directory:
./aaa.gdf

# Or use absolute path:
/home/ccd/bg/aaa.gdf
```

### Page Won't Load

1. **Check server is running**: Should see output in terminal
2. **Check URL**: Should be `http://localhost:8080`
3. **Check browser console**: Press F12, look for errors
4. **Try different browser**: Chrome, Firefox, etc.

### Game State Not Updating

1. **Check browser console** (F12) for errors
2. **Refresh the page** (but you'll lose current game)
3. **Restart server** and start new game

## Testing the Interface

### Test Scenario 1: Basic Turn Flow

1. Start game as Germany
2. Purchase 3 infantry
3. Plan a move from Berlin to Poland
4. Execute moves
5. Resolve any battles
6. Skip noncombat moves
7. Place units in Berlin
8. Collect income
9. ✓ You should advance to USSR's turn

### Test Scenario 2: NPC Turn

1. Start game as Germany
2. Complete your turn (quickly skip all phases)
3. When USSR's turn starts, click "Watch NPC Turn"
4. ✓ NPC should complete its turn automatically
5. ✓ Game should advance back to Germany

### Test Scenario 3: Territory Navigation

1. Start game
2. Click different territories in the list
3. ✓ Details should update in right sidebar
4. Try searching for "Berlin"
5. ✓ List should filter to show only Berlin
6. Click on connected territories
7. ✓ Selection should jump to that territory

## API Testing (Advanced)

You can test the API directly with curl:

```bash
# Create a new game
curl -X POST http://localhost:8080/api/game/new \
  -H "Content-Type: application/json" \
  -d '{"gdfPath":"../aaa.gdf","playerName":"Germany"}'

# Get game state (replace SESSION_ID with actual ID from above)
curl http://localhost:8080/api/game/SESSION_ID

# Get territories
curl http://localhost:8080/api/game/SESSION_ID/territories
```

## Performance Tips

### For Large Games
- Territory list is searchable - use it!
- Auto-resolve battles for speed
- NPC turns execute quickly

### For Multiple Sessions
- Each game is a separate session
- Server handles multiple games concurrently
- Sessions auto-cleanup after 24 hours

## Next Steps

Once you're comfortable with the basics:

1. **Read the full guide**: `WEB_INTERFACE_README.md`
2. **Explore the API**: `WEB_API_DESIGN.md`
3. **Review the code**: Start with `webserver/server.go`
4. **Add features**: See `WEB_INTERFACE_SUMMARY.md` for ideas

## Getting Help

- **Game Rules**: See `README.md` in the project root
- **API Details**: See `WEB_API_DESIGN.md`
- **User Guide**: See `WEB_INTERFACE_README.md`
- **Implementation**: See `WEB_INTERFACE_SUMMARY.md`

## Common Questions

**Q: Can I play with multiple human players?**
A: Not yet - currently supports one human player vs. NPCs. Multiplayer is a planned feature.

**Q: Can I save my game?**
A: Not yet - games are session-based and don't persist. Game persistence is planned.

**Q: Where's the map?**
A: The interactive map is a future enhancement. For now, use the territory list and details sidebar.

**Q: Can I run this on a remote server?**
A: Yes! Just make sure the port is accessible and use the server's IP instead of localhost.

**Q: Is this the same game as the terminal UI?**
A: Yes! Both interfaces use the exact same game engine and logic.

## Stopping the Server

To stop the server:
1. Go to the terminal where it's running
2. Press `Ctrl+C`
3. Server will shut down gracefully

## Cleaning Up

To remove the binary:
```bash
rm webserver
```

To clean all build artifacts:
```bash
go clean
```

---

**Enjoy playing Axis & Allies with the new web interface!**

For issues or questions, refer to the full documentation or check the code comments.
