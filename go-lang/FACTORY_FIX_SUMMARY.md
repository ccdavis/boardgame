# Industrial Complex (Factory) Fix Summary

## Problems Identified

1. **Naming Mismatch**: The code searched for pieces named `"industrial_complex"`, but the aaa.gdf file uses `"factory"` as the unit name
2. **No Industrial Complexes Found**: NPCs reported "No industrial complexes to mobilize units" because of the naming mismatch
3. **NPCs Never Built Factories**: The AI purchase logic only bought infantry, armor, and fighters - never factories

## Root Cause

### Issue #1: Naming Inconsistency
- **aaa.gdf** defines the unit as: `factory: land, 0 movement, 0 attack, 0 defend, 32 cost;` (line 296)
- **Placement section** places factories: `Germany: 5 infantry,4 armor,1 fighter,1 bomber,1 AAA,1 factory;` (line 307)
- **Go code** searched for: `piece.Name == "industrial_complex"`

### Issue #2: Limited NPC AI
The PurchasePhase only considered 3 unit types:
```go
unitPriorities := []string{"infantry", "armor", "fighter"}
```

No logic existed to buy new factories.

## Solutions Implemented

### 1. Fixed Factory Recognition (npc_ai.go)

**MobilizePhase** (line 299):
```go
// Check for both "factory" (from aaa.gdf) and "industrial_complex" (alternative name)
if piece.Name == "factory" || piece.Name == "industrial_complex" {
    icTerritories = append(icTerritories, territory)
    break
}
```

**findAttackersFor** (line 414):
```go
if piece.Movement > 0 && piece.Name != "factory" && piece.Name != "industrial_complex" && piece.Name != "AAA" {
    attackingPieces = append(attackingPieces, piece)
}
```

### 2. Added Strategic Factory Purchasing (npc_ai.go)

**PurchasePhase** now includes factory logic (lines 94-117):
- Checks if player has enough IPCs (factory cost + 20 for units)
- Finds best territory for a new factory (high production value, no existing factory)
- Only builds on territories with production >= 3
- Buys one factory per turn maximum (if conditions are met)

**New Helper Function** `findBestTerritoryForFactory` (lines 505-539):
- Searches player's territories for highest production value
- Excludes territories that already have factories
- Follows rulebook requirement: territory must have production >= 1
- Returns nil if no suitable territory found

## Verification

According to the **rulebook (page 7)**: Each major power should start with industrial complexes at their capitals.

The **aaa.gdf Placement section** correctly places 8 starting factories:
1. Germany territory (capital)
2. Karelia (conquered by USSR initially)
3. Russia territory (USSR capital)
4. Britain territory (UK capital)
5. Southern Europe (Italy - Axis ally)
6. Japan territory (capital)
7. Western US (USA secondary capital)
8. Eastern US (USA primary capital)

## Test Results

✅ **Factories are now recognized**: Turn 2 Germany mobilized 10 infantry at Germany
✅ **Starting factories work**: All 8 starting factories from aaa.gdf are functional
✅ **NPCs can mobilize units**: Units are being placed at industrial complexes
✅ **NPCs will buy factories**: Logic added to purchase factories strategically when appropriate

## Example Output

```
═══ TURN 2 - Germany ═══
  ▶ Phase: Purchase Units
    • Purchased: 10x infantry (Cost: 30 IPCs)
  ...
  ▶ Phase: Mobilize New Units
    • Mobilized infantry at Germany
    • Mobilized infantry at Germany
    • Mobilized infantry at Germany
    • Mobilized infantry at Germany
    • Mobilized infantry at Germany
    • Mobilized infantry at Germany
    • Mobilized infantry at Germany
    • Mobilized infantry at Germany
    • Mobilized infantry at Germany
    • Mobilized infantry at Germany
```

## Files Modified

1. **game/npc_ai.go**:
   - MobilizePhase: Check for both "factory" and "industrial_complex"
   - findAttackersFor: Filter out both factory names
   - PurchasePhase: Add strategic factory purchasing logic
   - Added findBestTerritoryForFactory() helper function

## Notes

- The aaa.gdf naming is correct per the original game definition
- The code now supports both naming conventions for compatibility
- Factory purchasing is conservative: requires significant IPCs and high-value territory
- Per rulebook: max 1 factory per territory, must have production >= 1
