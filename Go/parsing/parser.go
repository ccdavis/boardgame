package parsing

import (
	"fmt"
	"io"
)

type ParseError struct {
	Line    int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("Parse error at line %d: %s", e.Line, e.Message)
}

type Parser struct {
	scanner   *Scanner
	lookahead *Token
	lastToken *Token
}

func NewParser(r io.Reader) *Parser {
	p := &Parser{
		scanner: NewScanner(r),
	}
	p.lookahead = p.scanner.NextToken()
	return p
}

func (p *Parser) Match(expected TokenType) error {
	if p.lookahead.Type != expected {
		return &ParseError{
			Line:    p.lookahead.Line,
			Message: fmt.Sprintf("expected %s, got %s", tokenTypeNames[expected], p.lookahead.String()),
		}
	}
	p.Skip()
	return nil
}

func (p *Parser) Skip() {
	p.lastToken = p.lookahead
	p.lookahead = p.scanner.NextToken()
}

func (p *Parser) NextToken() *Token {
	return p.lookahead
}

func (p *Parser) LastTokenAsString() string {
	if p.lastToken != nil {
		return p.lastToken.StrValue
	}
	return ""
}

func (p *Parser) LastTokenAsInteger() int64 {
	if p.lastToken != nil {
		return p.lastToken.IntValue
	}
	return 0
}

func (p *Parser) LastTokenAsFloat() float64 {
	if p.lastToken != nil {
		return p.lastToken.FloatValue
	}
	return 0.0
}

func (p *Parser) LastTokenAsRange() Range {
	if p.lastToken != nil {
		return p.lastToken.RangeValue
	}
	return Range{}
}

// Deprecated - scanner now skips newlines
func (p *Parser) SkipNewlines() {
	// No-op - kept for compatibility
}