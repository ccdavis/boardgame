package game

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type SavedPiece struct {
	ID       PieceID     `json:"id"`
	Name     string      `json:"name"`
	Cost     int16       `json:"cost"`
	Attack   int16       `json:"attack"`
	Defend   int16       `json:"defend"`
	Movement int16       `json:"movement"`
	Capacity int16       `json:"capacity"`
	Terrain  TerrainType `json:"terrain"`
	CanCarry []string    `json:"can_carry"`
	Holding  []PieceID   `json:"holding"`
}

type SavedTerritory struct {
	Name        string    `json:"name"`
	OwnerName   string    `json:"owner"`
	Pieces      []PieceID `json:"pieces"`
	Terrain     int       `json:"terrain"`
	Production  int       `json:"production"`
	ConnectedTo []string  `json:"connected_to"`
}

type SavedPlayer struct {
	Name           string                 `json:"name"`
	NPC            bool                   `json:"npc"`
	Active         bool                   `json:"active"`
	Territories    []string               `json:"territories"`
	PieceTemplates map[string]*SavedPiece `json:"piece_templates"`
}

type SavedGame struct {
	Territories          []SavedTerritory       `json:"territories"`
	Players              []SavedPlayer          `json:"players"`
	Pieces               []SavedPiece           `json:"pieces"`
	GlobalPieceTemplates map[string]*SavedPiece `json:"global_piece_templates"`
	CurrentTurn          int                    `json:"current_turn"`
}

func (g *Game) ToSavedGame() *SavedGame {
	saved := &SavedGame{
		Territories:          make([]SavedTerritory, 0, len(g.Board)),
		Players:              make([]SavedPlayer, 0, len(g.Players)),
		Pieces:               make([]SavedPiece, 0, len(g.Pieces)),
		GlobalPieceTemplates: make(map[string]*SavedPiece),
		CurrentTurn:          0,
	}

	for _, territory := range g.Board {
		connectedNames := make([]string, 0, len(territory.ConnectedTo))
		for _, connected := range territory.ConnectedTo {
			connectedNames = append(connectedNames, connected.Name)
		}

		savedTerritory := SavedTerritory{
			Name:        territory.Name,
			OwnerName:   territory.Owner.Name,
			Pieces:      territory.Pieces,
			Terrain:     int(territory.Terrain),
			Production:  territory.Production,
			ConnectedTo: connectedNames,
		}
		saved.Territories = append(saved.Territories, savedTerritory)
	}

	for _, player := range g.Players {
		territoryNames := make([]string, 0, len(player.Territories))
		for _, territory := range player.Territories {
			territoryNames = append(territoryNames, territory.Name)
		}

		pieceTemplates := make(map[string]*SavedPiece)
		for name, piece := range player.PieceTemplates {
			pieceTemplates[name] = pieceToSavedPiece(piece, 0)
		}

		savedPlayer := SavedPlayer{
			Name:           player.Name,
			NPC:            player.NPC,
			Active:         player.Active,
			Territories:    territoryNames,
			PieceTemplates: pieceTemplates,
		}
		saved.Players = append(saved.Players, savedPlayer)
	}

	for id, piece := range g.Pieces {
		saved.Pieces = append(saved.Pieces, *pieceToSavedPiece(piece, id))
	}

	for name, piece := range g.GlobalPieceTemplates {
		saved.GlobalPieceTemplates[name] = pieceToSavedPiece(piece, 0)
	}

	return saved
}

func pieceToSavedPiece(piece *Piece, id PieceID) *SavedPiece {
	return &SavedPiece{
		ID:       id,
		Name:     piece.Name,
		Cost:     piece.Cost,
		Attack:   piece.Attack,
		Defend:   piece.Defend,
		Movement: piece.Movement,
		Capacity: piece.Capacity,
		Terrain:  piece.Terrain,
		CanCarry: piece.CanCarry,
		Holding:  piece.Holding,
	}
}

func FromSavedGame(saved *SavedGame) (*Game, error) {
	game := &Game{
		Board:                make([]*Territory, 0, len(saved.Territories)),
		Players:              make([]*Player, 0, len(saved.Players)),
		Pieces:               make(map[PieceID]*Piece),
		GlobalPieceTemplates: make(map[string]*Piece),
	}

	playersByName := make(map[string]*Player)
	territoryByName := make(map[string]*Territory)

	for _, savedPlayer := range saved.Players {
		player := &Player{
			Name:           savedPlayer.Name,
			NPC:            savedPlayer.NPC,
			Active:         savedPlayer.Active,
			Territories:    make([]*Territory, 0),
			PieceTemplates: make(map[string]*Piece),
		}

		for name, savedPiece := range savedPlayer.PieceTemplates {
			player.PieceTemplates[name] = savedPieceToPiece(savedPiece)
		}

		game.Players = append(game.Players, player)
		playersByName[player.Name] = player
	}

	for _, savedTerritory := range saved.Territories {
		owner, ok := playersByName[savedTerritory.OwnerName]
		if !ok {
			return nil, fmt.Errorf("could not find player named %s for territory %s", savedTerritory.OwnerName, savedTerritory.Name)
		}

		territory := &Territory{
			Name:        savedTerritory.Name,
			Owner:       owner,
			Pieces:      savedTerritory.Pieces,
			Terrain:     TerrainType(savedTerritory.Terrain),
			Production:  savedTerritory.Production,
			ConnectedTo: make([]*Territory, 0),
		}

		game.Board = append(game.Board, territory)
		territoryByName[territory.Name] = territory
		owner.Territories = append(owner.Territories, territory)
	}

	for i, savedTerritory := range saved.Territories {
		territory := game.Board[i]
		for _, connectedName := range savedTerritory.ConnectedTo {
			connectedTerritory, ok := territoryByName[connectedName]
			if !ok {
				return nil, fmt.Errorf("cannot connect %s to %s: territory not found", territory.Name, connectedName)
			}
			territory.ConnectedTo = append(territory.ConnectedTo, connectedTerritory)
		}
	}

	for _, savedPiece := range saved.Pieces {
		game.Pieces[savedPiece.ID] = savedPieceToPiece(&savedPiece)
	}

	for name, savedPiece := range saved.GlobalPieceTemplates {
		game.GlobalPieceTemplates[name] = savedPieceToPiece(savedPiece)
	}

	return game, nil
}

func savedPieceToPiece(saved *SavedPiece) *Piece {
	return &Piece{
		Name:     saved.Name,
		Cost:     saved.Cost,
		Attack:   saved.Attack,
		Defend:   saved.Defend,
		Movement: saved.Movement,
		Capacity: saved.Capacity,
		Terrain:  saved.Terrain,
		CanCarry: saved.CanCarry,
		Holding:  saved.Holding,
	}
}

func (g *Game) SaveToFile(filename string) error {
	saved := g.ToSavedGame()
	
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal game state: %v", err)
	}

	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

func LoadGameFromFile(filename string) (*Game, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var saved SavedGame
	err = json.Unmarshal(data, &saved)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal game state: %v", err)
	}

	return FromSavedGame(&saved)
}

func (g *Game) SaveToJSON() (string, error) {
	saved := g.ToSavedGame()
	
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal game state: %v", err)
	}

	return string(data), nil
}

func LoadGameFromJSON(jsonData string) (*Game, error) {
	var saved SavedGame
	err := json.Unmarshal([]byte(jsonData), &saved)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal game state: %v", err)
	}

	return FromSavedGame(&saved)
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}