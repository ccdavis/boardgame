package scanner

import (
	"strings"
	"testing"
)

// tokenize reads a source to EOF and returns the token stream.
func tokenize(src string) []*Token {
	s := NewScanner(strings.NewReader(src))
	tokens := make([]*Token, 0)
	for {
		token := s.NextToken()
		if token.Type == EOF {
			return tokens
		}
		tokens = append(tokens, token)
	}
}

func types(tokens []*Token) []TokenType {
	out := make([]TokenType, len(tokens))
	for i, token := range tokens {
		out[i] = token.Type
	}
	return out
}

func TestScanner_Punctuation(t *testing.T) {
	got := types(tokenize(":,;"))
	want := []TokenType{COLON, COMMA, SEMICOLON}

	if len(got) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestScanner_Numbers(t *testing.T) {
	tokens := tokenize("10 0 137")
	if len(tokens) != 3 {
		t.Fatalf("got %d tokens, want 3", len(tokens))
	}
	for _, token := range tokens {
		if token.Type != INTEGER {
			t.Errorf("token %q scanned as %v, want INTEGER", token.Content, token.Type)
		}
	}
	// An integer carries its value in IntVal; Content is for text tokens.
	if tokens[0].IntVal != 10 {
		t.Errorf("first number is %d, want 10", tokens[0].IntVal)
	}
	if tokens[2].IntVal != 137 {
		t.Errorf("third number is %d, want 137", tokens[2].IntVal)
	}
}

// Reserved words are matched without regard to case. This is why "neutral"
// must never become a keyword: aaa.gdf uses Neutral as a territory owner
// throughout, and reserving it would break every neutral territory line.
func TestScanner_ReservedWordsAreCaseInsensitive(t *testing.T) {
	for _, src := range []string{"Players", "players", "PLAYERS", "PlAyErS"} {
		tokens := tokenize(src)
		if len(tokens) != 1 {
			t.Fatalf("%q produced %d tokens", src, len(tokens))
		}
		if tokens[0].Type != PLAYERS {
			t.Errorf("%q scanned as %v, want PLAYERS", src, tokens[0].Type)
		}
	}
}

func TestScanner_BoardMetadataKeywords(t *testing.T) {
	cases := map[string]TokenType{
		"Sides":         SIDES,
		"Capitals":      CAPITALS,
		"VictoryCities": VICTORYCITIES,
		"Neutrality":    NEUTRALITY,
	}
	for src, want := range cases {
		tokens := tokenize(src)
		if len(tokens) != 1 || tokens[0].Type != want {
			t.Errorf("%q scanned as %v, want %v", src, types(tokens), want)
		}
	}
}

// "Neutral" is an owner name, not a keyword. If it were reserved, every
// neutral territory in the board file would fail to parse.
func TestScanner_NeutralIsAnIdentifier(t *testing.T) {
	tokens := tokenize("Neutral")
	if len(tokens) != 1 {
		t.Fatalf("got %d tokens, want 1", len(tokens))
	}
	if tokens[0].Type != IDENTIFIER {
		t.Errorf("Neutral scanned as %v, want IDENTIFIER", tokens[0].Type)
	}
}

func TestScanner_SkipsComments(t *testing.T) {
	tokens := tokenize("Germany # this is a comment\n Japan")
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2 (%v)", len(tokens), types(tokens))
	}
	if tokens[0].Content != "Germany" || tokens[1].Content != "Japan" {
		t.Errorf("got %q and %q", tokens[0].Content, tokens[1].Content)
	}
}

// Identifiers may contain underscores. The original scanner accepted only
// letters and digits, so a name like industrial_complex could not be lexed at
// all -- it arrived as two tokens with the underscore silently dropped.
func TestScanner_IdentifiersMayContainUnderscores(t *testing.T) {
	tokens := tokenize("industrial_complex")
	if len(tokens) != 1 {
		t.Fatalf("got %d tokens, want 1: %v", len(tokens), types(tokens))
	}
	if tokens[0].Content != "industrial_complex" {
		t.Errorf("scanned %q, want industrial_complex", tokens[0].Content)
	}
}

// Territory names are token sequences, not strings, so a name may wrap across
// source lines and internal whitespace collapses. Several names in the board
// file do wrap.
func TestScanner_MultiWordNamesAreSeparateTokens(t *testing.T) {
	tokens := tokenize("South West\n Indian Ocean")
	if len(tokens) != 4 {
		t.Fatalf("got %d tokens, want 4: %v", len(tokens), types(tokens))
	}
	for _, token := range tokens {
		if token.Type != IDENTIFIER {
			t.Errorf("token %q is %v, want IDENTIFIER", token.Content, token.Type)
		}
	}
}

func TestScanner_EmptyInput(t *testing.T) {
	if tokens := tokenize(""); len(tokens) != 0 {
		t.Errorf("empty input produced %d tokens", len(tokens))
	}
}

func TestScanner_WhitespaceOnly(t *testing.T) {
	if tokens := tokenize("  \n\t\r\n  "); len(tokens) != 0 {
		t.Errorf("whitespace produced %d tokens", len(tokens))
	}
}

// Line numbers drive parser error messages, so they need to be roughly right.
func TestScanner_TracksLineNumbers(t *testing.T) {
	tokens := tokenize("first\nsecond\nthird")
	if len(tokens) != 3 {
		t.Fatalf("got %d tokens", len(tokens))
	}
	if tokens[0].Line >= tokens[2].Line {
		t.Errorf("line numbers do not increase: %d then %d", tokens[0].Line, tokens[2].Line)
	}
}
