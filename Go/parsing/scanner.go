package parsing

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

var reservedWords = map[string]TokenType{
	"players":     PLAYERS,
	"turn":        TURN,
	"territories": TERRITORIES,
	"units":       UNITS,
	"containers":  CONTAINERS,
	"placement":   PLACEMENT,
	"map":         MAP,
}

type Scanner struct {
	reader     *bufio.Reader
	line       int
	currentCh  rune
	nextCh     rune
	eofReached bool
}

func NewScanner(r io.Reader) *Scanner {
	s := &Scanner{
		reader: bufio.NewReader(r),
		line:   1,
	}
	s.advance()
	s.advance()
	return s
}

func (s *Scanner) advance() {
	s.currentCh = s.nextCh
	
	if s.eofReached {
		return
	}
	
	ch, _, err := s.reader.ReadRune()
	if err != nil {
		s.nextCh = 0
		s.eofReached = true
	} else {
		s.nextCh = ch
		if ch == '\n' {
			s.line++
		}
	}
}

func (s *Scanner) skipWhitespace() {
	for s.currentCh != 0 && unicode.IsSpace(s.currentCh) && s.currentCh != '\n' {
		s.advance()
	}
}

func (s *Scanner) atLineEnd() bool {
	ch := s.currentCh
	// Skip any spaces/tabs
	for ch != 0 && (ch == ' ' || ch == '\t') {
		ch = s.nextCh
	}
	return ch == '\n' || ch == 0
}

func (s *Scanner) skipComment() {
	if s.currentCh == '#' {
		for s.currentCh != 0 && s.currentCh != '\n' {
			s.advance()
		}
	}
}

func (s *Scanner) NextToken() *Token {
	for {
		// Skip all whitespace INCLUDING newlines
		for s.currentCh != 0 && unicode.IsSpace(s.currentCh) {
			s.advance()
		}
		
		if s.currentCh != '#' {
			break
		}
		s.skipComment()
	}

	token := &Token{Line: s.line}

	if s.currentCh == 0 {
		token.Type = END_OF_FILE
		return token
	}

	if unicode.IsDigit(s.currentCh) {
		return s.scanNumber()
	}

	if unicode.IsLetter(s.currentCh) {
		return s.scanIdentifier()
	}

	if s.currentCh == '"' {
		return s.scanString()
	}

	switch s.currentCh {
	case ',':
		token.Type = COMMA
		s.advance()
	case ';':
		token.Type = SEMICOLON
		s.advance()
	case ':':
		s.advance()
		if s.currentCh == '=' {
			token.Type = ASSIGN
			s.advance()
		} else {
			token.Type = COLON
		}
	case '[':
		token.Type = LEFT_BRACKET
		s.advance()
	case ']':
		token.Type = RIGHT_BRACKET
		s.advance()
	case '(':
		token.Type = LEFT_PAREN
		s.advance()
	case ')':
		token.Type = RIGHT_PAREN
		s.advance()
	case '{':
		token.Type = LEFT_BRACE
		s.advance()
	case '}':
		token.Type = RIGHT_BRACE
		s.advance()
	case '+':
		token.Type = PLUS
		s.advance()
	case '-':
		token.Type = MINUS
		s.advance()
	case '*':
		token.Type = MULTIPLY
		s.advance()
	case '/':
		token.Type = DIVIDE
		s.advance()
	case '<':
		token.Type = LESSTHAN
		s.advance()
	case '>':
		token.Type = GREATERTHAN
		s.advance()
	case '=':
		token.Type = EQUAL
		s.advance()
	default:
		token.Type = UNKNOWN
		s.advance()
	}

	return token
}

func (s *Scanner) scanNumber() *Token {
	var sb strings.Builder
	token := &Token{Line: s.line}

	for unicode.IsDigit(s.currentCh) {
		sb.WriteRune(s.currentCh)
		s.advance()
	}

	if s.currentCh == '.' && s.nextCh == '.' {
		lo, _ := strconv.ParseInt(sb.String(), 10, 64)
		s.advance()
		s.advance()
		sb.Reset()

		for unicode.IsDigit(s.currentCh) {
			sb.WriteRune(s.currentCh)
			s.advance()
		}

		hi, _ := strconv.ParseInt(sb.String(), 10, 64)
		token.Type = RANGE
		token.RangeValue = Range{Lo: lo, Hi: hi}
		return token
	}

	if s.currentCh == '.' && unicode.IsDigit(s.nextCh) {
		sb.WriteRune(s.currentCh)
		s.advance()

		for unicode.IsDigit(s.currentCh) {
			sb.WriteRune(s.currentCh)
			s.advance()
		}

		val, _ := strconv.ParseFloat(sb.String(), 64)
		token.Type = FLOAT
		token.FloatValue = val
		return token
	}

	val, _ := strconv.ParseInt(sb.String(), 10, 64)
	token.Type = INTEGER
	token.IntValue = val
	return token
}

func (s *Scanner) scanIdentifier() *Token {
	var sb strings.Builder
	token := &Token{Line: s.line}

	for unicode.IsLetter(s.currentCh) || unicode.IsDigit(s.currentCh) || s.currentCh == '_' {
		sb.WriteRune(s.currentCh)
		s.advance()
	}

	ident := sb.String()
	lowerIdent := strings.ToLower(ident)

	if tokenType, ok := reservedWords[lowerIdent]; ok {
		token.Type = tokenType
		token.StrValue = ident
	} else {
		token.Type = IDENTIFIER
		token.StrValue = ident
	}

	return token
}

func (s *Scanner) scanString() *Token {
	var sb strings.Builder
	token := &Token{Line: s.line, Type: STRING}

	s.advance()

	for s.currentCh != '"' && s.currentCh != 0 && s.currentCh != '\n' {
		if s.currentCh == '\\' && s.nextCh == '"' {
			sb.WriteRune('"')
			s.advance()
			s.advance()
		} else {
			sb.WriteRune(s.currentCh)
			s.advance()
		}
	}

	if s.currentCh == '"' {
		s.advance()
	} else {
		panic(fmt.Sprintf("Unterminated string at line %d", s.line))
	}

	token.StrValue = sb.String()
	return token
}

func (s *Scanner) CurrentLine() int {
	return s.line
}