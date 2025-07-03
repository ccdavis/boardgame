package parsing

import (
	"fmt"
)

type TokenType int

const (
	UNKNOWN TokenType = iota
	END_OF_FILE
	NEWLINE
	COMMA
	SEMICOLON
	COLON
	LEFT_BRACKET
	RIGHT_BRACKET
	LEFT_PAREN
	RIGHT_PAREN
	LEFT_BRACE
	RIGHT_BRACE
	INTEGER
	FLOAT
	STRING
	RANGE
	IDENTIFIER
	PLUS
	MINUS
	MULTIPLY
	DIVIDE
	LESSTHAN
	GREATERTHAN
	EQUAL
	ASSIGN
	AND
	OR
	NOT
	IF
	THEN
	ELSE
	WHILE
	RETURN
	FUNC
	LIST
	RECORD
	MAP
	FILTER
	READ
	WRITE
	PLAYERS
	TURN
	TERRITORIES
	UNITS
	CONTAINERS
	PLACEMENT
)

type Range struct {
	Lo int64
	Hi int64
}

type Token struct {
	Type       TokenType
	IntValue   int64
	FloatValue float64
	StrValue   string
	RangeValue Range
	BoolValue  bool
	Line       int
}

func NewToken(tokenType TokenType) *Token {
	return &Token{Type: tokenType}
}

func NewTokenWithInt(tokenType TokenType, value int64) *Token {
	return &Token{Type: tokenType, IntValue: value}
}

func NewTokenWithFloat(tokenType TokenType, value float64) *Token {
	return &Token{Type: tokenType, FloatValue: value}
}

func NewTokenWithString(tokenType TokenType, value string) *Token {
	return &Token{Type: tokenType, StrValue: value}
}

func NewTokenWithRange(tokenType TokenType, lo, hi int64) *Token {
	return &Token{Type: tokenType, RangeValue: Range{Lo: lo, Hi: hi}}
}

func NewTokenWithBool(tokenType TokenType, value bool) *Token {
	return &Token{Type: tokenType, BoolValue: value}
}

func (t *Token) String() string {
	switch t.Type {
	case INTEGER:
		return fmt.Sprintf("INTEGER(%d)", t.IntValue)
	case FLOAT:
		return fmt.Sprintf("FLOAT(%f)", t.FloatValue)
	case STRING:
		return fmt.Sprintf("STRING(%q)", t.StrValue)
	case IDENTIFIER:
		return fmt.Sprintf("IDENTIFIER(%s)", t.StrValue)
	case RANGE:
		return fmt.Sprintf("RANGE(%d..%d)", t.RangeValue.Lo, t.RangeValue.Hi)
	default:
		return tokenTypeNames[t.Type]
	}
}

var tokenTypeNames = map[TokenType]string{
	UNKNOWN:       "UNKNOWN",
	END_OF_FILE:   "EOF",
	NEWLINE:       "NEWLINE",
	COMMA:         "COMMA",
	SEMICOLON:     "SEMICOLON",
	COLON:         "COLON",
	LEFT_BRACKET:  "LEFT_BRACKET",
	RIGHT_BRACKET: "RIGHT_BRACKET",
	LEFT_PAREN:    "LEFT_PAREN",
	RIGHT_PAREN:   "RIGHT_PAREN",
	LEFT_BRACE:    "LEFT_BRACE",
	RIGHT_BRACE:   "RIGHT_BRACE",
	INTEGER:       "INTEGER",
	FLOAT:         "FLOAT",
	STRING:        "STRING",
	RANGE:         "RANGE",
	IDENTIFIER:    "IDENTIFIER",
	PLUS:          "PLUS",
	MINUS:         "MINUS",
	MULTIPLY:      "MULTIPLY",
	DIVIDE:        "DIVIDE",
	LESSTHAN:      "LESSTHAN",
	GREATERTHAN:   "GREATERTHAN",
	EQUAL:         "EQUAL",
	ASSIGN:        "ASSIGN",
	AND:           "AND",
	OR:            "OR",
	NOT:           "NOT",
	IF:            "IF",
	THEN:          "THEN",
	ELSE:          "ELSE",
	WHILE:         "WHILE",
	RETURN:        "RETURN",
	FUNC:          "FUNC",
	LIST:          "LIST",
	RECORD:        "RECORD",
	MAP:           "MAP",
	FILTER:        "FILTER",
	READ:          "READ",
	WRITE:         "WRITE",
	PLAYERS:       "PLAYERS",
	TURN:          "TURN",
	TERRITORIES:   "TERRITORIES",
	UNITS:         "UNITS",
	CONTAINERS:    "CONTAINERS",
	PLACEMENT:     "PLACEMENT",
}