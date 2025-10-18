# Web API Design for Axis & Allies

## Overview

This document describes the REST API architecture for the web-based GUI interface to the Axis & Allies game.

## Architecture Principles

1. **Stateful Sessions**: The server maintains game state in memory with session IDs
2. **RESTful Design**: Standard HTTP methods (GET, POST, DELETE)
3. **JSON Communication**: All request/response bodies use JSON
4. **Phase-Driven Actions**: Available actions depend on current game phase
5. **Human Player Only**: API is designed for human interaction (NPC turns execute server-side)

## Session Management

### Session Structure
```go
type GameSession struct {
    ID              string           // UUID
    Controller      *game.GameController
    HumanPlayer     string           // Name of human player
    CreatedAt       time.Time
    LastAccessedAt  time.Time
}
```

### Session Lifecycle
- Created when player starts new game
- Stored in memory map (can extend to Redis/database later)
- Expires after 24 hours of inactivity
- Destroyed when player explicitly ends game

## API Endpoints

### Game Management

#### Create New Game
```
POST /api/game/new
Content-Type: application/json

Request:
{
  "gdfPath": "/path/to/game.gdf",  // Path to game definition file
  "playerName": "Germany"           // Which nation the human plays
}

Response (201):
{
  "sessionId": "uuid-here",
  "game": {
    "turn": 1,
    "currentPower": "Germany",
    "currentPhase": "Purchase Units",
    "playerOrder": ["Germany", "USSR", "UK", "Japan", "USA", "Italy"]
  }
}
```

#### Get Game State
```
GET /api/game/:sessionId

Response (200):
{
  "turn": 1,
  "currentPower": "Germany",
  "currentPhase": "Purchase Units",
  "humanPlayer": "Germany",
  "players": [
    {
      "name": "Germany",
      "ipcs": 40,
      "side": "Axis",
      "isHuman": true,
      "territoryCount": 12
    },
    ...
  ],
  "victoryCities": {
    "axis": 4,
    "allies": 8
  }
}
```

#### Delete Game Session
```
DELETE /api/game/:sessionId

Response (204): No Content
```

### Territory Information

#### Get All Territories
```
GET /api/game/:sessionId/territories

Response (200):
{
  "territories": [
    {
      "name": "Berlin",
      "owner": "Germany",
      "terrain": "land",
      "production": 10,
      "isVictoryCity": true,
      "unitCount": 5,
      "icDamage": 0,
      "neutralType": "not_neutral",
      "connectedTo": ["Poland", "East Prussia", ...]
    },
    ...
  ]
}
```

#### Get Territory Details
```
GET /api/game/:sessionId/territory/:territoryName

Response (200):
{
  "name": "Berlin",
  "owner": "Germany",
  "terrain": "land",
  "production": 10,
  "isVictoryCity": true,
  "icDamage": 0,
  "neutralType": "not_neutral",
  "units": [
    {
      "id": 1,
      "name": "infantry",
      "attack": 1,
      "defend": 2,
      "movement": 1,
      "canMove": true  // Based on current phase and prior moves
    },
    ...
  ],
  "connectedTerritories": [
    {
      "name": "Poland",
      "owner": "Germany",
      "unitCount": 2,
      "canAttack": false,
      "canMoveTo": true
    },
    ...
  ]
}
```

### Available Actions

#### Get Available Actions for Current Phase
```
GET /api/game/:sessionId/available-actions

Response (200) - Purchase Phase:
{
  "phase": "Purchase Units",
  "isHumanTurn": true,
  "actions": {
    "purchase": {
      "availableUnits": [
        {"type": "infantry", "cost": 3, "available": true},
        {"type": "armor", "cost": 5, "available": true},
        ...
      ],
      "currentIPCs": 40
    },
    "repair": {
      "damagedICs": [
        {"territory": "Berlin", "damage": 3, "repairCost": 3}
      ]
    }
  }
}

Response (200) - Combat Move Phase:
{
  "phase": "Combat Move",
  "isHumanTurn": true,
  "actions": {
    "planMove": {
      "movableUnits": [
        {
          "pieceId": 1,
          "type": "infantry",
          "currentTerritory": "Berlin",
          "movement": 1,
          "reachableTerritories": ["Poland", "East Prussia"]
        },
        ...
      ]
    },
    "plannedMoves": [
      {"pieceId": 1, "from": "Berlin", "to": "Poland"}
    ],
    "plannedAttacks": ["Poland", "Ukraine"]
  }
}
```

### Player Actions

#### Purchase Units
```
POST /api/game/:sessionId/action/purchase
Content-Type: application/json

Request:
{
  "unitType": "infantry",
  "quantity": 5
}

Response (200):
{
  "success": true,
  "remainingIPCs": 25,
  "purchasedUnits": [
    {"type": "infantry", "quantity": 5}
  ]
}

Error (400):
{
  "error": "Insufficient IPCs: need 15, have 10"
}
```

#### Plan a Move
```
POST /api/game/:sessionId/action/plan-move
Content-Type: application/json

Request:
{
  "pieceId": 1,
  "from": "Berlin",
  "to": "Poland"
}

Response (200):
{
  "success": true,
  "move": {
    "pieceId": 1,
    "from": "Berlin",
    "to": "Poland",
    "type": "combat"  // or "noncombat"
  },
  "willCreateBattle": true
}
```

#### Cancel a Move
```
POST /api/game/:sessionId/action/cancel-move
Content-Type: application/json

Request:
{
  "pieceId": 1
}

Response (200):
{
  "success": true
}
```

#### Advance Phase
```
POST /api/game/:sessionId/action/advance-phase

Response (200):
{
  "success": true,
  "previousPhase": "Purchase Units",
  "currentPhase": "Combat Move",
  "summary": "Purchased 5 units for 15 IPCs",
  "warnings": []
}

Response (200) - with warnings:
{
  "success": true,
  "previousPhase": "Combat Move",
  "currentPhase": "Conduct Combat",
  "summary": "Executing 12 combat moves",
  "warnings": [],
  "battles": ["Poland", "Ukraine", "Leningrad"]
}

Error (400):
{
  "error": "Cannot advance: you still have 3 unresolved battles"
}
```

#### Mobilize Units
```
POST /api/game/:sessionId/action/mobilize
Content-Type: application/json

Request:
{
  "unitType": "infantry",
  "territory": "Berlin",
  "quantity": 3
}

Response (200):
{
  "success": true,
  "remainingUnits": [
    {"type": "infantry", "quantity": 2}
  ]
}
```

#### Resolve Battle
```
POST /api/game/:sessionId/action/resolve-battle
Content-Type: application/json

Request:
{
  "territory": "Poland"
}

Response (200):
{
  "success": true,
  "result": {
    "territory": "Poland",
    "attackerWins": true,
    "defenderWins": false,
    "rounds": 2,
    "attackerCasualties": ["infantry", "infantry"],
    "defenderCasualties": ["infantry", "infantry", "armor"],
    "territoryCaptured": true
  }
}
```

#### Auto-Resolve All Battles
```
POST /api/game/:sessionId/action/auto-resolve-battles

Response (200):
{
  "success": true,
  "battles": [
    {
      "territory": "Poland",
      "attackerWins": true,
      "rounds": 2,
      "attackerCasualties": 2,
      "defenderCasualties": 3
    },
    ...
  ]
}
```

#### Get Reachable Territories for Piece
```
POST /api/game/:sessionId/action/get-reachable
Content-Type: application/json

Request:
{
  "pieceId": 1,
  "fromTerritory": "Berlin"
}

Response (200):
{
  "piece": {
    "id": 1,
    "type": "armor",
    "movement": 2
  },
  "reachable": [
    {
      "name": "Poland",
      "distance": 1,
      "owner": "USSR",
      "isAttack": true,
      "unitCount": 3
    },
    {
      "name": "East Prussia",
      "distance": 1,
      "owner": "Germany",
      "isAttack": false,
      "unitCount": 0
    }
  ]
}
```

## Data Transfer Objects (DTOs)

To avoid circular references and reduce payload size, we use simplified DTOs:

### TerritoryDTO
```go
type TerritoryDTO struct {
    Name          string   `json:"name"`
    Owner         string   `json:"owner"`
    Terrain       string   `json:"terrain"`
    Production    int      `json:"production"`
    IsVictoryCity bool     `json:"isVictoryCity"`
    ICDamage      int      `json:"icDamage"`
    NeutralType   string   `json:"neutralType"`
    UnitCount     int      `json:"unitCount"`
    ConnectedTo   []string `json:"connectedTo"`  // Just names, not full objects
}
```

### UnitDTO
```go
type UnitDTO struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    Attack   int    `json:"attack"`
    Defend   int    `json:"defend"`
    Movement int    `json:"movement"`
    Terrain  string `json:"terrain"`
    CanMove  bool   `json:"canMove"`  // Computed based on phase/prior moves
}
```

### PlayerDTO
```go
type PlayerDTO struct {
    Name           string `json:"name"`
    IPCs           int    `json:"ipcs"`
    Side           string `json:"side"`  // "Axis" or "Allies"
    IsHuman        bool   `json:"isHuman"`
    TerritoryCount int    `json:"territoryCount"`
    Capital        string `json:"capital"`
}
```

## Error Handling

All errors return appropriate HTTP status codes:
- `400 Bad Request`: Invalid input or action not allowed in current phase
- `404 Not Found`: Session ID or resource not found
- `500 Internal Server Error`: Unexpected server error

Error response format:
```json
{
  "error": "Human-readable error message"
}
```

## NPC Turn Handling

When it's an NPC's turn:
1. Client polls `GET /api/game/:sessionId/state` to detect NPC turn
2. Client sends `POST /api/game/:sessionId/action/execute-npc-turn`
3. Server executes full NPC turn and returns summary
4. Game advances to next player

```
POST /api/game/:sessionId/action/execute-npc-turn

Response (200):
{
  "success": true,
  "player": "Japan",
  "summary": "Japan purchased 5 infantry, attacked China, mobilized units",
  "newPhase": "Purchase Units",
  "newCurrentPower": "USA"
}
```

## WebSocket Alternative (Future Enhancement)

For real-time updates, can add WebSocket support:
- `WS /api/game/:sessionId/ws`
- Push game state changes to client
- Reduce polling
- Enable spectator mode

## Security Considerations

1. **Session Validation**: All endpoints validate session ID
2. **Player Verification**: Only human player can take actions (checked server-side)
3. **Rate Limiting**: Prevent API abuse
4. **Input Validation**: Sanitize all inputs
5. **CORS Configuration**: Configure allowed origins

## Implementation Plan

1. Create `webserver` package
2. Implement session manager
3. Implement DTO converters
4. Implement endpoints incrementally:
   - Game management first
   - Territory queries
   - Purchase phase actions
   - Movement actions
   - Combat actions
5. Add comprehensive tests for each endpoint
6. Document with OpenAPI/Swagger spec

## Testing Strategy

- Unit tests for DTO converters
- Integration tests for each endpoint
- End-to-end tests for complete turn flows
- Load tests for session management
