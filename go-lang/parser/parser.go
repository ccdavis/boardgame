package parser

import (
	"boardgame/models"
	"boardgame/scanner"
	"fmt"
	"os"
	"strings"
)

type Parser struct {
	scanner   *scanner.Scanner
	lookahead *scanner.Token
	game      *models.Game
}

func NewParser(filename string) (*Parser, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}

	s := scanner.NewScanner(file)
	p := &Parser{
		scanner: s,
		game:    models.NewGame(),
	}

	// Prime the parser with the first token
	p.lookahead = p.scanner.NextToken()

	return p, nil
}

func (p *Parser) match(tokenType scanner.TokenType) error {
	if p.lookahead.Type != tokenType {
		return fmt.Errorf("line %d: expected %s but got %s",
			p.lookahead.Line, tokenType, p.lookahead.Type)
	}
	p.lookahead = p.scanner.NextToken()
	return nil
}

func (p *Parser) peek() scanner.TokenType {
	return p.lookahead.Type
}

func (p *Parser) error(msg string) error {
	return fmt.Errorf("line %d: %s", p.lookahead.Line, msg)
}

// parseTerritoryName reads multiple identifier tokens and joins them with spaces
// until it hits a colon or comma. This allows territory names to have multiple words.
func (p *Parser) parseTerritoryName() (string, error) {
	var parts []string

	for p.peek() == scanner.IDENTIFIER {
		parts = append(parts, p.lookahead.Content)
		p.match(scanner.IDENTIFIER)
	}

	if len(parts) == 0 {
		return "", p.error("expected territory name")
	}

	return strings.Join(parts, " "), nil
}

// Parse parses the entire game definition file
func (p *Parser) Parse() (*models.Game, error) {
	for p.peek() != scanner.EOF {
		switch p.peek() {
		case scanner.PLAYERS:
			if err := p.parsePlayers(); err != nil {
				return nil, err
			}
		case scanner.TURN:
			if err := p.parseTurn(); err != nil {
				return nil, err
			}
		case scanner.TERRITORIES:
			if err := p.parseTerritories(); err != nil {
				return nil, err
			}
		case scanner.MAP:
			if err := p.parseMap(); err != nil {
				return nil, err
			}
		case scanner.UNITS:
			if err := p.parseUnits(); err != nil {
				return nil, err
			}
		case scanner.CONTAINERS:
			if err := p.parseContainers(); err != nil {
				return nil, err
			}
		case scanner.PLACEMENT:
			if err := p.parsePlacement(); err != nil {
				return nil, err
			}
		default:
			return nil, p.error(fmt.Sprintf("unexpected token: %s", p.lookahead.Type))
		}
	}

	return p.game, nil
}

// parsePlayers parses the Players section
// Format: Players name1, name2, name3;
func (p *Parser) parsePlayers() error {
	if err := p.match(scanner.PLAYERS); err != nil {
		return err
	}

	for p.peek() != scanner.SEMICOLON && p.peek() != scanner.EOF {
		if p.peek() != scanner.IDENTIFIER {
			return p.error("expected player name")
		}

		playerName := p.lookahead.Content
		p.game.PlayerOrder = append(p.game.PlayerOrder, playerName)
		p.game.GetOrCreatePlayer(playerName)
		p.match(scanner.IDENTIFIER)

		if p.peek() == scanner.COMMA {
			p.match(scanner.COMMA)
		}
	}

	if err := p.match(scanner.SEMICOLON); err != nil {
		return err
	}

	return nil
}

// parseTurn parses the Turn section
// Format: Turn 1;
func (p *Parser) parseTurn() error {
	if err := p.match(scanner.TURN); err != nil {
		return err
	}

	if p.peek() != scanner.INTEGER {
		return p.error("expected turn number")
	}

	p.game.Turn = int(p.lookahead.IntVal)
	p.match(scanner.INTEGER)

	if err := p.match(scanner.SEMICOLON); err != nil {
		return err
	}

	return nil
}

// parseTerritories parses the Territories section
// Format: Territories
//   Name :type, owner, production;
func (p *Parser) parseTerritories() error {
	if err := p.match(scanner.TERRITORIES); err != nil {
		return err
	}

	for p.peek() != scanner.MAP && p.peek() != scanner.EOF {
		if p.peek() != scanner.IDENTIFIER {
			break
		}

		// Territory name (can have multiple words)
		territoryName, err := p.parseTerritoryName()
		if err != nil {
			return err
		}

		if err := p.match(scanner.COLON); err != nil {
			return err
		}

		// Terrain type
		if p.peek() != scanner.IDENTIFIER {
			return p.error("expected terrain type")
		}
		terrainStr := p.lookahead.Content
		terrain, terr_err := models.ParseTerrainType(terrainStr)
		if terr_err != nil {
			return p.error(fmt.Sprintf("invalid terrain type: %s", terrainStr))
		}
		p.match(scanner.IDENTIFIER)

		if err := p.match(scanner.COMMA); err != nil {
			return err
		}

		// Owner
		if p.peek() != scanner.IDENTIFIER {
			return p.error("expected owner name")
		}
		owner := p.lookahead.Content
		p.match(scanner.IDENTIFIER)

		if err := p.match(scanner.COMMA); err != nil {
			return err
		}

		// Production value
		if p.peek() != scanner.INTEGER {
			return p.error("expected production value")
		}
		production := int(p.lookahead.IntVal)
		p.match(scanner.INTEGER)

		if err := p.match(scanner.SEMICOLON); err != nil {
			return err
		}

		// Add territory to game
		if err := p.game.AddTerritory(territoryName, terrain, owner, production); err != nil {
			return p.error(err.Error())
		}
	}

	return nil
}

// parseMap parses the Map section
// Format: Map
//   Territory: connected1, connected2;
func (p *Parser) parseMap() error {
	if err := p.match(scanner.MAP); err != nil {
		return err
	}

	for p.peek() != scanner.UNITS && p.peek() != scanner.EOF {
		if p.peek() != scanner.IDENTIFIER {
			break
		}

		// Territory name (can have multiple words)
		fromTerritory, err := p.parseTerritoryName()
		if err != nil {
			return err
		}

		if err := p.match(scanner.COLON); err != nil {
			return err
		}

		// List of connected territories
		for p.peek() != scanner.SEMICOLON && p.peek() != scanner.EOF {
			toTerritory, err := p.parseTerritoryName()
			if err != nil {
				return err
			}

			// Add connection
			if err := p.game.ConnectTerritories(fromTerritory, toTerritory); err != nil {
				return p.error(err.Error())
			}

			if p.peek() == scanner.COMMA {
				p.match(scanner.COMMA)
			}
		}

		if err := p.match(scanner.SEMICOLON); err != nil {
			return err
		}
	}

	return nil
}

// parseUnits parses the Units section
// Format: Units
//   name: type, movement movement, attack attack, defend defend, cost cost;
func (p *Parser) parseUnits() error {
	if err := p.match(scanner.UNITS); err != nil {
		return err
	}

	for p.peek() != scanner.CONTAINERS && p.peek() != scanner.PLACEMENT &&
		p.peek() != scanner.EOF {

		if p.peek() != scanner.IDENTIFIER {
			break
		}

		// Unit name
		unitName := p.lookahead.Content
		p.match(scanner.IDENTIFIER)

		if err := p.match(scanner.COLON); err != nil {
			return err
		}

		// Parse attributes: type, movement, attack, defend, cost
		var terrainType models.TerrainType
		var movement, attack, defend, cost int

		// Terrain type
		if p.peek() != scanner.IDENTIFIER {
			return p.error("expected unit terrain type")
		}
		terrainStr := p.lookahead.Content
		var err error
		terrainType, err = models.ParseTerrainType(terrainStr)
		if err != nil {
			return p.error(fmt.Sprintf("invalid terrain type: %s", terrainStr))
		}
		p.match(scanner.IDENTIFIER)

		if err := p.match(scanner.COMMA); err != nil {
			return err
		}

		// Parse attributes in any order
		for p.peek() != scanner.SEMICOLON && p.peek() != scanner.EOF {
			if p.peek() == scanner.INTEGER {
				// Number followed by attribute name
				value := int(p.lookahead.IntVal)
				p.match(scanner.INTEGER)

				if p.peek() != scanner.IDENTIFIER {
					return p.error("expected attribute name after number")
				}

				attrName := strings.ToLower(p.lookahead.Content)
				p.match(scanner.IDENTIFIER)

				switch attrName {
				case "movement":
					movement = value
				case "attack":
					attack = value
				case "defend":
					defend = value
				case "cost":
					cost = value
				default:
					return p.error(fmt.Sprintf("unknown attribute: %s", attrName))
				}

				if p.peek() == scanner.COMMA {
					p.match(scanner.COMMA)
				}
			} else {
				return p.error("expected integer for attribute value")
			}
		}

		if err := p.match(scanner.SEMICOLON); err != nil {
			return err
		}

		// Add unit template to game
		p.game.AddPieceTemplate(unitName, terrainType, movement, attack, defend, cost)
	}

	return nil
}

// parseContainers parses the Containers section
// Format: Containers
//   name: capacity, type1, type2;
func (p *Parser) parseContainers() error {
	if err := p.match(scanner.CONTAINERS); err != nil {
		return err
	}

	for p.peek() != scanner.PLACEMENT && p.peek() != scanner.EOF {
		if p.peek() != scanner.IDENTIFIER {
			break
		}

		// Container name
		containerName := p.lookahead.Content
		p.match(scanner.IDENTIFIER)

		if err := p.match(scanner.COLON); err != nil {
			return err
		}

		// Capacity
		if p.peek() != scanner.INTEGER {
			return p.error("expected container capacity")
		}
		capacity := int(p.lookahead.IntVal)
		p.match(scanner.INTEGER)

		if err := p.match(scanner.COMMA); err != nil {
			return err
		}

		// List of unit types this container can carry
		var canCarry []string
		for p.peek() != scanner.SEMICOLON && p.peek() != scanner.EOF {
			if p.peek() != scanner.IDENTIFIER {
				return p.error("expected unit type name")
			}

			canCarry = append(canCarry, p.lookahead.Content)
			p.match(scanner.IDENTIFIER)

			if p.peek() == scanner.COMMA {
				p.match(scanner.COMMA)
			}
		}

		if err := p.match(scanner.SEMICOLON); err != nil {
			return err
		}

		// Set container capacity
		if err := p.game.SetContainerCapacity(containerName, capacity, canCarry); err != nil {
			return p.error(err.Error())
		}
	}

	return nil
}

// parsePlacement parses the Placement section
// Format: Placement
//   Territory: count type, count type;
func (p *Parser) parsePlacement() error {
	if err := p.match(scanner.PLACEMENT); err != nil {
		return err
	}

	for p.peek() != scanner.EOF {
		if p.peek() != scanner.IDENTIFIER {
			break
		}

		// Territory name (can have multiple words)
		territoryName, err := p.parseTerritoryName()
		if err != nil {
			return err
		}

		if err := p.match(scanner.COLON); err != nil {
			return err
		}

		// List of unit placements: count unitType
		for p.peek() != scanner.SEMICOLON && p.peek() != scanner.EOF {
			if p.peek() != scanner.INTEGER {
				return p.error("expected unit count")
			}
			count := int(p.lookahead.IntVal)
			p.match(scanner.INTEGER)

			if p.peek() != scanner.IDENTIFIER {
				return p.error("expected unit type")
			}
			unitType := p.lookahead.Content
			p.match(scanner.IDENTIFIER)

			// Place units on territory
			if err := p.game.PlacePieces(territoryName, unitType, count); err != nil {
				return p.error(err.Error())
			}

			if p.peek() == scanner.COMMA {
				p.match(scanner.COMMA)
			}
		}

		if err := p.match(scanner.SEMICOLON); err != nil {
			return err
		}
	}

	return nil
}
