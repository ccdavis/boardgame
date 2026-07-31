package game

import (
	"boardgame/models"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

// BattleType represents the type of combat
type BattleType int

const (
	LandBattle BattleType = iota
	SeaBattle
	AirBattle
	StrategicBombing
	AmphibiousAssault
)

func (bt BattleType) String() string {
	switch bt {
	case LandBattle:
		return "Land Battle"
	case SeaBattle:
		return "Sea Battle"
	case AirBattle:
		return "Air Battle"
	case StrategicBombing:
		return "Strategic Bombing"
	case AmphibiousAssault:
		return "Amphibious Assault"
	default:
		return "Unknown"
	}
}

// Battle represents a combat encounter
type Battle struct {
	Location          string
	Type              BattleType
	Attackers         []*models.Piece
	Defenders         []*models.Piece
	AttackerID        string // Player name
	DefenderID        string // Player name
	Round             int
	AttackingPieceIDs []int // Track which pieces are attackers

	// AttackerOrigins records where each attacking piece came from, so survivors
	// of a broken-off attack can be put back. Attackers are physically moved
	// into the contested territory before combat, and nothing used to move them
	// out again on retreat -- they stayed inside enemy territory and were
	// counted as *defenders* of that enemy in the next battle fought there.
	AttackerOrigins map[int]string

	// Bombarding are warships standing off shore in support of an amphibious
	// landing. They fire once before the first round of land combat and take
	// no further part -- they are not attackers, cannot be casualties here,
	// and stay at sea whatever happens on the beach.
	Bombarding []*models.Piece

	// AmphibiousUnits is how many attackers came ashore from transports. Each
	// supporting warship may fire one bombardment shot per unit offloaded, so
	// this caps the barrage: three battleships covering a single landed
	// infantry fire once, not three times.
	AmphibiousUnits int
}

// Hit represents a successful hit in combat
type Hit struct {
	Roll      int
	Threshold int
	UnitType  string
}

// BattleResult contains the outcome of a battle
type BattleResult struct {
	AttackerWins       bool
	DefenderWins       bool
	AttackerCasualties []*models.Piece
	DefenderCasualties []*models.Piece
	AttackersRemaining []*models.Piece
	DefendersRemaining []*models.Piece
	Rounds             int
	AttackerRetreated  bool

	// BombardmentHits are the shore-bombardment shots that connected before
	// the first round, when warships supported an amphibious landing.
	BombardmentHits []Hit
}

// DiceRoller provides dice rolling functionality
type DiceRoller struct {
	rng *rand.Rand
}

// NewDiceRoller creates a new dice roller with a random seed
func NewDiceRoller() *DiceRoller {
	return &DiceRoller{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewSeededDiceRoller creates a dice roller with a specific seed (for testing)
func NewSeededDiceRoller(seed int64) *DiceRoller {
	return &DiceRoller{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// Roll rolls a single six-sided die (returns 1-6)
func (dr *DiceRoller) Roll() int {
	return dr.rng.Intn(6) + 1
}

// RollDice rolls multiple dice and returns the results
func (dr *DiceRoller) RollDice(count int) []int {
	results := make([]int, count)
	for i := 0; i < count; i++ {
		results[i] = dr.Roll()
	}
	return results
}

// CombatRound fights one round.
//
// The returned attackerHits and defenderHits are UNAPPLIED: the caller selects
// and removes casualties for them. Surprise-strike hits are not among them --
// their casualties are applied here, before the victims can fire, and are
// reported through SurpriseLosses. The hit lists used to include the surprise
// hits too, so the caller applied every submarine hit a second time against
// its own casualty selection; when the two selections picked different victims
// (the surprise pass shields submarines from air, the plain pass does not),
// one unit died twice and another was recorded dead while fighting on.
//
// Surprise-strike losses are returned per side. They used to come back as a
// single merged list, and the caller appended that same list to *both* sides'
// casualties -- so every submarine kill was reported as a loss for the killer
// as well as the victim, and both were then removed from the board.
func (dr *DiceRoller) CombatRound(battle *Battle) (attackerHits []Hit, defenderHits []Hit, surprise SurpriseLosses) {
	battle.Round++
	attackerHits = make([]Hit, 0)
	defenderHits = make([]Hit, 0)
	surprise.Attacker = make([]*models.Piece, 0)
	surprise.Defender = make([]*models.Piece, 0)

	// STEP 2: Submarine Surprise Strike (only in sea battles)
	if battle.Type == SeaBattle {
		attackerHasDestroyer := hasDestroyer(battle.Attackers)
		defenderHasDestroyer := hasDestroyer(battle.Defenders)

		// Attacking submarines fire surprise strike if no defending destroyer
		if !defenderHasDestroyer {
			attackingSubs := getSubmarines(battle.Attackers)
			for _, sub := range attackingSubs {
				roll := dr.Roll()
				if roll <= int(sub.Attack) {
					surprise.AttackerHits = append(surprise.AttackerHits, Hit{
						Roll:      roll,
						Threshold: int(sub.Attack),
						UnitType:  sub.Name,
					})
				}
			}

			// Select casualties from defender - these units don't get to fire back
			if len(surprise.AttackerHits) > 0 {
				// Casualties from surprise strikes cannot hit submarines with air
				casualties := SelectCasualtiesAvoidingAir(battle.Defenders, len(surprise.AttackerHits), !defenderHasDestroyer)
				surprise.Defender = append(surprise.Defender, casualties...)
				battle.Defenders = RemoveCasualties(battle.Defenders, casualties)
			}
		}

		// Defending submarines fire surprise strike if no attacking destroyer
		if !attackerHasDestroyer {
			defendingSubs := getSubmarines(battle.Defenders)
			for _, sub := range defendingSubs {
				roll := dr.Roll()
				if roll <= int(sub.Defend) {
					surprise.DefenderHits = append(surprise.DefenderHits, Hit{
						Roll:      roll,
						Threshold: int(sub.Defend),
						UnitType:  sub.Name,
					})
				}
			}

			// Select casualties from attacker - these units don't get to fire back
			if len(surprise.DefenderHits) > 0 {
				casualties := SelectCasualtiesAvoidingAir(battle.Attackers, len(surprise.DefenderHits), !attackerHasDestroyer)
				surprise.Attacker = append(surprise.Attacker, casualties...)
				battle.Attackers = RemoveCasualties(battle.Attackers, casualties)
			}
		}
	}

	// STEP 3 & 4: Regular combat (non-submarine units, or submarines if enemy destroyer present)
	// Attackers roll (excluding submarines that already fired)
	attackerRegularHits := dr.RollForNonSubmarineUnits(battle.Attackers, true, battle.Type == SeaBattle && !hasDestroyer(battle.Defenders))
	attackerHits = append(attackerHits, attackerRegularHits...)

	// If defending subs didn't get surprise strike (enemy has destroyer), they fire now
	if battle.Type == SeaBattle && hasDestroyer(battle.Attackers) {
		defendingSubs := getSubmarines(battle.Defenders)
		subHits := dr.RollForUnits(defendingSubs, false)
		defenderHits = append(defenderHits, subHits...)
	}

	// Defenders roll (excluding submarines that already fired)
	defenderRegularHits := dr.RollForNonSubmarineUnits(battle.Defenders, false, battle.Type == SeaBattle && !hasDestroyer(battle.Attackers))
	defenderHits = append(defenderHits, defenderRegularHits...)

	// If attacking subs didn't get surprise strike (enemy has destroyer), they fire now
	if battle.Type == SeaBattle && hasDestroyer(battle.Defenders) {
		attackingSubs := getSubmarines(battle.Attackers)
		subHits := dr.RollForUnits(attackingSubs, true)
		attackerHits = append(attackerHits, subHits...)
	}

	return attackerHits, defenderHits, surprise
}

// RollForNonSubmarineUnits rolls for all non-submarine units
// If subsCannotBeHit is true, excludes submarines from being valid targets
func (dr *DiceRoller) RollForNonSubmarineUnits(units []*models.Piece, isAttacking bool, subsCannotBeHit bool) []Hit {
	hits := make([]Hit, 0)
	nonSubs := getNonSubmarines(units)

	for _, unit := range nonSubs {
		threshold := int(unit.Defend)
		if isAttacking {
			threshold = int(unit.Attack)
		}

		roll := dr.Roll()
		if roll <= threshold {
			hits = append(hits, Hit{
				Roll:      roll,
				Threshold: threshold,
				UnitType:  unit.Name,
			})
		}
	}

	return hits
}

// SelectCasualtiesAvoidingAir selects casualties, optionally sparing submarines.
//
// A submerged submarine cannot be hit by aircraft, and in a fleet with no
// destroyer to hold it down that means aircraft cannot target it at all. This
// was a passthrough to SelectCasualties that ignored its own flag, so
// submarines were freely killed by air units the rules say cannot reach them.
func SelectCasualtiesAvoidingAir(units []*models.Piece, hitCount int, subsCannotBeHitByAir bool) []*models.Piece {
	if !subsCannotBeHitByAir {
		return SelectCasualties(units, hitCount)
	}

	// Split out the submarines, take casualties from everything else first, and
	// only fall back to submarines when nothing else is left to lose.
	targetable := make([]*models.Piece, 0, len(units))
	protected := make([]*models.Piece, 0)
	for _, unit := range units {
		if models.CapabilitiesOf(unit).IsSubmarine {
			protected = append(protected, unit)
		} else {
			targetable = append(targetable, unit)
		}
	}

	casualties := SelectCasualties(targetable, hitCount)
	if remaining := hitCount - len(casualties); remaining > 0 && len(protected) > 0 {
		casualties = append(casualties, SelectCasualties(protected, remaining)...)
	}
	return casualties
}

// RollForUnits rolls dice for all units and returns hits
func (dr *DiceRoller) RollForUnits(units []*models.Piece, isAttacking bool) []Hit {
	hits := make([]Hit, 0)

	for _, unit := range units {
		threshold := int(unit.Defend)
		if isAttacking {
			threshold = int(unit.Attack)
		}

		roll := dr.Roll()
		if roll <= threshold {
			hits = append(hits, Hit{
				Roll:      roll,
				Threshold: threshold,
				UnitType:  unit.Name,
			})
		}
	}

	return hits
}

// SurpriseLosses carries everything about the submarine first strike: the
// hits each side's submarines scored, and the units each side lost to them.
// The casualties are already applied when CombatRound returns.
type SurpriseLosses struct {
	Attacker []*models.Piece // attacking units killed by defending submarines
	Defender []*models.Piece // defending units killed by attacking submarines

	AttackerHits []Hit // hits scored by attacking submarines (already applied)
	DefenderHits []Hit // hits scored by defending submarines (already applied)
}

// Helper functions for submarine and destroyer mechanics.
//
// These derive capabilities from the piece itself (models.CapabilitiesOf)
// rather than comparing names at the call site or consulting shared state.
// Two earlier designs both failed: literal name tests ("submarine",
// "destroyer") never matched aaa.gdf's "sub", so the rules were silently
// inert; and the package-global registry that replaced them was overwritten
// by every controller and read during every battle, so concurrent games raced
// it and games on different boards used each other's unit rules.

// hasDestroyer checks whether any unit cancels submarine abilities
func hasDestroyer(units []*models.Piece) bool {
	for _, unit := range units {
		if models.CapabilitiesOf(unit).NegatesSubmarines {
			return true
		}
	}
	return false
}

// getSubmarines returns all submarines from unit list
func getSubmarines(units []*models.Piece) []*models.Piece {
	subs := make([]*models.Piece, 0)
	for _, unit := range units {
		if models.CapabilitiesOf(unit).IsSubmarine {
			subs = append(subs, unit)
		}
	}
	return subs
}

// getNonSubmarines returns all non-submarine units
func getNonSubmarines(units []*models.Piece) []*models.Piece {
	nonSubs := make([]*models.Piece, 0)
	for _, unit := range units {
		if !models.CapabilitiesOf(unit).IsSubmarine {
			nonSubs = append(nonSubs, unit)
		}
	}
	return nonSubs
}

// getAirUnits returns all air units from unit list
func getAirUnits(units []*models.Piece) []*models.Piece {
	airUnits := make([]*models.Piece, 0)
	for _, unit := range units {
		if unit.Terrain == models.Air {
			airUnits = append(airUnits, unit)
		}
	}
	return airUnits
}

// getSeaUnits returns all sea units (excluding air units and submarines)
func getSeaUnits(units []*models.Piece) []*models.Piece {
	seaUnits := make([]*models.Piece, 0)
	for _, unit := range units {
		if unit.Terrain == models.Water && !models.CapabilitiesOf(unit).IsSubmarine {
			seaUnits = append(seaUnits, unit)
		}
	}
	return seaUnits
}

// getBombardmentShips returns ships capable of bombardment (battleships and cruisers)
func getBombardmentShips(units []*models.Piece) []*models.Piece {
	ships := make([]*models.Piece, 0)
	for _, unit := range units {
		if models.CapabilitiesOf(unit).CanBombard {
			ships = append(ships, unit)
		}
	}
	return ships
}

// getLandUnits returns all land units
func getLandUnits(units []*models.Piece) []*models.Piece {
	landUnits := make([]*models.Piece, 0)
	for _, unit := range units {
		if unit.Terrain == models.Land {
			landUnits = append(landUnits, unit)
		}
	}
	return landUnits
}

// RollAAAFire rolls AAA fire against attacking air units
// Each AAA can fire up to 3 times or once per air unit (whichever is less)
// Hits on a roll of 1
func (dr *DiceRoller) RollAAAFire(aaaUnits []*models.Piece, airUnits []*models.Piece) []Hit {
	hits := make([]Hit, 0)

	for _, aaa := range aaaUnits {
		// Each AAA fires up to 3 shots or 1 per air unit
		shots := 3
		if len(airUnits) < shots {
			shots = len(airUnits)
		}

		for i := 0; i < shots; i++ {
			roll := dr.Roll()
			if roll == 1 {
				hits = append(hits, Hit{
					Roll:      roll,
					Threshold: 1,
					UnitType:  aaa.Name,
				})
			}
		}
	}

	return hits
}

// RollBombardment rolls offshore bombardment for amphibious assaults
// Each battleship/cruiser can support one land unit being offloaded
// Battleships fire at attack value 4, cruisers at 3
// Returns hits that will be applied to defenders before land combat
func (dr *DiceRoller) RollBombardment(bombardingShips []*models.Piece, unitsBeingOffloaded int) []Hit {
	hits := make([]Hit, 0)

	// Number of ships that can bombard is limited by units being offloaded
	bombardCount := len(bombardingShips)
	if unitsBeingOffloaded < bombardCount {
		bombardCount = unitsBeingOffloaded
	}

	// Roll for each bombarding ship
	for i := 0; i < bombardCount; i++ {
		ship := bombardingShips[i]
		roll := dr.Roll()
		threshold := int(ship.Attack) // Battleship=4, Cruiser=3

		if roll <= threshold {
			hits = append(hits, Hit{
				Roll:      roll,
				Threshold: threshold,
				UnitType:  ship.Name,
			})
		}
	}

	return hits
}

// StrategicBombingResult contains the outcome of a strategic bombing raid
type StrategicBombingResult struct {
	TotalBombers     int
	BombersDestroyed int // Shot down by AA
	BombersSurvived  int
	DamageRolls      []int // Damage from each surviving bomber
	TotalDamage      int
	AAHits           []Hit
}

// RollICAADefense rolls industrial complex built-in AA defense
// Each IC fires at each bomber with a roll of 1 hitting
func (dr *DiceRoller) RollICAADefense(bombers []*models.Piece) []Hit {
	hits := make([]Hit, 0)

	// IC gets one shot at each bomber
	for range bombers {
		roll := dr.Roll()
		if roll == 1 {
			hits = append(hits, Hit{
				Roll:      roll,
				Threshold: 1,
				UnitType:  "IC_AA",
			})
		}
	}

	return hits
}

// ResolveStrategicBombing executes a strategic bombing raid on an industrial complex
// Steps: 1) IC AA defense fires, 2) Surviving bombers roll for damage (1d6 each)
func ResolveStrategicBombing(bombers []*models.Piece, diceRoller *DiceRoller) (*StrategicBombingResult, error) {
	if diceRoller == nil {
		diceRoller = NewDiceRoller()
	}

	result := &StrategicBombingResult{
		TotalBombers: len(bombers),
		DamageRolls:  make([]int, 0),
		AAHits:       make([]Hit, 0),
	}

	// STEP 1: IC AA Defense
	aaHits := diceRoller.RollICAADefense(bombers)
	result.AAHits = aaHits
	result.BombersDestroyed = len(aaHits)

	// Remove destroyed bombers
	survivingBombers := len(bombers) - result.BombersDestroyed
	if survivingBombers < 0 {
		survivingBombers = 0
	}
	result.BombersSurvived = survivingBombers

	// STEP 2: Surviving bombers roll for damage
	for i := 0; i < survivingBombers; i++ {
		damageRoll := diceRoller.Roll()
		result.DamageRolls = append(result.DamageRolls, damageRoll)
		result.TotalDamage += damageRoll
	}

	return result, nil
}

// ApplyICDamage applies damage to a territory's industrial complex
// Returns actual damage applied (respecting max damage = 2x production)
func ApplyICDamage(territory *models.Territory, damage int) int {
	maxDamage := territory.Production * 2

	// Calculate how much damage can be applied
	currentDamage := territory.ICDamage
	remainingCapacity := maxDamage - currentDamage

	if remainingCapacity <= 0 {
		return 0 // Already at max damage
	}

	actualDamage := damage
	if actualDamage > remainingCapacity {
		actualDamage = remainingCapacity
	}

	territory.ICDamage += actualDamage
	return actualDamage
}

// RepairIC repairs industrial complex damage
// Costs 1 IPC per damage point repaired
func RepairIC(territory *models.Territory, repairAmount int) int {
	if territory.ICDamage <= 0 {
		return 0 // No damage to repair
	}

	actualRepair := repairAmount
	if actualRepair > territory.ICDamage {
		actualRepair = territory.ICDamage
	}

	territory.ICDamage -= actualRepair
	return actualRepair
}

// GetEffectiveProduction returns the effective production capacity after damage
func GetEffectiveProduction(territory *models.Territory) int {
	effective := territory.Production - territory.ICDamage
	if effective < 0 {
		effective = 0
	}
	return effective
}

// ApplyArtillerySupport modifies infantry attack values when supported by
// artillery: for every artillery, one infantry gets +1 attack (from 1 to 2).
//
// It returns the pre-boost attack values so RemoveArtillerySupport can undo
// the boost exactly. The record used to live in a package-level map, which two
// concurrent battles -- one per game session -- wrote without a lock.
func ApplyArtillerySupport(units []*models.Piece) map[*models.Piece]int16 {
	artilleryCount := 0
	infantryNeedingSupport := make([]*models.Piece, 0)

	// Count artillery and infantry. Both ends of the pairing come from the
	// capability registry -- the artillery side always did, but the infantry
	// side compared unit.Name against the literal "infantry", the exact
	// spelling-dependence the registry exists to remove.
	for _, unit := range units {
		caps := models.CapabilitiesOf(unit)
		if caps.SupportsInfantry {
			artilleryCount++
		} else if caps.SupportedByArtillery && unit.Attack == 1 {
			infantryNeedingSupport = append(infantryNeedingSupport, unit)
		}
	}

	// Apply support up to the number of artillery
	supportCount := artilleryCount
	if len(infantryNeedingSupport) < supportCount {
		supportCount = len(infantryNeedingSupport)
	}

	boosted := make(map[*models.Piece]int16, supportCount)
	for i := 0; i < supportCount; i++ {
		boosted[infantryNeedingSupport[i]] = infantryNeedingSupport[i].Attack
		infantryNeedingSupport[i].Attack = 2
	}
	return boosted
}

// RemoveArtillerySupport undoes the boost applied by ApplyArtillerySupport.
//
// It restores each boosted unit to the attack value it actually had. Matching
// on "infantry with attack 2" instead would demote infantry whose base attack is
// 2 on some other board, permanently weakening units that were never boosted.
func RemoveArtillerySupport(boosted map[*models.Piece]int16) {
	for unit, original := range boosted {
		unit.Attack = original
	}
}

// SelectCasualties selects which units to remove as casualties
// Handles multi-hit units like battleships (require 2 hits to destroy)
func SelectCasualties(units []*models.Piece, hitCount int) []*models.Piece {
	if hitCount <= 0 || len(units) == 0 {
		return []*models.Piece{}
	}

	casualties := make([]*models.Piece, 0)

	// Cheapest first, so the expensive units survive longest.
	sortedUnits := make([]*models.Piece, len(units))
	copy(sortedUnits, units)
	sort.SliceStable(sortedUnits, func(i, j int) bool {
		return sortedUnits[i].Cost < sortedUnits[j].Cost
	})

	// Apply hits
	for hitCount > 0 && len(sortedUnits) > 0 {
		unit := sortedUnits[0]

		// Multi-hit units absorb a hit and stay in the fight. How many hits a
		// unit takes comes from the registry rather than a name comparison, so a
		// board that calls its capital ship something else still works.
		maxHits := models.CapabilitiesOf(unit).MaxHits
		if maxHits > 1 {
			unit.Hits++
			hitCount--

			if unit.Hits >= maxHits {
				casualties = append(casualties, unit)
				sortedUnits = sortedUnits[1:]
			}
			continue
		}

		// Regular unit - destroyed with 1 hit
		casualties = append(casualties, unit)
		sortedUnits = sortedUnits[1:]
		hitCount--
	}

	return casualties
}

// RepairDamagedUnits clears accumulated damage from surviving units.
//
// Damage is per-battle: a battleship that soaks a hit is at full strength for
// the next engagement. Nothing ever reset Hits, so a damaged battleship stayed
// a one-hit unit for the remainder of the game.
func RepairDamagedUnits(units []*models.Piece) {
	for _, unit := range units {
		if unit != nil {
			unit.Hits = 0
		}
	}
}

// RemoveCasualties removes casualties from the unit list
func RemoveCasualties(units []*models.Piece, casualties []*models.Piece) []*models.Piece {
	remaining := make([]*models.Piece, 0, len(units))

	// Create a map of casualties for quick lookup
	casualtyMap := make(map[*models.Piece]bool)
	for _, casualty := range casualties {
		casualtyMap[casualty] = true
	}

	// Keep units that are not casualties
	for _, unit := range units {
		if !casualtyMap[unit] {
			remaining = append(remaining, unit)
		}
	}

	return remaining
}

// AmphibiousAssaultResult contains the full outcome of an amphibious assault
type AmphibiousAssaultResult struct {
	SeaBattleResult  *BattleResult // nil if no sea battle
	BombardmentHits  []Hit
	LandBattleResult *BattleResult
	Success          bool // True if attackers won
}

// ResolveAmphibiousAssault executes a full amphibious assault
// Steps: 1) Sea combat (if enemy ships present), 2) Bombardment, 3) Land combat
func ResolveAmphibiousAssault(
	location string,
	attackingShips []*models.Piece,
	attackingLandUnits []*models.Piece,
	defendingShips []*models.Piece,
	defendingLandUnits []*models.Piece,
	attackerID string,
	defenderID string,
	diceRoller *DiceRoller,
) (*AmphibiousAssaultResult, error) {
	if diceRoller == nil {
		diceRoller = NewDiceRoller()
	}

	result := &AmphibiousAssaultResult{
		BombardmentHits: make([]Hit, 0),
	}

	// STEP 1: Sea battle (if defending ships present)
	if len(defendingShips) > 0 {
		seaBattle := NewBattle(location+" (Sea)", SeaBattle, attackerID, defenderID)
		seaBattle.Attackers = attackingShips
		seaBattle.Defenders = defendingShips

		seaResult, err := ResolveCombat(seaBattle, diceRoller, 100)
		if err != nil {
			return nil, fmt.Errorf("sea battle failed: %v", err)
		}
		result.SeaBattleResult = seaResult

		// If attackers lost sea battle, amphibious assault fails
		if !seaResult.AttackerWins {
			result.Success = false
			return result, nil
		}

		// Update attacking ships to survivors
		attackingShips = seaResult.AttackersRemaining
	}

	// STEP 2: Offshore bombardment
	bombardShips := getBombardmentShips(attackingShips)
	if len(bombardShips) > 0 && len(attackingLandUnits) > 0 {
		bombardmentHits := diceRoller.RollBombardment(bombardShips, len(attackingLandUnits))
		result.BombardmentHits = bombardmentHits

		// Apply bombardment casualties to defenders
		if len(bombardmentHits) > 0 {
			bombardmentCasualties := SelectCasualties(defendingLandUnits, len(bombardmentHits))
			defendingLandUnits = RemoveCasualties(defendingLandUnits, bombardmentCasualties)
		}
	}

	// STEP 3: Land combat
	landBattle := NewBattle(location, LandBattle, attackerID, defenderID)
	landBattle.Attackers = attackingLandUnits
	landBattle.Defenders = defendingLandUnits

	landResult, err := ResolveCombat(landBattle, diceRoller, 100)
	if err != nil {
		return nil, fmt.Errorf("land battle failed: %v", err)
	}
	result.LandBattleResult = landResult
	result.Success = landResult.AttackerWins

	return result, nil
}

// RetreatDecider is a callback function that determines if the attacker should retreat
// It receives: initialAttackers, currentAttackers, initialDefenders, currentDefenders, currentRound
// Returns true if the attacker should retreat
type RetreatDecider func(int, int, int, int, int) bool

// ResolveCombat executes a complete battle until one side wins or retreats
func ResolveCombat(battle *Battle, diceRoller *DiceRoller, maxRounds int) (*BattleResult, error) {
	return ResolveCombatWithRetreat(battle, diceRoller, maxRounds, nil)
}

// ResolveCombatWithRetreat executes a complete battle with optional retreat decision callback
func ResolveCombatWithRetreat(battle *Battle, diceRoller *DiceRoller, maxRounds int, retreatDecider RetreatDecider) (*BattleResult, error) {
	if diceRoller == nil {
		diceRoller = NewDiceRoller()
	}

	if maxRounds <= 0 {
		maxRounds = 100 // Safety limit
	}

	result := &BattleResult{
		AttackerCasualties: make([]*models.Piece, 0),
		DefenderCasualties: make([]*models.Piece, 0),
		Rounds:             0,
	}

	attackers := make([]*models.Piece, len(battle.Attackers))
	defenders := make([]*models.Piece, len(battle.Defenders))
	copy(attackers, battle.Attackers)
	copy(defenders, battle.Defenders)

	initialAttackerCount := len(attackers)
	initialDefenderCount := len(defenders)

	// AAA FIRE PHASE (before combat, only happens once).
	//
	// Anti-aircraft artillery fires its pre-combat shots and then sits the
	// battle out: it does not roll in the combat rounds, cannot be chosen as a
	// casualty, and cannot keep a battle alive on its own. It used to stay in
	// the defender list afterwards, where it double-dipped -- special shots
	// first, then defence dice every round like any other unit. If the defence
	// falls, the gun is captured with the territory, like a factory.
	aaaUnits := make([]*models.Piece, 0)
	airUnits := make([]*models.Piece, 0)

	for _, unit := range defenders {
		if models.CapabilitiesOf(unit).IsAA {
			aaaUnits = append(aaaUnits, unit)
		}
	}

	for _, unit := range attackers {
		if unit.Terrain == models.Air {
			airUnits = append(airUnits, unit)
		}
	}

	// Roll AAA fire if there are both AAA and air units
	if len(aaaUnits) > 0 && len(airUnits) > 0 {
		aaaHits := diceRoller.RollAAAFire(aaaUnits, airUnits)
		if len(aaaHits) > 0 {
			// Select air casualties
			aaaCasualties := SelectCasualties(airUnits, len(aaaHits))
			result.AttackerCasualties = append(result.AttackerCasualties, aaaCasualties...)
			attackers = RemoveCasualties(attackers, aaaCasualties)
		}
	}

	// Its shots fired, the AAA withdraws from the fight.
	defenders = RemoveCasualties(defenders, aaaUnits)
	initialDefenderCount = len(defenders)

	// SHORE BOMBARDMENT (once, before the first round).
	//
	// Warships covering an amphibious landing each fire one supporting shot,
	// capped at one per unit that came ashore. Casualties are removed before
	// combat begins and do not fire back -- the Classic rule, which this board
	// follows; the 1942 second edition lets them return fire once.
	if len(battle.Bombarding) > 0 && len(defenders) > 0 && battle.AmphibiousUnits > 0 {
		bombardmentHits := diceRoller.RollBombardment(battle.Bombarding, battle.AmphibiousUnits)
		result.BombardmentHits = bombardmentHits
		if len(bombardmentHits) > 0 {
			casualties := SelectCasualties(defenders, len(bombardmentHits))
			result.DefenderCasualties = append(result.DefenderCasualties, casualties...)
			defenders = RemoveCasualties(defenders, casualties)
		}
	}

	// ARTILLERY SUPPORT (applied before combat begins, undone after -- the
	// boost record covers casualties too, since it holds the boosted pieces
	// themselves rather than scanning a list of survivors)
	boosted := ApplyArtillerySupport(attackers)

	// Fight until one side is eliminated or max rounds reached
	for result.Rounds < maxRounds {
		// Check if battle is over
		if len(attackers) == 0 {
			result.DefenderWins = true
			break
		}
		if len(defenders) == 0 {
			result.AttackerWins = true
			break
		}

		// Check if attacker wants to retreat (after first round)
		if result.Rounds > 0 && retreatDecider != nil {
			if retreatDecider(initialAttackerCount, len(attackers), initialDefenderCount, len(defenders), result.Rounds) {
				result.AttackerRetreated = true
				break
			}
		}

		// Execute one combat round
		battle.Attackers = attackers
		battle.Defenders = defenders
		attackerHits, defenderHits, surprise := diceRoller.CombatRound(battle)

		// Surprise casualties were already removed from the battle in
		// CombatRound; record each side's losses against that side -- and
		// adopt the thinned lists. The local slices used to keep the victims:
		// a submarine's kill was recorded as a casualty yet fought on in every
		// later round, and if the attack then broke off, the controller
		// deleted the piece as a casualty AND "withdrew" it as a survivor,
		// leaving its dead ID in the origin territory's list.
		attackers = battle.Attackers
		defenders = battle.Defenders
		result.AttackerCasualties = append(result.AttackerCasualties, surprise.Attacker...)
		result.DefenderCasualties = append(result.DefenderCasualties, surprise.Defender...)

		// Select and remove remaining casualties from regular combat
		defenderCasualties := SelectCasualties(defenders, len(attackerHits))
		attackerCasualties := SelectCasualties(attackers, len(defenderHits))

		// Track all casualties
		result.DefenderCasualties = append(result.DefenderCasualties, defenderCasualties...)
		result.AttackerCasualties = append(result.AttackerCasualties, attackerCasualties...)

		// Remove casualties from active forces
		defenders = RemoveCasualties(defenders, defenderCasualties)
		attackers = RemoveCasualties(attackers, attackerCasualties)

		result.Rounds++
	}

	// Remove artillery support from every boosted unit, casualties included
	RemoveArtillerySupport(boosted)

	// Damage does not carry between battles.
	RepairDamagedUnits(attackers)
	RepairDamagedUnits(defenders)

	result.AttackersRemaining = attackers
	result.DefendersRemaining = defenders

	return result, nil
}

// NewBattle creates a new battle
func NewBattle(location string, battleType BattleType, attackerID, defenderID string) *Battle {
	return &Battle{
		Location:          location,
		Type:              battleType,
		Attackers:         make([]*models.Piece, 0),
		Defenders:         make([]*models.Piece, 0),
		AttackerID:        attackerID,
		DefenderID:        defenderID,
		Round:             0,
		AttackingPieceIDs: make([]int, 0),
		AttackerOrigins:   make(map[int]string),
	}
}

// AddAttacker adds an attacking unit to the battle
func (b *Battle) AddAttacker(piece *models.Piece) {
	b.Attackers = append(b.Attackers, piece)
}

// AddDefender adds a defending unit to the battle
func (b *Battle) AddDefender(piece *models.Piece) {
	b.Defenders = append(b.Defenders, piece)
}

// GetTotalAttackPower returns the sum of all attacker values
func (b *Battle) GetTotalAttackPower() int {
	total := 0
	for _, piece := range b.Attackers {
		total += int(piece.Attack)
	}
	return total
}

// GetTotalDefensePower returns the sum of all defender values
func (b *Battle) GetTotalDefensePower() int {
	total := 0
	for _, piece := range b.Defenders {
		total += int(piece.Defend)
	}
	return total
}

// String returns a string representation of the battle
func (b *Battle) String() string {
	return fmt.Sprintf("%s at %s: %s (%d units, power %d) vs %s (%d units, power %d)",
		b.Type, b.Location,
		b.AttackerID, len(b.Attackers), b.GetTotalAttackPower(),
		b.DefenderID, len(b.Defenders), b.GetTotalDefensePower())
}

// EstimateAttackSuccess estimates the probability of attacker winning
// Returns a value between 0.0 and 1.0
func EstimateAttackSuccess(attackers []*models.Piece, defenders []*models.Piece) float64 {
	attackPower := 0
	defendPower := 0

	// Calculate total attack power
	for _, unit := range attackers {
		attackPower += int(unit.Attack)
	}

	// Calculate total defense power
	for _, unit := range defenders {
		defendPower += int(unit.Defend)
	}

	// Simple heuristic: compare power ratios
	// This is a rough estimate, actual combat involves dice rolls
	if defendPower == 0 {
		return 1.0 // Guaranteed win if no defenders
	}

	ratio := float64(attackPower) / float64(defendPower)

	// Convert ratio to probability estimate
	// ratio < 0.5: very unlikely to win
	// ratio = 1.0: even match, ~50% chance
	// ratio > 2.0: very likely to win
	if ratio < 0.5 {
		return 0.1 + (ratio * 0.3) // 10-25% chance
	} else if ratio < 1.0 {
		return 0.25 + (ratio-0.5)*0.5 // 25-50% chance
	} else if ratio < 2.0 {
		return 0.5 + (ratio-1.0)*0.35 // 50-85% chance
	} else {
		return 0.85 + math.Min((ratio-2.0)*0.05, 0.14) // 85-99% chance
	}
}

// ShouldAttackerRetreat determines if the attacker should retreat based on casualties
// Returns true if the attacker is taking disproportionate losses
func ShouldAttackerRetreat(initialAttackers, currentAttackers, initialDefenders, currentDefenders int, round int) bool {
	// Don't retreat in the first round
	if round < 2 {
		return false
	}

	attackerLosses := initialAttackers - currentAttackers
	defenderLosses := initialDefenders - currentDefenders

	// Retreat if we've lost everything
	if currentAttackers == 0 {
		return false // Can't retreat, we're dead
	}

	// Retreat if we've lost more than 70% of our forces and defenders still have >50%
	attackerLossRatio := float64(attackerLosses) / float64(initialAttackers)
	defenderLossRatio := float64(defenderLosses) / float64(initialDefenders)

	if attackerLossRatio > 0.7 && defenderLossRatio < 0.5 {
		return true
	}

	// Retreat if we're trading unfavorably (losing 2+ units for every 1 defender killed)
	if defenderLosses > 0 {
		lossRatio := float64(attackerLosses) / float64(defenderLosses)
		if lossRatio >= 2.0 && currentAttackers < currentDefenders {
			return true
		}
	}

	// Retreat if we're outnumbered 3:1 or more after losses
	if currentDefenders >= currentAttackers*3 {
		return true
	}

	return false
}

// CalculateTerritoryValue estimates the strategic value of a territory for attack decision
func CalculateTerritoryValue(territory *models.Territory, isVictoryCity bool) int {
	value := territory.Production

	// Victory cities are worth much more
	if isVictoryCity {
		value += 5
	}

	return value
}
