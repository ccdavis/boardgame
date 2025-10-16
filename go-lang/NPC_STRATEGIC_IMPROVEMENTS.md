# NPC Strategic Improvements - Victory City Focus

## Overview
Enhanced the NPC AI to play with strategic focus on capturing victory cities while maintaining defensive awareness to prevent territories from being left vulnerable to enemy counterattacks.

## Key Improvements

### 1. Victory City Prioritization (`game/npc_ai.go`)

#### Attack Target Selection
The `findAttackTargets()` function now uses a sophisticated scoring system:
- **Victory cities get +15 score bonus** (massive priority)
- Base score from territory production value
- Connectivity bonus (territories with more friendly neighbors are easier to hold)
- Penalty for heavily defended territories (still considered, just lower priority)

Example: A victory city with production 3 gets a score of 18 (3 + 15), while a regular territory with production 8 only gets 8.

#### Strategic Target Identification
New `identifyStrategicTargets()` function:
- Identifies all enemy victory cities
- Sorts them by production value
- Used to guide noncombat movement decisions

### 2. Defensive Vulnerability System (`game/npc_ai.go`)

#### Territory Threat Evaluation
New `evaluateTerritoryThreat()` function:
- Compares enemy attack power in adjacent territories vs friendly defense
- Returns threat score (0 = safe, higher = more threatened)
- Considers only mobile units that can actually attack

#### Vulnerability Checks
New `wouldLeaveTerritoryVulnerable()` function checks if moving units would create weakness:
- Ensures territories aren't left with 0-1 units if enemies are adjacent
- Calculates remaining defense power after unit removal
- Territory is vulnerable if defense < 60% of max adjacent enemy threat

#### Combat Move Protection
Enhanced `CombatMovePhase()`:
- Checks vulnerability before moving units for attack
- Reduces force commitment if source territory would be weakened
- **Victory cities kept with minimum 3 units** for defense
- Prevents leaving territories open to easy counterattack

### 3. Strategic Non-Combat Movement (`game/npc_ai.go`)

Complete overhaul of `NoncombatMovePhase()` with three-tier priority system:

#### Priority 1: Defend Threatened Territories
- Identifies territories under threat (threat score > 0)
- Prioritizes reinforcing:
  - Own victory cities
  - High production territories (3+)
  - Heavily threatened territories (threat > 5)
- Moves units from safe territories to threatened ones

#### Priority 2: Position for Victory City Attacks
- Identifies "staging territories" (friendly territories adjacent to enemy victory cities)
- Moves units from safe rear areas to staging positions
- Enables future attacks on strategic objectives
- Creates forward momentum toward victory conditions

#### Priority 3: General Border Consolidation
- Only if units haven't been moved by priorities 1-2
- Consolidates forces at border territories
- Provides general defensive posture

**All priorities respect vulnerability checks** - units won't be moved if it leaves the source territory vulnerable.

### 4. Intelligent Attack Decision Making (`game/npc_ai.go`)

Enhanced attack evaluation in `CombatMovePhase()`:
- Territory value heavily weighted by victory city status
- Success probability threshold adjusted for high-value targets:
  - Regular territory: 60% success needed
  - Victory city or high production: 45% success needed (worth the risk!)
- Desperation factor when losing (< 5 territories owned)

### 5. Victory City Defense (`game/npc_ai.go`)

Special protection for owned victory cities:
- Combat moves keep minimum 3 units in victory cities
- Noncombat moves prioritize reinforcing threatened victory cities
- Prevents giving away victory conditions through careless moves

## Strategic Impact

### Before Improvements
- NPCs attacked randomly adjacent territories
- No consideration of victory cities
- Often left territories vulnerable to counterattack
- Units positioned randomly during noncombat phase
- No focus on winning condition

### After Improvements
- NPCs actively pursue victory cities
- Maintain strong defense of owned victory cities
- Position units strategically to enable future attacks
- Never leave territories dangerously exposed
- Play toward actual win condition (capturing 9/10/13 victory cities)

## Example Scenarios

### Scenario 1: Germany attacking Leningrad (Victory City)
**Before:** Leningrad scored same as any production 3 territory
**After:** Leningrad scores 18 (3 + 15 victory city bonus), becomes top priority target

### Scenario 2: Moving units from Moscow for attack
**Before:** Would move 50% of units, potentially leaving Moscow weakly defended
**After:** Checks vulnerability, sees adjacent German forces, reduces to 25% or skips move if Moscow would become vulnerable

### Scenario 3: Non-combat movement from Siberia
**Before:** Moved randomly to nearby border territories
**After:** Identifies nearest enemy victory city (e.g., Tokyo), moves units toward staging territory adjacent to Tokyo for future attack

## Files Modified

- `game/npc_ai.go` - Core AI improvements
  - `findAttackTargets()` - Victory city prioritization
  - `CombatMovePhase()` - Defensive awareness in attacks
  - `NoncombatMovePhase()` - Strategic positioning system
  - Added helper functions:
    - `identifyStrategicTargets()`
    - `findNearestVictoryCity()`
    - `evaluateTerritoryThreat()`
    - `wouldLeaveTerritoryVulnerable()`

## Testing

All existing tests pass. The improvements are behavioral enhancements that maintain backward compatibility while adding strategic depth.

Run `./npc_game_demo` to see the improved behavior in action:
- NPCs will focus attacks on victory cities
- Defensive posture maintained for owned territories
- Strategic positioning during noncombat phases
- More coherent long-term strategy

## Future Enhancements

Potential areas for further improvement:
- Multi-turn attack planning (building up forces for 2-3 turns before major assault)
- Naval strategy for island-hopping toward victory cities
- Coordination between purchases and attack goals
- Air superiority calculations for victory city assaults
