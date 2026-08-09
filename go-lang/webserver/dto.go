package webserver

import (
	"sort"

	"boardgame/game"
	"boardgame/models"
)

// TerritoryDTO is a JSON-serializable territory representation
type TerritoryDTO struct {
	Name          string   `json:"name"`
	Owner         string   `json:"owner"`
	Terrain       string   `json:"terrain"`
	Production    int      `json:"production"`
	IsVictoryCity bool     `json:"isVictoryCity"`
	ICDamage      int      `json:"icDamage"`
	NeutralType   string   `json:"neutralType"`
	UnitCount     int      `json:"unitCount"`
	ConnectedTo   []string `json:"connectedTo"`

	// HasFactory and FriendlyUnits drive the map's phase highlighting: which
	// territories light up as production sites, and which hold units the human
	// player could move. FriendlyUnits counts the human's pieces here excluding
	// structures, which are captured with the territory rather than moved.
	HasFactory    bool `json:"hasFactory"`
	FriendlyUnits int  `json:"friendlyUnits"`

	// PieceOwner is the power owning (the plurality of) the units here. In sea
	// zones it is the only ownership that matters: the zone itself is Neutral,
	// but the fleet in it belongs to someone, and the map badge shows whom.
	PieceOwner string `json:"pieceOwner,omitempty"`
}

// TerritoryDetailDTO includes unit information
type TerritoryDetailDTO struct {
	TerritoryDTO
	Units                []UnitDTO                     `json:"units"`
	ConnectedTerritories []ConnectedTerritoryDTO       `json:"connectedTerritories"`
}

// ConnectedTerritoryDTO represents a connected territory with context
type ConnectedTerritoryDTO struct {
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	UnitCount int    `json:"unitCount"`
	CanAttack bool   `json:"canAttack"`
	CanMoveTo bool   `json:"canMoveTo"`
}

// UnitDTO is a JSON-serializable unit representation
type UnitDTO struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Attack   int    `json:"attack"`
	Defend   int    `json:"defend"`
	Movement int    `json:"movement"`
	Terrain  string `json:"terrain"`
	CanMove  bool   `json:"canMove"`
	Hits     int    `json:"hits"`
	// Owner matters in shared spaces: a sea zone holds ships from several
	// powers, and the unit picker must offer only the human player's.
	Owner string `json:"owner"`
	// Aboard is the ID of the transport carrying this piece, when it is cargo.
	// Cargo lives in the transport's hold, not the territory's piece list, so
	// without this the browser would never see loaded units at all.
	Aboard int `json:"aboard,omitempty"`
}

// PlayerDTO is a JSON-serializable player representation
type PlayerDTO struct {
	Name           string `json:"name"`
	IPCs           int    `json:"ipcs"`
	Side           string `json:"side"`
	IsHuman        bool   `json:"isHuman"`
	TerritoryCount int    `json:"territoryCount"`
	Capital        string `json:"capital"`
}

// GameStateDTO represents the overall game state
type GameStateDTO struct {
	Turn         int                  `json:"turn"`
	CurrentPower string               `json:"currentPower"`
	CurrentPhase string               `json:"currentPhase"`
	HumanPlayer  string               `json:"humanPlayer"`
	IsHumanTurn  bool                 `json:"isHumanTurn"`
	Players      []PlayerDTO          `json:"players"`
	VictoryCities VictoryCityCount    `json:"victoryCities"`
	// GameOver and Winner report the victory check, so the browser learns the
	// war has been decided from the same poll that carries everything else.
	GameOver bool   `json:"gameOver"`
	Winner   string `json:"winner,omitempty"`
}

// VictoryCityCount represents victory city control
type VictoryCityCount struct {
	Axis   int `json:"axis"`
	Allies int `json:"allies"`
}

// MoveDTO represents a planned move
type MoveDTO struct {
	PieceID int    `json:"pieceId"`
	From    string `json:"from"`
	To      string `json:"to"`
	Type    string `json:"type"` // "combat" or "noncombat"
	// Landing marks a booked amphibious assault rather than an ordinary
	// move: cancelled through cancel-landing, not cancel-move.
	Landing bool `json:"landing,omitempty"`
}

// BattleResultDTO represents the result of a battle
type BattleResultDTO struct {
	Territory           string   `json:"territory"`
	AttackerWins        bool     `json:"attackerWins"`
	DefenderWins        bool     `json:"defenderWins"`
	AttackerRetreated   bool     `json:"attackerRetreated"`
	Rounds              int      `json:"rounds"`
	AttackerCasualties  []string `json:"attackerCasualties"`
	DefenderCasualties  []string `json:"defenderCasualties"`
	TerritoryCaptured   bool     `json:"territoryCaptured"`
}

// UnitGroupDTO is a stack of identical units, as shown on a battle screen.
type UnitGroupDTO struct {
	Type   string `json:"type"`
	Count  int    `json:"count"`
	Attack int    `json:"attack"`
	Defend int    `json:"defend"`
}

// PendingBattleDTO describes a battle waiting to be fought: both rosters, so
// the battle screen can show the two sides face to face before any dice roll.
type PendingBattleDTO struct {
	Territory string         `json:"territory"`
	Attacker  string         `json:"attacker"`
	Defender  string         `json:"defender"`
	Attackers []UnitGroupDTO `json:"attackers"`
	Defenders []UnitGroupDTO `json:"defenders"`
	// Bombarding are the warships standing off shore in support of an
	// amphibious landing -- shown so the player knows the beach is covered.
	Bombarding []UnitGroupDTO `json:"bombarding,omitempty"`
}

// ReachableTerritoryDTO represents a territory a unit can reach
type ReachableTerritoryDTO struct {
	Name      string `json:"name"`
	Distance  int    `json:"distance"`
	Owner     string `json:"owner"`
	IsAttack  bool   `json:"isAttack"`
	UnitCount int    `json:"unitCount"`
	// IsBoard: a sea zone where the selected land units can board transports.
	IsBoard bool `json:"isBoard,omitempty"`
	// IsUnload: a land territory the selected cargo can be unloaded onto.
	IsUnload bool `json:"isUnload,omitempty"`
	// IsCarrier: a sea zone where the selected aircraft can set down on a
	// friendly carrier's deck. Open water is not a destination -- a plane that
	// ends the turn over it is lost -- so the browser confirms before booking
	// the move, and Note says what it will land on.
	IsCarrier bool   `json:"isCarrier,omitempty"`
	Note      string `json:"note,omitempty"`
}

// AvailableUnitDTO represents a unit type that can be purchased
type AvailableUnitDTO struct {
	Type      string `json:"type"`
	Cost      int    `json:"cost"`
	Attack    int    `json:"attack"`
	Defend    int    `json:"defend"`
	Movement  int    `json:"movement"`
	Available bool   `json:"available"`
}

// PurchasedUnitDTO represents a purchased but not yet placed unit
type PurchasedUnitDTO struct {
	Type     string `json:"type"`
	Quantity int    `json:"quantity"`
	// Targets is filled in during the Mobilize phase: the territories where a
	// unit of this type may legally be placed right now.
	Targets []string `json:"targets,omitempty"`
}

// Conversion Functions

// ToTerritoryDTO converts a Territory to a DTO. The game and the human
// player's name are needed to fill the highlighting fields: what counts as a
// factory comes from the unit registry, and "friendly" means the human's.
func ToTerritoryDTO(t *models.Territory, g *models.Game, humanPlayer string) TerritoryDTO {
	connectedNames := make([]string, len(t.ConnectedTo))
	for i, conn := range t.ConnectedTo {
		connectedNames[i] = conn.Name
	}

	units := g.Units()
	hasFactory := false
	friendly := 0
	ownerCounts := make(map[string]int)
	for _, pieceID := range t.Pieces {
		piece := g.Pieces[pieceID]
		if piece == nil {
			continue
		}
		if units.For(piece).IsStructure {
			hasFactory = true
			continue
		}
		if piece.Owner != nil {
			ownerCounts[piece.Owner.Name]++
			if piece.Owner.Name == humanPlayer {
				friendly++
			}
		}
	}
	pieceOwner := ""
	for owner, n := range ownerCounts {
		if pieceOwner == "" || n > ownerCounts[pieceOwner] ||
			(n == ownerCounts[pieceOwner] && owner < pieceOwner) {
			pieceOwner = owner
		}
	}

	return TerritoryDTO{
		Name:          t.Name,
		Owner:         t.Owner.Name,
		Terrain:       t.Terrain.String(),
		Production:    t.Production,
		IsVictoryCity: t.IsVictoryCity,
		ICDamage:      t.ICDamage,
		NeutralType:   t.NeutralType.String(),
		UnitCount:     len(t.Pieces),
		ConnectedTo:   connectedNames,
		HasFactory:    hasFactory,
		FriendlyUnits: friendly,
		PieceOwner:    pieceOwner,
	}
}

// ToUnitDTO converts a Piece to a DTO
func ToUnitDTO(pieceID int, piece *models.Piece, canMove bool) UnitDTO {
	owner := ""
	if piece.Owner != nil {
		owner = piece.Owner.Name
	}
	return UnitDTO{
		ID:       pieceID,
		Name:     piece.Name,
		Attack:   int(piece.Attack),
		Defend:   int(piece.Defend),
		Movement: int(piece.Movement),
		Terrain:  piece.Terrain.String(),
		CanMove:  canMove,
		Hits:     piece.Hits,
		Owner:    owner,
	}
}

// ToPendingBattleDTO summarises a pending battle's rosters by unit type.
//
// The rosters are derived here rather than read from the Battle: Attackers and
// Defenders are only populated when the battle is resolved. The split mirrors
// ResolveBattle's -- pieces on the tracked attacking-ID list attack, structures
// are captured with the territory rather than fought, everything else defends.
func ToPendingBattleDTO(g *models.Game, b *game.Battle) PendingBattleDTO {
	group := func(pieces []*models.Piece) []UnitGroupDTO {
		counts := make(map[string]int)
		stats := make(map[string]*models.Piece)
		for _, piece := range pieces {
			counts[piece.Name]++
			stats[piece.Name] = piece
		}
		names := make([]string, 0, len(counts))
		for name := range counts {
			names = append(names, name)
		}
		sort.Strings(names)
		out := make([]UnitGroupDTO, 0, len(names))
		for _, name := range names {
			out = append(out, UnitGroupDTO{
				Type:   name,
				Count:  counts[name],
				Attack: int(stats[name].Attack),
				Defend: int(stats[name].Defend),
			})
		}
		return out
	}

	attackingIDs := make(map[int]bool, len(b.AttackingPieceIDs))
	for _, id := range b.AttackingPieceIDs {
		attackingIDs[id] = true
	}

	units := g.Units()
	var attackers, defenders []*models.Piece
	if territory, ok := g.Board[b.Location]; ok {
		for _, pieceID := range territory.Pieces {
			piece := g.Pieces[pieceID]
			switch {
			case piece == nil:
				continue
			case attackingIDs[pieceID]:
				attackers = append(attackers, piece)
			case units.For(piece).IsStructure:
				// captured with the territory, not fought over
			default:
				defenders = append(defenders, piece)
			}
		}
	}

	return PendingBattleDTO{
		Territory:  b.Location,
		Attacker:   b.AttackerID,
		Defender:   b.DefenderID,
		Attackers:  group(attackers),
		Defenders:  group(defenders),
		Bombarding: group(b.Bombarding),
	}
}

// ToPlayerDTO converts a Player to a DTO
func ToPlayerDTO(p *models.Player, humanPlayer string) PlayerDTO {
	return PlayerDTO{
		Name:           p.Name,
		IPCs:           p.IPCs,
		Side:           p.Side,
		IsHuman:        p.Name == humanPlayer,
		TerritoryCount: len(p.Territories),
		Capital:        p.Capital,
	}
}

// ToGameStateDTO converts game state to a DTO
func ToGameStateDTO(g *models.Game, humanPlayer string) GameStateDTO {
	players := make([]PlayerDTO, len(g.PlayerOrder))
	for i, playerName := range g.PlayerOrder {
		players[i] = ToPlayerDTO(g.Players[playerName], humanPlayer)
	}

	axisVC, alliesVC := g.CountVictoryCities()

	return GameStateDTO{
		Turn:         g.Turn,
		CurrentPower: g.CurrentPower,
		CurrentPhase: g.CurrentPhase.String(),
		HumanPlayer:  humanPlayer,
		IsHumanTurn:  g.CurrentPower == humanPlayer,
		Players:      players,
		VictoryCities: VictoryCityCount{
			Axis:   axisVC,
			Allies: alliesVC,
		},
	}
}

// ToMoveDTO converts a game Move to a DTO
func ToMoveDTO(m *game.Move) MoveDTO {
	moveType := "noncombat"
	if m.Type == game.CombatMove {
		moveType = "combat"
	}

	return MoveDTO{
		PieceID: m.PieceID,
		From:    m.From,
		To:      m.To,
		Type:    moveType,
	}
}

// ToBattleResultDTO converts a BattleResult to a DTO
func ToBattleResultDTO(territory string, result *game.BattleResult) BattleResultDTO {
	attackerCasualties := make([]string, len(result.AttackerCasualties))
	for i, piece := range result.AttackerCasualties {
		attackerCasualties[i] = piece.Name
	}

	defenderCasualties := make([]string, len(result.DefenderCasualties))
	for i, piece := range result.DefenderCasualties {
		defenderCasualties[i] = piece.Name
	}

	return BattleResultDTO{
		Territory:          territory,
		AttackerWins:       result.AttackerWins,
		DefenderWins:       result.DefenderWins,
		AttackerRetreated:  result.AttackerRetreated,
		Rounds:             result.Rounds,
		AttackerCasualties: attackerCasualties,
		DefenderCasualties: defenderCasualties,
		TerritoryCaptured:  result.AttackerWins && !result.AttackerRetreated,
	}
}

// ToAvailableUnitDTOs creates a list of available units for purchase
func ToAvailableUnitDTOs(templates map[string]*models.Piece, currentIPCs int) []AvailableUnitDTO {
	units := make([]AvailableUnitDTO, 0, len(templates))

	for name, template := range templates {
		units = append(units, AvailableUnitDTO{
			Type:      name,
			Cost:      int(template.Cost),
			Attack:    int(template.Attack),
			Defend:    int(template.Defend),
			Movement:  int(template.Movement),
			Available: currentIPCs >= int(template.Cost),
		})
	}

	// A stable order matters more than which order: this list is re-fetched by
	// the browser's poll every couple of seconds, and Go's map iteration is
	// deliberately random, so without the sort the purchase menu reshuffled
	// its rows under the player's cursor.
	sort.Slice(units, func(i, j int) bool {
		if units[i].Cost != units[j].Cost {
			return units[i].Cost < units[j].Cost
		}
		return units[i].Type < units[j].Type
	})

	return units
}

// GroupPurchasedUnits converts a list of pending units to quantity groups
func GroupPurchasedUnits(units []*models.PendingUnit) []PurchasedUnitDTO {
	counts := make(map[string]int)
	for _, unit := range units {
		counts[unit.Type]++
	}

	types := make([]string, 0, len(counts))
	for unitType := range counts {
		types = append(types, unitType)
	}
	sort.Strings(types)

	result := make([]PurchasedUnitDTO, 0, len(counts))
	for _, unitType := range types {
		result = append(result, PurchasedUnitDTO{
			Type:     unitType,
			Quantity: counts[unitType],
		})
	}

	return result
}
