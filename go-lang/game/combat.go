package game

import (
	"boardgame/models"
	"fmt"
	"math/rand"
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
	Location      string
	Type          BattleType
	Attackers     []*models.Piece
	Defenders     []*models.Piece
	AttackerID    string // Player name
	DefenderID    string // Player name
	Round         int
	AttackingPieceIDs []int // Track which pieces are attackers
}

// Hit represents a successful hit in combat
type Hit struct {
	Roll      int
	Threshold int
	UnitType  string
}

// BattleResult contains the outcome of a battle
type BattleResult struct {
	AttackerWins         bool
	DefenderWins         bool
	AttackerCasualties   []*models.Piece
	DefenderCasualties   []*models.Piece
	AttackersRemaining   []*models.Piece
	DefendersRemaining   []*models.Piece
	Rounds               int
	AttackerRetreated    bool
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

// CombatRound executes one round of combat
func (dr *DiceRoller) CombatRound(battle *Battle) (attackerHits []Hit, defenderHits []Hit) {
	battle.Round++

	// Attackers roll
	attackerHits = dr.RollForUnits(battle.Attackers, true)

	// Defenders roll
	defenderHits = dr.RollForUnits(battle.Defenders, false)

	return attackerHits, defenderHits
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

// ApplyArtillerySupport modifies infantry attack values when supported by artillery
// For every artillery, one infantry gets +1 attack (from 1 to 2)
func ApplyArtillerySupport(units []*models.Piece) {
	artilleryCount := 0
	infantryNeedingSupport := make([]*models.Piece, 0)

	// Count artillery and infantry
	for _, unit := range units {
		if unit.Name == "artillery" {
			artilleryCount++
		} else if unit.Name == "infantry" && unit.Attack == 1 {
			infantryNeedingSupport = append(infantryNeedingSupport, unit)
		}
	}

	// Apply support up to the number of artillery
	supportCount := artilleryCount
	if len(infantryNeedingSupport) < supportCount {
		supportCount = len(infantryNeedingSupport)
	}

	for i := 0; i < supportCount; i++ {
		infantryNeedingSupport[i].Attack = 2
	}
}

// RemoveArtillerySupport resets infantry attack values
func RemoveArtillerySupport(units []*models.Piece) {
	for _, unit := range units {
		if unit.Name == "infantry" && unit.Attack == 2 {
			unit.Attack = 1
		}
	}
}

// SelectCasualties selects which units to remove as casualties
// Basic implementation: prefer low-cost units
func SelectCasualties(units []*models.Piece, hitCount int) []*models.Piece {
	if hitCount <= 0 || len(units) == 0 {
		return []*models.Piece{}
	}

	// Can't take more casualties than units available
	if hitCount > len(units) {
		hitCount = len(units)
	}

	// Sort units by cost (ascending) to prefer removing cheap units
	sortedUnits := make([]*models.Piece, len(units))
	copy(sortedUnits, units)

	// Simple bubble sort by cost
	for i := 0; i < len(sortedUnits); i++ {
		for j := i + 1; j < len(sortedUnits); j++ {
			if sortedUnits[i].Cost > sortedUnits[j].Cost {
				sortedUnits[i], sortedUnits[j] = sortedUnits[j], sortedUnits[i]
			}
		}
	}

	// Take the first hitCount units (lowest cost)
	casualties := make([]*models.Piece, hitCount)
	copy(casualties, sortedUnits[:hitCount])

	return casualties
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

// ResolveCombat executes a complete battle until one side wins or retreats
func ResolveCombat(battle *Battle, diceRoller *DiceRoller, maxRounds int) (*BattleResult, error) {
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

	// AAA FIRE PHASE (before combat, only happens once)
	// Find AAA units among defenders
	aaaUnits := make([]*models.Piece, 0)
	airUnits := make([]*models.Piece, 0)

	for _, unit := range defenders {
		if unit.Name == "AAA" {
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

	// ARTILLERY SUPPORT (applied before combat begins)
	ApplyArtillerySupport(attackers)

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

		// Execute one combat round
		battle.Attackers = attackers
		battle.Defenders = defenders
		attackerHits, defenderHits := diceRoller.CombatRound(battle)

		// Select and remove casualties
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

	// Remove artillery support before returning
	RemoveArtillerySupport(attackers)

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
