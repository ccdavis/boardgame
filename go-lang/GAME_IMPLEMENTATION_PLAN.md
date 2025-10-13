# Axis & Allies 1942 - Implementation Plan

## Overview
Implementing a playable text-based version of Axis & Allies 1942 with human player vs NPC AI.

## Phase 1: Core Architecture & Data Models ✓ (Already Complete)

We already have:
- Game, Player, Territory, Piece models
- Board representation with connections
- Piece templates and placement
- Basic systems (ChangeOwnership, MovePiece)

**Need to Add:**
- Turn/Phase state management
- Player treasury tracking (already have IPCs)
- Victory city tracking
- Industrial complex damage tracking

## Phase 2: Turn Sequence Engine

### Game Controller (`game/controller.go`)
```
- TurnManager: Manages 6-phase turn sequence
  1. PurchaseUnitsPhase
  2. CombatMovePhase
  3. ConductCombatPhase
  4. NoncombatMovePhase
  5. MobilizeUnitsPhase
  6. CollectIncomePhase
- Victory condition checking
- Turn order: USSR → Germany → UK → Japan → USA
```

### Key Structures Needed:
```go
type GameController struct {
    Game *models.Game
    CurrentPower string
    CurrentPhase Phase
    PurchasedUnits map[string][]*PendingUnit
    CombatMoves []*CombatMove
    VictoryCityCount map[string]int
}

type Phase int
const (
    PurchasePhase Phase = iota
    CombatMovePhase
    ConductCombatPhase
    NoncombatMovePhase
    MobilizePhase
    CollectIncomePhase
)
```

## Phase 3: Movement System

### Movement Validator (`game/movement.go`)
**Must validate:**
- Unit movement ranges (infantry=1, tank=2, fighter=4, bomber=6, etc.)
- Terrain restrictions (land units on land, sea units in sea, air anywhere)
- Hostile vs friendly territory rules
- Special movements:
  - Tank blitzing (move through unoccupied hostile territory)
  - Air units saving movement for return landing
  - Submarine stealth movement
  - Amphibious assault setup

**Key Functions:**
```go
func ValidateMove(piece *Piece, from, to *Territory, phase Phase) error
func CanBlitz(tank *Piece, territory *Territory) bool
func FindLandingZones(airUnit *Piece, maxRange int) []*Territory
func IsAmph ibiousAssaultValid(transport *Transport, units []*Piece, target *Territory) bool
```

## Phase 4: Combat Resolution System

### Combat Engine (`game/combat.go`)
**Core Combat Flow:**
1. Place units on battle strip (attacker/defender)
2. Submarine surprise strike (if applicable)
3. Attackers roll dice (≤ attack value = hit)
4. Defenders roll dice (≤ defense value = hit)
5. Assign casualties
6. Repeat or retreat

**Special Combat Types:**
- **Strategic Bombing**: Bomber attacks industrial complex
  - IC rolls defense (1 die per bomber, hit on 1)
  - Surviving bombers roll 1 die for damage
  - Max damage = 2× territory IPC value

- **Amphibious Assault**:
  - Sea combat first (if hostile sea zone)
  - Battleship/Cruiser bombardment (if no sea combat)
  - Land combat

- **AAA Fire**: Before combat, AAA rolls against attacking air
  - Up to 3 shots per AAA, or 1 per air unit (whichever less)
  - Hit on roll of 1

**Special Unit Rules:**
- Infantry: Attack 2 when supported by artillery (1:1 ratio)
- Battleship: 2 hits to destroy
- Submarine: Surprise strike, can submerge, ignored by non-destroyers
- Destroyer: Cancels submarine special abilities
- Air vs submarines: Can't hit subs without friendly destroyer

**Key Structures:**
```go
type Battle struct {
    Location string
    Attackers []*Piece
    Defenders []*Piece
    Type BattleType  // Land, Sea, Strategic Bombing, Amphibious
}

func ResolveCombat(battle *Battle) *BattleResult
func RollDice(units []*Piece, isAttacking bool) []Hit
func AssignCasualties(hits int, units []*Piece) []*Piece
```

## Phase 5: Purchase & Production

### Purchase System (`game/purchase.go`)
```go
func PurchaseUnit(player *Player, unitType string, quantity int) error
func CanAfford(player *Player, cost int) bool
func MobilizeUnits(player *Player, ic *Territory, units []*Piece) error
```

**Production Rules:**
- Can only mobilize at IC you controlled at start of turn
- Max units = territory IPC value
- Damage reduces production capacity (1 damage = -1 unit)
- Repair costs 1 IPC per damage point

## Phase 6: Text User Interface

### Command-Line Interface (`ui/terminal.go`)
```
Available Commands by Phase:

[Purchase Phase]
- buy <unit> <quantity>          # Buy units
- repair <territory> <amount>    # Repair IC damage
- done                           # Proceed to next phase

[Combat Move Phase]
- move <unit-id> <from> <to>     # Move unit
- attack <territory>             # View planned attack
- cancel <unit-id>               # Cancel unit's move
- done                           # Lock in moves, proceed

[Combat Phase]
- view <territory>               # Show battle details
- roll                           # Roll attack/defense dice
- casualty <unit-type>           # Select casualty
- retreat [territory]            # Retreat from combat
- continue                       # Next round of combat

[Noncombat Move Phase]
- move <unit-id> <from> <to>     # Move non-combatants
- done                           # Proceed

[Mobilize Phase]
- place <unit> <territory> <qty> # Place purchased units
- done                           # Proceed

[Collect Income Phase]
- (automatic)                    # Shows income collected

[General Commands]
- status                         # Show game state
- board [territory]              # Show board/territory
- units <territory>              # List units in territory
- income                         # Show all players' income
- help                           # Show commands
- save <filename>                # Save game
- quit                           # Exit
```

### Display Functions:
```go
func DisplayBoard(game *Game)
func DisplayTerritory(territory *Territory)
func DisplayBattle(battle *Battle)
func DisplayPlayerStatus(player *Player)
func DisplayPhase(phase Phase)
```

## Phase 7: NPC AI System

### AI Strategy Levels

**Basic AI (Priority 1 - Implement First):**
```go
type BasicAI struct {
    Player *Player
    Game *Game
}

func (ai *BasicAI) PurchasePhase() {
    // Simple: Spend 50% on land, 30% on air, 20% on sea
    // Build where most threatened
}

func (ai *BasicAI) CombatMovePhase() {
    // Attack weak adjacent territories
    // Criteria: AttackStrength > DefenseStrength * 1.5
}

func (ai *BasicAI) SelectCasualties(units []*Piece, hits int) []*Piece {
    // Prefer low-cost units first
    // Keep high-value units (tanks, fighters)
}
```

**Intermediate AI (Priority 2 - Later):**
- Territory value assessment (income + strategic position)
- Force concentration (stack units before attacking)
- Defensive positioning (protect capitals, high-value territories)
- Economic targeting (strategic bombing of enemy ICs)

**Advanced AI (Priority 3 - Future):**
- Multi-turn planning
- Coordinated attacks across territories
- Naval superiority strategies
- Air superiority control
- Amphibious assault planning

### AI Decision Framework:
```go
type TerritoryThreat struct {
    Territory *Territory
    AttackPower int
    DefensePower int
    ThreatLevel float64  // ratio of enemy nearby / friendly nearby
}

func (ai *BasicAI) EvaluateThreats() []*TerritoryThreat
func (ai *BasicAI) PrioritizeTargets() []*Territory
func (ai *BasicAI) AllocateForces(target *Territory) []*Piece
```

## Phase 8: Special Rules Implementation

### Priority Order:
1. **Basic Combat** (highest priority)
   - Standard land combat
   - Naval combat
   - Air combat

2. **Artillery Support** (high priority)
   - Infantry attack bonus with artillery

3. **Tank Blitzing** (medium-high)
   - Move through empty hostile territory

4. **Strategic Bombing** (medium)
   - Damage industrial complexes

5. **Amphibious Assault** (medium)
   - Complex but important for island territories

6. **Submarine Special Rules** (medium-low)
   - Surprise strike
   - Submerge
   - Stealth movement

7. **Destroyer Anti-Sub** (medium-low)
   - Cancels submarine abilities

8. **Carrier Operations** (low - fewer carriers in game)
   - Fighter landing/launching

9. **Multi-national Forces** (low - single player vs AI)

## Implementation Order (Recommended)

### Sprint 1: Foundation (Days 1-2)
- [ ] Extend models with combat stats
- [ ] Turn/Phase management system
- [ ] Basic UI framework (display board, territories)
- [ ] Command parser

### Sprint 2: Movement (Days 3-4)
- [ ] Movement validation
- [ ] Combat move vs noncombat move
- [ ] Basic move commands in UI

### Sprint 3: Combat Core (Days 5-7)
- [ ] Dice rolling system
- [ ] Basic land combat resolution
- [ ] Casualty selection
- [ ] Combat UI (show battles, roll dice)

### Sprint 4: Economy (Days 8-9)
- [ ] Purchase system
- [ ] Industrial complex production
- [ ] Mobilization rules
- [ ] Income collection

### Sprint 5: Basic AI (Days 10-12)
- [ ] AI purchase logic
- [ ] AI attack selection (simple)
- [ ] AI casualty selection
- [ ] Full AI turn execution

### Sprint 6: Polish & Special Rules (Days 13-15)
- [ ] Artillery support
- [ ] Tank blitzing
- [ ] Strategic bombing
- [ ] AAA fire
- [ ] Victory conditions
- [ ] Save/load game state

### Sprint 7: Advanced Features (Days 16+)
- [ ] Amphibious assaults
- [ ] Submarine rules
- [ ] Carrier operations
- [ ] Improved AI
- [ ] Better UI/UX

## Testing Strategy

1. **Unit Tests**: Test individual components
   - Movement validation
   - Combat resolution
   - Purchase validation

2. **Integration Tests**: Test full turns
   - Complete AI turn
   - Combat sequences
   - Victory condition checking

3. **Playtest**: Human vs AI games
   - Ensure AI makes reasonable decisions
   - Game balance
   - No infinite loops or deadlocks

## Success Criteria

**Minimum Viable Product:**
- ✅ Load aaa.gdf
- [ ] Human player can take full turn (all 6 phases)
- [ ] AI can take full turn automatically
- [ ] Basic combat works (land combat minimum)
- [ ] Purchase and production works
- [ ] Game ends when victory condition met
- [ ] Can save/load game state

**Full Implementation:**
- [ ] All unit types functional
- [ ] All special rules implemented
- [ ] Competent AI opponent
- [ ] Good UX (clear prompts, error messages)
- [ ] Complete game from start to finish

## Technical Debt to Address

From existing code:
- PieceTemplates stored in both Player and Game (consolidate?)
- Need better piece ID management
- Territory connections should be bidirectional?
- Add industrial complex as a proper component

## Notes

- Start simple: Get basic gameplay loop working first
- Defer complex rules (amphibious assault, submarines) until core works
- AI doesn't need to be perfect, just playable
- Text UI should be clear and instructive
- Focus on turn-based gameplay, not real-time
