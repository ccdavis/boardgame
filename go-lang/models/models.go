package models

import "fmt"

type TerrainType int

const (
	Land TerrainType = iota
	Water
	Air
	Both
)

func (t TerrainType) String() string {
	switch t {
	case Land:
		return "land"
	case Water:
		return "water"
	case Air:
		return "air"
	case Both:
		return "both"
	default:
		return "unknown"
	}
}

func ParseTerrainType(s string) (TerrainType, error) {
	switch s {
	case "land":
		return Land, nil
	case "water":
		return Water, nil
	case "air":
		return Air, nil
	case "both":
		return Both, nil
	default:
		return Land, fmt.Errorf("unknown terrain type: %s", s)
	}
}

// Phase represents the current phase of a player's turn
type Phase int

const (
	PurchasePhase Phase = iota
	CombatMovePhase
	ConductCombatPhase
	NoncombatMovePhase
	MobilizePhase
	CollectIncomePhase
)

func (p Phase) String() string {
	switch p {
	case PurchasePhase:
		return "Purchase Units"
	case CombatMovePhase:
		return "Combat Move"
	case ConductCombatPhase:
		return "Conduct Combat"
	case NoncombatMovePhase:
		return "Noncombat Move"
	case MobilizePhase:
		return "Mobilize New Units"
	case CollectIncomePhase:
		return "Collect Income"
	default:
		return "Unknown"
	}
}

// PendingUnit represents a unit purchased but not yet placed
type PendingUnit struct {
	Type string
	Cost int
}

// Piece represents a game unit (tank, ship, etc.)
type Piece struct {
	Capacity  int16
	Cost      int16
	Attack    int16
	Defend    int16
	Movement  int16
	Terrain   TerrainType
	Name      string
	CanCarry  []string
	Holding   []int // piece IDs
	Hits      int   // Number of hits taken (for multi-hit units like battleships)
}

// Player represents a player in the game
type Player struct {
	Name           string
	NPC            bool
	Active         bool
	IPCs           int // Industrial Production Certificates (money)
	Territories    []*Territory
	PieceTemplates map[string]*Piece
	Capital        string // Name of capital territory
	Side           string // "Axis" or "Allies"
}

// Territory represents a location on the game board
type Territory struct {
	Name        string
	Owner       *Player
	Pieces      []int // piece IDs
	Terrain     TerrainType
	Production  int
	ConnectedTo []*Territory
	IsVictoryCity bool // Whether this territory is a victory city
	ICDamage      int  // Industrial Complex damage (reduces production capacity)
}

// Game is the top-level container for all game state
type Game struct {
	Board                map[string]*Territory
	Players              map[string]*Player
	PlayerOrder          []string
	Pieces               map[int]*Piece
	GlobalPieceTemplates map[string]*Piece
	Turn                 int
	NextPieceID          int

	// Turn state management
	CurrentPower    string                     // Name of player whose turn it is
	CurrentPhase    Phase                      // Current phase of the turn
	PurchasedUnits  map[string][]*PendingUnit  // Player name -> purchased units not yet placed

	// Combat tracking (will be populated in combat phase)
	PendingBattles  []string                   // Territory names where battles will occur
}

// NewGame creates a new Game instance
func NewGame() *Game {
	return &Game{
		Board:                make(map[string]*Territory),
		Players:              make(map[string]*Player),
		PlayerOrder:          make([]string, 0),
		Pieces:               make(map[int]*Piece),
		GlobalPieceTemplates: make(map[string]*Piece),
		Turn:                 1,
		NextPieceID:          1,
		CurrentPower:         "",
		CurrentPhase:         PurchasePhase,
		PurchasedUnits:       make(map[string][]*PendingUnit),
		PendingBattles:       make([]string, 0),
	}
}

// GetOrCreatePlayer returns an existing player or creates a new one
func (g *Game) GetOrCreatePlayer(name string) *Player {
	if p, exists := g.Players[name]; exists {
		return p
	}
	player := &Player{
		Name:           name,
		NPC:            false,
		Active:         true,
		IPCs:           0,
		Territories:    make([]*Territory, 0),
		PieceTemplates: make(map[string]*Piece),
	}
	g.Players[name] = player
	return player
}

// AddTerritory adds a territory to the game
func (g *Game) AddTerritory(name string, terrain TerrainType, ownerName string, production int) error {
	if _, exists := g.Board[name]; exists {
		return fmt.Errorf("territory %s already exists", name)
	}

	owner := g.GetOrCreatePlayer(ownerName)
	territory := &Territory{
		Name:          name,
		Owner:         owner,
		Pieces:        make([]int, 0),
		Terrain:       terrain,
		Production:    production,
		ConnectedTo:   make([]*Territory, 0),
		IsVictoryCity: false,
		ICDamage:      0,
	}

	g.Board[name] = territory
	owner.Territories = append(owner.Territories, territory)
	return nil
}

// ConnectTerritories creates a connection between two territories
func (g *Game) ConnectTerritories(from, to string) error {
	fromTerr, exists := g.Board[from]
	if !exists {
		return fmt.Errorf("territory %s not found", from)
	}

	toTerr, exists := g.Board[to]
	if !exists {
		return fmt.Errorf("territory %s not found", to)
	}

	// Check if connection already exists
	for _, t := range fromTerr.ConnectedTo {
		if t == toTerr {
			return nil // Already connected
		}
	}

	fromTerr.ConnectedTo = append(fromTerr.ConnectedTo, toTerr)
	return nil
}

// AddPieceTemplate adds a piece template to the global templates
func (g *Game) AddPieceTemplate(name string, terrain TerrainType, movement, attack, defend, cost int) {
	piece := &Piece{
		Name:     name,
		Terrain:  terrain,
		Movement: int16(movement),
		Attack:   int16(attack),
		Defend:   int16(defend),
		Cost:     int16(cost),
		CanCarry: make([]string, 0),
		Holding:  make([]int, 0),
	}
	g.GlobalPieceTemplates[name] = piece
}

// SetContainerCapacity sets which piece types a container can carry
func (g *Game) SetContainerCapacity(containerName string, capacity int, canCarry []string) error {
	template, exists := g.GlobalPieceTemplates[containerName]
	if !exists {
		return fmt.Errorf("piece template %s not found", containerName)
	}

	template.Capacity = int16(capacity)
	template.CanCarry = canCarry
	return nil
}

// PlacePieces places pieces on a territory
func (g *Game) PlacePieces(territoryName, pieceType string, count int) error {
	territory, exists := g.Board[territoryName]
	if !exists {
		return fmt.Errorf("territory %s not found", territoryName)
	}

	template, exists := g.GlobalPieceTemplates[pieceType]
	if !exists {
		return fmt.Errorf("piece template %s not found", pieceType)
	}

	// Create count number of pieces
	for i := 0; i < count; i++ {
		// Clone the template
		piece := &Piece{
			Name:     template.Name,
			Terrain:  template.Terrain,
			Movement: template.Movement,
			Attack:   template.Attack,
			Defend:   template.Defend,
			Cost:     template.Cost,
			Capacity: template.Capacity,
			CanCarry: make([]string, len(template.CanCarry)),
			Holding:  make([]int, 0),
		}
		copy(piece.CanCarry, template.CanCarry)

		pieceID := g.NextPieceID
		g.NextPieceID++
		g.Pieces[pieceID] = piece
		territory.Pieces = append(territory.Pieces, pieceID)
	}

	return nil
}
