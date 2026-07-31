package scanner

import "fmt"

type TokenType int

const (
	UNKNOWN TokenType = iota
	EOF
	NEWLINE
	COMMA
	LEFT_BRACKET
	RIGHT_BRACKET
	LEFT_PAREN
	RIGHT_PAREN
	LEFT_BRACE
	RIGHT_BRACE
	INTEGER
	FLOAT
	STRING
	IDENTIFIER
	SEMICOLON
	COLON
	PLUS
	MINUS
	MULTIPLY
	DIVIDE
	LESSTHAN
	GREATERTHAN
	EQUAL
	ASSIGN

	// Reserved words
	PLAYERS
	TURN
	TERRITORIES
	UNITS
	CONTAINERS
	PLACEMENT
	MAP
	SIDES
	CAPITALS
	VICTORYCITIES
	NEUTRALITY
)

var tokenNames = map[TokenType]string{
	UNKNOWN:       "unknown",
	EOF:           "end of file",
	NEWLINE:       "newline",
	COMMA:         "comma",
	LEFT_BRACKET:  "left bracket",
	RIGHT_BRACKET: "right bracket",
	LEFT_PAREN:    "left parentheses",
	RIGHT_PAREN:   "right parentheses",
	LEFT_BRACE:    "left brace",
	RIGHT_BRACE:   "right brace",
	INTEGER:       "integer",
	FLOAT:         "float",
	STRING:        "string",
	IDENTIFIER:    "identifier",
	SEMICOLON:     "semicolon",
	COLON:         "colon",
	PLUS:          "plus",
	MINUS:         "minus",
	MULTIPLY:      "multiply",
	DIVIDE:        "divide",
	LESSTHAN:      "less than",
	GREATERTHAN:   "greater than",
	EQUAL:         "equal",
	ASSIGN:        "assign",
	PLAYERS:       "players",
	TURN:          "turn",
	TERRITORIES:   "territories",
	UNITS:         "units",
	CONTAINERS:    "containers",
	PLACEMENT:     "placement",
	MAP:           "map",
	SIDES:         "sides",
	CAPITALS:      "capitals",
	VICTORYCITIES: "victorycities",
	NEUTRALITY:    "neutrality",
}

var reservedWords = map[string]TokenType{
	"players":     PLAYERS,
	"turn":        TURN,
	"territories": TERRITORIES,
	"units":       UNITS,
	"containers":  CONTAINERS,
	"placement":   PLACEMENT,
	"map":         MAP,
	// Board metadata sections. These are new keywords, so they must not collide
	// with any identifier already used in a .gdf. Note in particular that
	// "neutral" is NOT reserved: aaa.gdf uses Neutral as a territory owner, and
	// reserved words are matched case-insensitively, so reserving it would break
	// every neutral territory line on the board.
	"sides":         SIDES,
	"capitals":      CAPITALS,
	"victorycities": VICTORYCITIES,
	"neutrality":    NEUTRALITY,
}

func (t TokenType) String() string {
	if name, ok := tokenNames[t]; ok {
		return name
	}
	return fmt.Sprintf("TokenType(%d)", t)
}

type Token struct {
	Type    TokenType
	Content string
	IntVal  int64
	FloatVal float64
	Line    int
}

func NewToken(tokenType TokenType, line int) *Token {
	return &Token{Type: tokenType, Line: line}
}

func NewTokenWithString(tokenType TokenType, content string, line int) *Token {
	return &Token{Type: tokenType, Content: content, Line: line}
}

func NewTokenWithInt(tokenType TokenType, val int64, line int) *Token {
	return &Token{Type: tokenType, IntVal: val, Line: line}
}

func NewTokenWithFloat(tokenType TokenType, val float64, line int) *Token {
	return &Token{Type: tokenType, FloatVal: val, Line: line}
}

func (t *Token) String() string {
	switch t.Type {
	case INTEGER:
		return fmt.Sprintf("INTEGER(%d)", t.IntVal)
	case FLOAT:
		return fmt.Sprintf("FLOAT(%f)", t.FloatVal)
	case STRING, IDENTIFIER:
		return fmt.Sprintf("%s(%s)", t.Type, t.Content)
	default:
		return t.Type.String()
	}
}
