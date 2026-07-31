package parser

import (
	"boardgame/models"
	"boardgame/scanner"
	"fmt"
	"io"
	"os"
	"strings"
)

type Parser struct {
	scanner   *scanner.Scanner
	lookahead *scanner.Token
	game      *models.Game
}

// NewParser opens a board definition file.
func NewParser(filename string) (*Parser, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	return NewParserFromReader(file), nil
}

// NewParserFromReader parses from any source.
//
// The parser could previously only be constructed from a filename, which meant
// no part of it could be unit-tested without writing a file to disk first --
// and so none of it was tested at all.
func NewParserFromReader(source io.Reader) *Parser {
	p := &Parser{
		scanner: scanner.NewScanner(source),
		game:    models.NewGame(),
	}

	// Prime the parser with the first token
	p.lookahead = p.scanner.NextToken()

	return p
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
// atSectionKeyword reports whether the next token starts a new top-level
// section.
//
// Each section loop used to stop at one specific following keyword --
// parseTerritories stopped at MAP, parseUnits at CONTAINERS or PLACEMENT, and
// so on. That silently assumed a fixed section order and broke the moment a new
// section was added between them. Terminating on any keyword removes the
// assumption, so sections can appear in any order and new ones cost nothing.
func (p *Parser) atSectionKeyword() bool {
	switch p.peek() {
	case scanner.PLAYERS, scanner.TURN, scanner.TERRITORIES, scanner.MAP,
		scanner.UNITS, scanner.CONTAINERS, scanner.PLACEMENT,
		scanner.SIDES, scanner.CAPITALS, scanner.VICTORYCITIES, scanner.NEUTRALITY:
		return true
	}
	return false
}

// atSectionEnd is true at a section boundary or end of input.
func (p *Parser) atSectionEnd() bool {
	return p.atSectionKeyword() || p.peek() == scanner.EOF
}

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
		case scanner.SIDES:
			if err := p.parseSides(); err != nil {
				return nil, err
			}
		case scanner.CAPITALS:
			if err := p.parseCapitals(); err != nil {
				return nil, err
			}
		case scanner.VICTORYCITIES:
			if err := p.parseVictoryCities(); err != nil {
				return nil, err
			}
		case scanner.NEUTRALITY:
			if err := p.parseNeutrality(); err != nil {
				return nil, err
			}
		default:
			return nil, p.error(fmt.Sprintf("unexpected token: %s", p.lookahead.Type))
		}
	}

	// A parsed board must satisfy the same invariants any game does. The checks
	// are self-gating: a board that declares no Sides marks nobody as playing, so
	// the metadata checks stay quiet and only structural defects are reported.
	if problems := p.game.Validate(); len(problems) > 0 {
		detail := ""
		for i, problem := range problems {
			if i == 5 {
				detail += fmt.Sprintf("; and %d more", len(problems)-5)
				break
			}
			if i > 0 {
				detail += "; "
			}
			detail += problem.Detail
		}
		return nil, fmt.Errorf("board is not valid: %s", detail)
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

	for !p.atSectionEnd() {
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

	for !p.atSectionEnd() {
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

	for !p.atSectionEnd() {

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

	for !p.atSectionEnd() {
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

	for !p.atSectionEnd() {
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

// parseSides parses the Sides section.
//
//	Sides
//	  Axis: Germany, Japan, Italy;
//	  Allies: USA, USSR, UK;
//
// Membership here is what makes a power playable: it sets Side (used for
// alliance checks) and TakesTurns. A power absent from every side -- Neutral --
// owns territory but never takes a turn.
func (p *Parser) parseSides() error {
	if err := p.match(scanner.SIDES); err != nil {
		return err
	}

	for !p.atSectionEnd() {
		if p.peek() != scanner.IDENTIFIER {
			break
		}
		side := p.lookahead.Content
		if err := p.match(scanner.IDENTIFIER); err != nil {
			return err
		}
		if err := p.match(scanner.COLON); err != nil {
			return err
		}

		for p.peek() != scanner.SEMICOLON && p.peek() != scanner.EOF {
			if p.peek() != scanner.IDENTIFIER {
				return p.error("expected a power name in the Sides section")
			}
			name := p.lookahead.Content
			p.match(scanner.IDENTIFIER)

			player, ok := p.game.Players[name]
			if !ok {
				return p.error(fmt.Sprintf("side %q names unknown power %q", side, name))
			}
			player.Side = side
			player.TakesTurns = true

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

// parseCapitals parses the Capitals section.
//
//	Capitals
//	  Germany: Germany;  USSR: Russia;
//
// A power's capital is where its treasury sits: capturing it transfers the
// defender's remaining IPCs.
func (p *Parser) parseCapitals() error {
	if err := p.match(scanner.CAPITALS); err != nil {
		return err
	}

	for !p.atSectionEnd() {
		if p.peek() != scanner.IDENTIFIER {
			break
		}
		powerName := p.lookahead.Content
		if err := p.match(scanner.IDENTIFIER); err != nil {
			return err
		}
		if err := p.match(scanner.COLON); err != nil {
			return err
		}

		territoryName, err := p.parseTerritoryName()
		if err != nil {
			return err
		}
		if err := p.match(scanner.SEMICOLON); err != nil {
			return err
		}

		player, ok := p.game.Players[powerName]
		if !ok {
			return p.error(fmt.Sprintf("capital declared for unknown power %q", powerName))
		}
		if _, ok := p.game.Board[territoryName]; !ok {
			return p.error(fmt.Sprintf("capital of %q is unknown territory %q",
				powerName, territoryName))
		}
		player.Capital = territoryName
	}
	return nil
}

// parseVictoryCities parses the VictoryCities section.
//
//	VictoryCities
//	  Germany, Russia, Britain, Japan;
//
// Names are separated by commas and the list ends with a semicolon. Territory
// names may be several words, so a name ends at a comma or semicolon.
func (p *Parser) parseVictoryCities() error {
	if err := p.match(scanner.VICTORYCITIES); err != nil {
		return err
	}

	for !p.atSectionEnd() {
		if p.peek() != scanner.IDENTIFIER {
			break
		}

		for p.peek() != scanner.SEMICOLON && p.peek() != scanner.EOF {
			name, err := p.parseTerritoryName()
			if err != nil {
				return err
			}
			territory, ok := p.game.Board[name]
			if !ok {
				return p.error(fmt.Sprintf("victory city %q is not a territory", name))
			}
			territory.IsVictoryCity = true

			if p.peek() == scanner.COMMA {
				p.match(scanner.COMMA)
			}
		}
		if err := p.match(scanner.SEMICOLON); err != nil {
			return err
		}
	}

	// Present in the data means available; a game may still switch the win
	// condition off.
	p.game.VictoryCitiesEnabled = true
	return nil
}

// parseNeutrality parses the Neutrality section.
//
//	Neutrality
//	  Turkey: strict;  Spain: proaxis;  Colombia: proallied;
//
// The values are single words on purpose. Reserved words are matched without
// regard to case, so a value like "neutral" would collide with the owner name
// used throughout the Territories section.
func (p *Parser) parseNeutrality() error {
	if err := p.match(scanner.NEUTRALITY); err != nil {
		return err
	}

	for !p.atSectionEnd() {
		if p.peek() != scanner.IDENTIFIER {
			break
		}
		territoryName, err := p.parseTerritoryName()
		if err != nil {
			return err
		}
		if err := p.match(scanner.COLON); err != nil {
			return err
		}
		if p.peek() != scanner.IDENTIFIER {
			return p.error("expected a neutrality kind")
		}
		kindStr := p.lookahead.Content
		p.match(scanner.IDENTIFIER)
		if err := p.match(scanner.SEMICOLON); err != nil {
			return err
		}

		territory, ok := p.game.Board[territoryName]
		if !ok {
			return p.error(fmt.Sprintf("neutrality declared for unknown territory %q",
				territoryName))
		}
		kind, err := models.ParseNeutralType(kindStr)
		if err != nil {
			return p.error(fmt.Sprintf("territory %q: %v", territoryName, err))
		}
		territory.NeutralType = kind
	}
	return nil
}
