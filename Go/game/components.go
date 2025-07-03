package game

import (
	"fmt"
	"strconv"

	"github.com/boardgame/Go/parsing"
)

type TerrainType int

const (
	Land TerrainType = iota
	Water
	Both
)

func ToTerrainType(t string) (TerrainType, error) {
	switch t {
	case "land":
		return Land, nil
	case "water":
		return Water, nil
	case "both":
		return Both, nil
	default:
		return Land, fmt.Errorf("unknown terrain type: %s", t)
	}
}

type PieceID int

type Piece struct {
	Capacity  int16
	Cost      int16
	Attack    int16
	Defend    int16
	Movement  int16
	Terrain   TerrainType
	Name      string
	CanCarry  []string
	Holding   []PieceID
}

type Player struct {
	Name           string
	NPC            bool
	Active         bool
	Territories    []*Territory
	PieceTemplates map[string]*Piece
}

type Territory struct {
	Name        string
	Owner       *Player
	Pieces      []PieceID
	Terrain     TerrainType
	Production  int
	ConnectedTo []*Territory
}

func NewTerritory(owner *Player) *Territory {
	return &Territory{
		Owner:       owner,
		Pieces:      make([]PieceID, 0),
		ConnectedTo: make([]*Territory, 0),
	}
}

type Game struct {
	Board                []*Territory
	Players              []*Player
	Pieces               map[PieceID]*Piece
	GlobalPieceTemplates map[string]*Piece
}

func NewGame(loadedGame *parsing.GameState) (*Game, error) {
	game := &Game{
		Board:                make([]*Territory, 0),
		Players:              make([]*Player, 0),
		Pieces:               make(map[PieceID]*Piece),
		GlobalPieceTemplates: make(map[string]*Piece),
	}

	playersByName := make(map[string]*Player)
	territoryByName := make(map[string]*Territory)

	for _, playerName := range loadedGame.Players {
		player := &Player{
			Name:           playerName,
			Territories:    make([]*Territory, 0),
			PieceTemplates: make(map[string]*Piece),
		}
		game.Players = append(game.Players, player)
		playersByName[playerName] = player
	}

	for name, attributes := range loadedGame.Territories {
		ownerName := attributes["owner"]
		player, ok := playersByName[ownerName]
		if !ok {
			return nil, fmt.Errorf("could not find player named %s to assign to territory %s", ownerName, name)
		}

		territory := NewTerritory(player)
		territory.Name = name

		terrainType, err := ToTerrainType(attributes["type"])
		if err != nil {
			return nil, err
		}
		territory.Terrain = terrainType

		production, err := strconv.Atoi(attributes["production"])
		if err != nil {
			return nil, fmt.Errorf("invalid production value for territory %s: %v", name, err)
		}
		territory.Production = production

		game.Board = append(game.Board, territory)
		territoryByName[name] = territory
		player.Territories = append(player.Territories, territory)
	}

	for _, territory := range game.Board {
		connections, ok := loadedGame.GameMap[territory.Name]
		if !ok {
			return nil, fmt.Errorf("cannot find any connections for territory named %s", territory.Name)
		}

		for _, connectedName := range connections {
			connectedTerritory, ok := territoryByName[connectedName]
			if !ok {
				return nil, fmt.Errorf("cannot connect %s to %s because no territory of exactly this name exists on the board", territory.Name, connectedName)
			}
			territory.ConnectedTo = append(territory.ConnectedTo, connectedTerritory)
		}
	}

	for unitName, attributes := range loadedGame.Units {
		piece := &Piece{
			Name:     unitName,
			Capacity: 0,
			CanCarry: make([]string, 0),
			Holding:  make([]PieceID, 0),
		}

		if cost, ok := attributes["cost"]; ok {
			piece.Cost = int16(cost)
		}
		if attack, ok := attributes["attack"]; ok {
			piece.Attack = int16(attack)
		}
		if defend, ok := attributes["defend"]; ok {
			piece.Defend = int16(defend)
		}
		if movement, ok := attributes["movement"]; ok {
			piece.Movement = int16(movement)
		}

		if _, ok := attributes["water"]; ok {
			piece.Terrain = Water
		} else if _, ok := attributes["land"]; ok {
			piece.Terrain = Land
		} else if _, ok := attributes["both"]; ok {
			piece.Terrain = Both
		} else if _, ok := attributes["air"]; ok {
			piece.Terrain = Both
		} else {
			return nil, fmt.Errorf("no known type of terrain found for piece named %s", piece.Name)
		}

		if container, ok := loadedGame.Containers[piece.Name]; ok {
			for key, value := range container {
				if key == "capacity" {
					piece.Capacity = int16(value)
				} else if len(key) > 6 && key[:6] == "carry_" {
					piece.CanCarry = append(piece.CanCarry, key[6:])
				}
			}
		}

		game.GlobalPieceTemplates[piece.Name] = piece
	}

	for _, player := range game.Players {
		for pieceName, pieceTemplate := range game.GlobalPieceTemplates {
			pieceCopy := *pieceTemplate
			player.PieceTemplates[pieceName] = &pieceCopy
		}
	}

	var pieceID PieceID = 0

	for territoryName, piecesPlaced := range loadedGame.Placement {
		territory, ok := territoryByName[territoryName]
		if !ok {
			return nil, fmt.Errorf("cannot place a piece on territory named %s because no such territory was defined on the map", territoryName)
		}

		for pieceName, quantity := range piecesPlaced {
			pieceTemplate, ok := game.GlobalPieceTemplates[pieceName]
			if !ok {
				return nil, fmt.Errorf("cannot place piece %s because no such piece type was defined", pieceName)
			}

			for i := 0; i < quantity; i++ {
				pieceCopy := *pieceTemplate
				game.Pieces[pieceID] = &pieceCopy
				territory.Pieces = append(territory.Pieces, pieceID)
				pieceID++
			}
		}
	}

	return game, nil
}