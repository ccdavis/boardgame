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

// ReachableTerritoryDTO represents a territory a unit can reach
type ReachableTerritoryDTO struct {
	Name      string `json:"name"`
	Distance  int    `json:"distance"`
	Owner     string `json:"owner"`
	IsAttack  bool   `json:"isAttack"`
	UnitCount int    `json:"unitCount"`
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

// ToTerritoryDTO converts a Territory to a DTO
func ToTerritoryDTO(t *models.Territory) TerritoryDTO {
	connectedNames := make([]string, len(t.ConnectedTo))
	for i, conn := range t.ConnectedTo {
		connectedNames[i] = conn.Name
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
	}
}

// ToUnitDTO converts a Piece to a DTO
func ToUnitDTO(pieceID int, piece *models.Piece, canMove bool) UnitDTO {
	return UnitDTO{
		ID:       pieceID,
		Name:     piece.Name,
		Attack:   int(piece.Attack),
		Defend:   int(piece.Defend),
		Movement: int(piece.Movement),
		Terrain:  piece.Terrain.String(),
		CanMove:  canMove,
		Hits:     piece.Hits,
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
