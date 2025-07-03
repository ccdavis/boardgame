package parsing

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type GameState struct {
	Players     []string                       `json:"players"`
	Turn        int                            `json:"turn"`
	Territories map[string]map[string]string   `json:"territories"`
	GameMap     map[string][]string            `json:"game_map"`
	Units       map[string]map[string]int      `json:"units"`
	Containers  map[string]map[string]int      `json:"containers"`
	Placement   map[string]map[string]int      `json:"placement"`
}

func NewGameState() *GameState {
	return &GameState{
		Players:     make([]string, 0),
		Territories: make(map[string]map[string]string),
		GameMap:     make(map[string][]string),
		Units:       make(map[string]map[string]int),
		Containers:  make(map[string]map[string]int),
		Placement:   make(map[string]map[string]int),
	}
}

func (gs *GameState) AsJSON() (string, error) {
	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type GameParser struct {
	*Parser
}

func NewGameParser(r io.Reader) *GameParser {
	return &GameParser{
		Parser: NewParser(r),
	}
}

func (gp *GameParser) Load() (*GameState, error) {
	gs := NewGameState()

	// Parse sections in order
	if err := gp.ParsePlayers(gs); err != nil {
		return nil, err
	}
	if err := gp.ParseTurn(gs); err != nil {
		return nil, err
	}
	if err := gp.ParseTerritories(gs); err != nil {
		return nil, err
	}
	if err := gp.ParseMap(gs); err != nil {
		return nil, err
	}
	if err := gp.ParseUnits(gs); err != nil {
		return nil, err
	}
	if err := gp.ParseContainers(gs); err != nil {
		return nil, err
	}
	if err := gp.ParsePlacement(gs); err != nil {
		return nil, err
	}

	return gs, nil
}

func (gp *GameParser) ParsePlayers(gs *GameState) error {
	if err := gp.Match(PLAYERS); err != nil {
		return err
	}

	for gp.NextToken().Type == IDENTIFIER && gp.NextToken().Type != END_OF_FILE {
		gp.Skip()
		gs.Players = append(gs.Players, gp.LastTokenAsString())

		if gp.NextToken().Type == COMMA {
			gp.Skip()
		}
	}

	return gp.Match(SEMICOLON)
}

func (gp *GameParser) ParseTurn(gs *GameState) error {
	if err := gp.Match(TURN); err != nil {
		return err
	}
	if err := gp.Match(INTEGER); err != nil {
		return err
	}
	gs.Turn = int(gp.LastTokenAsInteger())
	return gp.Match(SEMICOLON)
}

func (gp *GameParser) ParseTerritories(gs *GameState) error {
	if err := gp.Match(TERRITORIES); err != nil {
		return err
	}

	for {
		// Check if we've reached the next section
		if gp.NextToken().Type == MAP || gp.NextToken().Type == END_OF_FILE {
			break
		}
		
		// Try to parse a territory name
		name := gp.tryParseTerritoryName()
		if name == "" {
			// No more territories
			break
		}

		if err := gp.Match(COLON); err != nil {
			return err
		}

		territory := make(map[string]string)

		if err := gp.Match(IDENTIFIER); err != nil {
			return err
		}
		territory["type"] = gp.LastTokenAsString()

		if err := gp.Match(COMMA); err != nil {
			return err
		}

		if err := gp.Match(IDENTIFIER); err != nil {
			return err
		}
		territory["owner"] = gp.LastTokenAsString()

		if err := gp.Match(COMMA); err != nil {
			return err
		}

		if err := gp.Match(INTEGER); err != nil {
			return err
		}
		territory["production"] = fmt.Sprintf("%d", gp.LastTokenAsInteger())

		if err := gp.Match(SEMICOLON); err != nil {
			return err
		}

		gs.Territories[name] = territory
	}

	return nil
}

func (gp *GameParser) tryParseTerritoryName() string {
	var parts []string

	for gp.NextToken().Type == IDENTIFIER && gp.NextToken().Type != END_OF_FILE {
		gp.Skip()
		parts = append(parts, gp.LastTokenAsString())
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " ")
}

func (gp *GameParser) parseTerritoryName() (string, error) {
	name := gp.tryParseTerritoryName()
	if name == "" {
		return "", &ParseError{
			Line:    gp.NextToken().Line,
			Message: "expected territory name",
		}
	}
	return name, nil
}

func (gp *GameParser) ParseMap(gs *GameState) error {
	if err := gp.Match(MAP); err != nil {
		return err
	}

	for {
		// Check if we've reached the next section
		if gp.NextToken().Type == UNITS || gp.NextToken().Type == END_OF_FILE {
			break
		}
		
		// Try to parse territory name
		name := gp.tryParseTerritoryName()
		if name == "" {
			break
		}

		if err := gp.Match(COLON); err != nil {
			return err
		}

		adjacents := make([]string, 0)

		for gp.NextToken().Type != SEMICOLON && gp.NextToken().Type != END_OF_FILE {
			adjName := gp.tryParseTerritoryName()
			if adjName == "" {
				// Check what token we have
				tok := gp.NextToken()
				if tok.Type == SEMICOLON {
					// Empty adjacency or we're done
					break
				} else if tok.Type == END_OF_FILE || tok.Type == UNITS {
					// Unexpected end
					return &ParseError{
						Line:    tok.Line,
						Message: fmt.Sprintf("expected territory name or semicolon, got %s", tok.String()),
					}
				} else {
					// Skip unexpected token
					gp.Skip()
				}
			} else {
				adjacents = append(adjacents, adjName)

				if gp.NextToken().Type == COMMA {
					gp.Skip()
				}
			}
		}

		if err := gp.Match(SEMICOLON); err != nil {
			return err
		}

		gs.GameMap[name] = adjacents
	}

	return nil
}

func (gp *GameParser) ParseUnits(gs *GameState) error {
	if err := gp.Match(UNITS); err != nil {
		return err
	}

	for {
		// Check if we've reached the next section
		if gp.NextToken().Type == CONTAINERS || gp.NextToken().Type == END_OF_FILE {
			break
		}
		
		if gp.NextToken().Type != IDENTIFIER {
			break
		}
		gp.Skip()
		unitType := gp.LastTokenAsString()

		if err := gp.Match(COLON); err != nil {
			return err
		}

		unit := make(map[string]int)

		if err := gp.Match(IDENTIFIER); err != nil {
			return err
		}
		terrain := gp.LastTokenAsString()
		// Store terrain type as a special attribute
		if terrain == "land" {
			unit["land"] = 1
		} else if terrain == "water" {
			unit["water"] = 1
		} else if terrain == "both" {
			unit["both"] = 1
		} else if terrain == "air" {
			unit["air"] = 1
		}

		if err := gp.Match(COMMA); err != nil {
			return err
		}

		for gp.NextToken().Type == INTEGER && gp.NextToken().Type != END_OF_FILE {
			gp.Skip()
			value := int(gp.LastTokenAsInteger())

			if err := gp.Match(IDENTIFIER); err != nil {
				return err
			}
			attr := gp.LastTokenAsString()

			unit[attr] = value

			if gp.NextToken().Type == COMMA {
				gp.Skip()
			}
		}

		if err := gp.Match(SEMICOLON); err != nil {
			return err
		}

		gs.Units[unitType] = unit
	}

	return nil
}

func (gp *GameParser) ParseContainers(gs *GameState) error {
	if err := gp.Match(CONTAINERS); err != nil {
		return err
	}

	for {
		// Check if we've reached the next section
		if gp.NextToken().Type == PLACEMENT || gp.NextToken().Type == END_OF_FILE {
			break
		}
		
		if gp.NextToken().Type != IDENTIFIER {
			break
		}
		gp.Skip()
		containerType := gp.LastTokenAsString()

		if err := gp.Match(COLON); err != nil {
			return err
		}

		container := make(map[string]int)

		if err := gp.Match(INTEGER); err != nil {
			return err
		}
		capacity := int(gp.LastTokenAsInteger())
		container["capacity"] = capacity

		if gp.NextToken().Type == COMMA {
			gp.Skip()

			idx := 0
			for gp.NextToken().Type == IDENTIFIER && gp.NextToken().Type != END_OF_FILE {
				gp.Skip()
				canCarry := gp.LastTokenAsString()
				container[fmt.Sprintf("carry_%d", idx)] = 1
				_ = canCarry
				idx++

				if gp.NextToken().Type == COMMA {
					gp.Skip()
				}
			}
		}

		if err := gp.Match(SEMICOLON); err != nil {
			return err
		}

		gs.Containers[containerType] = container
	}

	return nil
}

func (gp *GameParser) ParsePlacement(gs *GameState) error {
	if err := gp.Match(PLACEMENT); err != nil {
		return err
	}

	count := 0
	for gp.NextToken().Type != END_OF_FILE {
		count++
		if count > 100 {
			return fmt.Errorf("placement parser exceeded 100 iterations - likely infinite loop")
		}
		
		// Try to parse territory name
		territory := gp.tryParseTerritoryName()
		if territory == "" {
			// No more territory names, we're done
			break
		}
		
		if err := gp.Match(COLON); err != nil {
			return err
		}

		placement := make(map[string]int)

		for gp.NextToken().Type != SEMICOLON {
			if gp.NextToken().Type == END_OF_FILE {
				return &ParseError{
					Line:    gp.NextToken().Line,
					Message: "unexpected end of file in placement section",
				}
			}
			
			if err := gp.Match(INTEGER); err != nil {
				return err
			}
			count := int(gp.LastTokenAsInteger())

			if err := gp.Match(IDENTIFIER); err != nil {
				return err
			}
			unitType := gp.LastTokenAsString()

			placement[unitType] = count

			if gp.NextToken().Type == COMMA {
				gp.Skip()
			}
		}

		if err := gp.Match(SEMICOLON); err != nil {
			return err
		}

		gs.Placement[territory] = placement
	}

	return nil
}