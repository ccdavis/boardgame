# Neutral Territory Rules Implementation

## Overview

This implementation adds proper neutral territory rules to the Axis & Allies game engine, fixing the previous issue where players could attack any neutral territory without restrictions.

## Problem Statement

Previously, the game allowed:
- USA to attack Argentina from Brazil
- Germany to attack Turkey
- Any power to attack any neutral territory

This violated standard Axis & Allies rules and dramatically changed game balance.

## Solution

### Three Types of Neutral Territories

The implementation introduces three neutral territory types:

#### 1. **Strict Neutrals** (Cannot be attacked)
- **Territories**: Turkey, Afghanistan, Syria, Mongolia
- **Rules**:
  - Cannot be attacked by any power
  - If ANY strict neutral is attacked, ALL strict neutrals become hostile to the attacker
  - They immediately join the opposing alliance with defending infantry

#### 2. **Pro-Allied Neutrals** (Friendly to Allies)
- **Territories**: Colombia, Venezuela, Peru, Chile, Argentina, Arabia, Iraq, Mozambique, Angola
- **Rules**:
  - Can be peacefully activated by Allied powers during noncombat move
  - When activated, they join the activating power with free infantry (1 per production value)
  - Can be attacked by Axis powers (but defenders will resist)

#### 3. **Pro-Axis Neutrals** (Friendly to Axis)
- **Territories**: None currently in aaa.gdf
- **Rules**:
  - Can be peacefully activated by Axis powers during noncombat move
  - Can be attacked by Allied powers

#### 4. **Not Neutral** (Normal territories)
- All water territories owned by "Neutral" are treated as not neutral
- Can be freely traversed

## Implementation Details

### Code Changes

#### 1. Models (`models/models.go`)
- Added `NeutralType` enum with four values:
  - `NotNeutral`: Normal territories
  - `StrictNeutral`: Cannot attack
  - `ProAlliedNeutral`: Can activate as Allies
  - `ProAxisNeutral`: Can activate as Axis

- Added `NeutralType` field to `Territory` struct
- Added `determineNeutralType()` function that automatically classifies neutrals based on territory name
- Added `ParseNeutralType()` for future .gdf file support

#### 2. Movement Rules (`game/movement.go`)
- Modified `canTraverseTerritory()` to check neutral territory rules:
  - **Combat moves**: Cannot target strict neutrals
  - **Combat moves**: Can attack pro-Allied neutrals (as Axis) or pro-Axis neutrals (as Allies)
  - **Noncombat moves**: Can activate compatible neutrals
  - **Noncombat moves**: Can move into allied territories

- Added `canAttackNeutral()` to validate attacks on neutral territories
- Added `canActivateNeutral()` to check if a neutral can be peacefully activated
- Fixed waypoint traversal to allow neutral water territories

#### 3. Game Controller (`game/controller.go`)
- Modified `ExecuteCombatMoves()`:
  - Detects when strict neutrals are attacked
  - Triggers chain reaction via `TriggerStrictNeutralChainReaction()`

- Modified `ExecuteNoncombatMoves()`:
  - Detects when pro-Allied/pro-Axis neutrals are activated
  - Calls `ActivateNeutralTerritory()` to transfer ownership

- Added `TriggerStrictNeutralChainReaction()`:
  - Finds the opposing alliance
  - Transfers all strict neutrals to an enemy major power
  - Adds defending infantry (1 per production value)

- Added `ActivateNeutralTerritory()`:
  - Validates activation rules
  - Transfers territory ownership
  - Adds free infantry to the activated territory

### Testing

Created comprehensive test suite (`game/neutral_test.go`) with 6 tests:

1. **TestCannotAttackStrictNeutral**: Verifies strict neutrals cannot be attacked
2. **TestCanActivateProAlliedNeutral**: Verifies Allies can activate pro-Allied neutrals
3. **TestCannotActivateProAlliedNeutralAsAxis**: Verifies Axis cannot activate pro-Allied neutrals
4. **TestAxisCanAttackProAlliedNeutral**: Verifies Axis can attack pro-Allied neutrals
5. **TestStrictNeutralChainReaction**: Verifies attacking one strict neutral makes all hostile
6. **TestNeutralWaterTerritoriesNotRestricted**: Verifies neutral water can be freely traversed

**All tests pass** ✅

## Examples

### Scenario 1: USA Cannot Attack Argentina
```
USA infantry in Brazil -> Cannot combat move to Argentina
Reason: Argentina is pro-Allied, and USA is Allied
Solution: USA can activate Argentina during noncombat move instead
Result: Argentina joins USA with 1 free infantry
```

### Scenario 2: Germany Cannot Attack Turkey
```
Germany infantry in Southern Europe -> Cannot combat move to Turkey
Reason: Turkey is a strict neutral
Result: Move is blocked, error returned
```

### Scenario 3: Germany Attacks Turkey (Hypothetical)
```
If Germany somehow attacked Turkey:
1. Turkey becomes owned by USSR (enemy major power)
2. Turkey gets 4 infantry (production value)
3. Afghanistan gets 1 infantry and joins USSR
4. Syria gets 1 infantry and joins USSR
5. Mongolia gets 1 infantry and joins USSR
All strict neutrals are now hostile to Axis!
```

### Scenario 4: Japan Can Attack Colombia
```
Japan (Axis) -> Can attack Colombia (pro-Allied)
Colombia will defend with units if present
If Japan wins, Colombia is conquered normally
```

## Game Balance Impact

This implementation fixes major game balance issues:

1. **South America Protected**: USA and UK can peacefully expand in South America, gaining free infantry
2. **Strategic Deterrent**: Attacking Turkey has severe consequences (all strict neutrals become hostile)
3. **Realistic Diplomacy**: Models historical WWII neutral country positions
4. **Water Movement Fixed**: Naval units can move through neutral sea zones

## Backward Compatibility

The implementation is backward compatible:
- Existing game files work without modification
- Neutral types are auto-detected based on territory names
- Water territories owned by "Neutral" automatically become NotNeutral type
- Future .gdf files can specify neutral types explicitly

## Future Enhancements

Potential improvements:
1. Add neutral type specification to .gdf file format
2. Implement Mongolia special rules (becomes pro-Allied if Japan attacks USSR)
3. Add UI indicators for neutral territory types
4. Add game log messages when neutrals are activated/attacked
5. Track which power activated each neutral for history

## Summary

The neutral territory system now correctly implements Axis & Allies rules:
- ✅ Strict neutrals cannot be attacked
- ✅ Strict neutral chain reaction works
- ✅ Pro-Allied neutrals can be peacefully activated by Allies
- ✅ Pro-Allied neutrals can be attacked by Axis
- ✅ Allied powers can move into each other's territories
- ✅ Neutral water can be freely traversed
- ✅ All existing tests still pass
- ✅ New comprehensive test coverage

The game now provides a much more authentic Axis & Allies experience with proper neutral territory mechanics.
