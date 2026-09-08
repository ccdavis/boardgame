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

// NeutralType represents the type of neutral territory
type NeutralType int

const (
	NotNeutral NeutralType = iota  // Territory is owned by an active power
	StrictNeutral                   // Cannot be attacked; attacking any strict neutral makes all hostile
	ProAlliedNeutral                // Can be peacefully activated by Allied powers during noncombat
	ProAxisNeutral                  // Can be peacefully activated by Axis powers during noncombat
)

func (n NeutralType) String() string {
	switch n {
	case NotNeutral:
		return "not_neutral"
	case StrictNeutral:
		return "strict_neutral"
	case ProAlliedNeutral:
		return "pro_allied"
	case ProAxisNeutral:
		return "pro_axis"
	default:
		return "unknown"
	}
}

// ParseNeutralType accepts the spellings a .gdf may use.
//
// The single-word forms ("proallied", "proaxis") are the ones the board file
// uses: they survive any tokeniser, whereas the underscore and hyphen forms
// depend on those characters being valid inside an identifier.
func ParseNeutralType(s string) (NeutralType, error) {
	switch s {
	case "strict", "strict_neutral", "strictneutral":
		return StrictNeutral, nil
	case "proallied", "pro_allied", "pro-allied":
		return ProAlliedNeutral, nil
	case "proaxis", "pro_axis", "pro-axis":
		return ProAxisNeutral, nil
	case "not_neutral", "notneutral", "none", "":
		return NotNeutral, nil
	default:
		return NotNeutral, fmt.Errorf(
			"unknown neutrality %q (want strict, proallied or proaxis)", s)
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
	// Earmark is the industrial complex the unit was bought at, when the
	// buyer said. Mobilisation prefers to place a unit where it was bought,
	// so a player who buys at two factories gets each stack in the right
	// place with one click. Advisory: placement is still checked.
	Earmark string
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

	// ID is the key this piece is stored under in Game.Pieces. Carrying it on
	// the piece removes the need to scan the whole map by pointer identity just
	// to answer "which piece is this?".
	ID int

	// Owner is the power the piece belongs to.
	//
	// Ownership used to be inferred from whichever territory a piece happened to
	// be sitting in, which is wrong the moment a piece is in transit, in a
	// captured territory, or aboard a transport -- and it is why movement
	// validation could not tell your units from the enemy's.
	Owner *Player `json:"-"`
}

// Player represents a player in the game
type Player struct {
	Name           string
	NPC            bool
	IPCs           int // Industrial Production Certificates (money)
	Territories    []*Territory
	PieceTemplates map[string]*Piece
	Capital        string // Name of capital territory
	Side           string // "Axis" or "Allies"

	// TakesTurns distinguishes a playable power from a bookkeeping entry.
	//
	// The Players line in a .gdf doubles as the turn order, and aaa.gdf lists
	// "Neutral" there so unclaimed territories have an owner. Without this flag
	// Neutral takes a full turn of its own: purchasing units and collecting
	// income from a dozen territories. It is set for any power named in the
	// Sides section.
	TakesTurns bool
}

// Territory represents a location on the game board
type Territory struct {
	Name          string
	Owner         *Player
	Pieces        []int // piece IDs
	Terrain       TerrainType
	Production    int
	ConnectedTo   []*Territory
	IsVictoryCity bool        // Whether this territory is a victory city
	ICDamage      int         // Industrial Complex damage (reduces production capacity)
	NeutralType   NeutralType // Type of neutral territory (if Owner is "Neutral")

	// OriginalOwner is the power the territory belongs to by right: its owner
	// on the printed board, or -- for a neutral country -- the first power to
	// bring it into the war. Liberation is judged against it: an ally who
	// retakes the territory hands it back rather than keeping it.
	OriginalOwner *Player
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

	// VictoryCitiesEnabled turns the victory-city win condition on and off at
	// play time. The cities themselves are always present in the board data;
	// this only controls whether holding them ends the game.
	VictoryCitiesEnabled bool

	// VictoryHoldSide and VictoryHoldRounds track sustained victory: the side
	// currently at or above its city threshold (Axis 9, Allies 10) and how
	// many consecutive round boundaries it has held it. Two boundaries means
	// the threshold was held for a full round of play, which wins the game.
	VictoryHoldSide   string
	VictoryHoldRounds int

	// unitRegistry is derived from GlobalPieceTemplates on first use.
	unitRegistry *UnitRegistry
}

// TurnTakingPowers returns the powers that actually play, in turn order.
func (g *Game) TurnTakingPowers() []string {
	out := make([]string, 0, len(g.PlayerOrder))
	for _, name := range g.PlayerOrder {
		if player, ok := g.Players[name]; ok && player.TakesTurns {
			out = append(out, name)
		}
	}
	return out
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

	// Neutral-owned land defaults to strict neutrality until the board's
	// Neutrality section says otherwise; water is freely traversable whoever
	// nominally owns it. There used to be a table of hardcoded territory names
	// here (Syria strict, unknown neutrals pro-Allied) that contradicted the
	// data the parser then wrote over it -- two sources of truth, one wrong.
	neutralType := NotNeutral
	if ownerName == "Neutral" && terrain != Water {
		neutralType = StrictNeutral
	}

	territory := &Territory{
		Name:          name,
		Owner:         owner,
		OriginalOwner: owner,
		Pieces:        make([]int, 0),
		Terrain:       terrain,
		Production:    production,
		ConnectedTo:   make([]*Territory, 0),
		IsVictoryCity: false,
		ICDamage:      0,
		NeutralType:   neutralType,
	}

	g.Board[name] = territory
	owner.Territories = append(owner.Territories, territory)
	return nil
}

// ConnectTerritories creates a connection between two territories.
//
// Adjacency is symmetric: both directions are recorded. The .gdf Map section
// normally declares each edge from both endpoints, but it is not required to,
// and several edges in aaa.gdf were only ever declared from one side. Recording
// just from->to made those edges one-way in the engine, so (for example) a fleet
// in Madagascar Sea could never invade Madagascar.
func (g *Game) ConnectTerritories(from, to string) error {
	fromTerr, exists := g.Board[from]
	if !exists {
		return fmt.Errorf("territory %s not found", from)
	}

	toTerr, exists := g.Board[to]
	if !exists {
		return fmt.Errorf("territory %s not found", to)
	}

	connect(fromTerr, toTerr)
	connect(toTerr, fromTerr)
	return nil
}

// connect appends dst to src's adjacency list if it is not already present.
func connect(src, dst *Territory) {
	for _, t := range src.ConnectedTo {
		if t == dst {
			return
		}
	}
	src.ConnectedTo = append(src.ConnectedTo, dst)
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
			Owner:    territory.Owner,
		}
		copy(piece.CanCarry, template.CanCarry)

		pieceID := g.NextPieceID
		g.NextPieceID++
		piece.ID = pieceID
		g.Pieces[pieceID] = piece
		territory.Pieces = append(territory.Pieces, pieceID)
	}

	return nil
}
