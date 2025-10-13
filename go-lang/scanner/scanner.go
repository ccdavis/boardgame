package scanner

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

type Scanner struct {
	reader   *bufio.Reader
	lastChar rune
	line     int
	eof      bool
}

func NewScanner(r io.Reader) *Scanner {
	s := &Scanner{
		reader: bufio.NewReader(r),
		line:   1,
	}
	s.nextChar() // Prime the scanner
	return s
}

func (s *Scanner) nextChar() rune {
	ch, _, err := s.reader.ReadRune()
	if err != nil {
		if err == io.EOF {
			s.eof = true
			s.lastChar = 0
			return 0
		}
		s.lastChar = 0
		return 0
	}

	if ch == '\n' {
		s.line++
	}

	s.lastChar = ch
	return ch
}

func (s *Scanner) skipLine() {
	for s.lastChar != '\n' && !s.eof {
		s.nextChar()
	}
}

func isWhitespace(ch rune) bool {
	return unicode.IsSpace(ch)
}

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch)
}

func isDigit(ch rune) bool {
	return unicode.IsDigit(ch)
}

func (s *Scanner) NextToken() *Token {
	// Skip whitespace
	for isWhitespace(s.lastChar) && !s.eof {
		s.nextChar()
	}

	if s.eof {
		return NewToken(EOF, s.line)
	}

	// Numbers (including negative)
	if isDigit(s.lastChar) || s.lastChar == '-' {
		return s.scanNumber()
	}

	// Strings
	if s.lastChar == '"' {
		return s.scanString()
	}

	// Identifiers and reserved words
	if isLetter(s.lastChar) {
		return s.scanIdentifier()
	}

	// Single-character tokens
	return s.scanOperator()
}

func (s *Scanner) scanNumber() *Token {
	var sb strings.Builder
	startLine := s.line
	isFloat := false

	// Handle negative sign
	if s.lastChar == '-' {
		sb.WriteRune(s.lastChar)
		s.nextChar()
		// Make sure there's a digit after the minus
		if !isDigit(s.lastChar) {
			// It's just a minus operator, back up
			return NewToken(MINUS, startLine)
		}
	}

	// Collect digits
	for isDigit(s.lastChar) && !s.eof {
		sb.WriteRune(s.lastChar)
		s.nextChar()
	}

	// Check for decimal point
	if s.lastChar == '.' && !s.eof {
		// Peek ahead to see if it's a float or range (..)
		s.nextChar()
		if s.lastChar == '.' {
			// It's a range, not supported in game format, treat as float
			// For now, just treat as integer
			numStr := sb.String()
			val, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				fmt.Printf("Error parsing integer on line %d: %v\n", startLine, err)
				return NewToken(UNKNOWN, startLine)
			}
			// Put back the dots
			return NewTokenWithInt(INTEGER, val, startLine)
		}

		if isDigit(s.lastChar) {
			// It's a float
			isFloat = true
			sb.WriteRune('.')
			for isDigit(s.lastChar) && !s.eof {
				sb.WriteRune(s.lastChar)
				s.nextChar()
			}
		}
	}

	numStr := sb.String()
	if isFloat {
		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			fmt.Printf("Error parsing float on line %d: %v\n", startLine, err)
			return NewToken(UNKNOWN, startLine)
		}
		return NewTokenWithFloat(FLOAT, val, startLine)
	}

	val, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		fmt.Printf("Error parsing integer on line %d: %v\n", startLine, err)
		return NewToken(UNKNOWN, startLine)
	}
	return NewTokenWithInt(INTEGER, val, startLine)
}

func (s *Scanner) scanString() *Token {
	var sb strings.Builder
	startLine := s.line

	// Skip opening quote
	s.nextChar()

	for s.lastChar != '"' && !s.eof {
		if s.lastChar == '\n' || s.lastChar == '\t' {
			fmt.Printf("Error: Unterminated string on line %d\n", startLine)
			return NewToken(UNKNOWN, startLine)
		}
		sb.WriteRune(s.lastChar)
		s.nextChar()
	}

	// Skip closing quote
	s.nextChar()

	return NewTokenWithString(STRING, sb.String(), startLine)
}

func (s *Scanner) scanIdentifier() *Token {
	var sb strings.Builder
	startLine := s.line

	// Read letters and digits only (like C++ version)
	for (isLetter(s.lastChar) || isDigit(s.lastChar)) && !s.eof {
		sb.WriteRune(s.lastChar)
		s.nextChar()
	}

	str := sb.String()

	// Check if it's a reserved word (case-insensitive)
	if tokenType, ok := reservedWords[strings.ToLower(str)]; ok {
		return NewToken(tokenType, startLine)
	}

	return NewTokenWithString(IDENTIFIER, str, startLine)
}

func (s *Scanner) isDelimiter() bool {
	return s.lastChar == ',' || s.lastChar == ';' || s.lastChar == ':' ||
		s.lastChar == '(' || s.lastChar == ')' || s.lastChar == '[' ||
		s.lastChar == ']' || s.lastChar == '{' || s.lastChar == '}' ||
		s.lastChar == '\n' || s.eof
}

func (s *Scanner) scanOperator() *Token {
	ch := s.lastChar
	startLine := s.line
	s.nextChar()

	switch ch {
	case ',':
		return NewToken(COMMA, startLine)
	case '(':
		return NewToken(LEFT_PAREN, startLine)
	case ')':
		return NewToken(RIGHT_PAREN, startLine)
	case '[':
		return NewToken(LEFT_BRACKET, startLine)
	case ']':
		return NewToken(RIGHT_BRACKET, startLine)
	case ';':
		return NewToken(SEMICOLON, startLine)
	case ':':
		if s.lastChar == '=' {
			s.nextChar()
			return NewToken(ASSIGN, startLine)
		}
		return NewToken(COLON, startLine)
	case '=':
		return NewToken(EQUAL, startLine)
	case '<':
		return NewToken(LESSTHAN, startLine)
	case '>':
		return NewToken(GREATERTHAN, startLine)
	case '-':
		return NewToken(MINUS, startLine)
	case '+':
		return NewToken(PLUS, startLine)
	case '/':
		return NewToken(DIVIDE, startLine)
	case '{':
		return NewToken(LEFT_BRACE, startLine)
	case '}':
		return NewToken(RIGHT_BRACE, startLine)
	case '*':
		return NewToken(MULTIPLY, startLine)
	case '#':
		// Comment - skip to end of line
		s.skipLine()
		return s.NextToken()
	default:
		fmt.Printf("Unhandled character '%c' (%d) on line %d - skipping\n", ch, ch, startLine)
		return s.NextToken()
	}
}
