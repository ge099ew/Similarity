package lexer

import "testing"

func TestBasicTokens(t *testing.T) {
	input := `Variable[let{int(x:10)}]`
	l := New(input)
	tokens := l.Tokenize()
	expected := []TokenType{
		TOKEN_VARIABLE, TOKEN_LBRACKET,
		TOKEN_LET, TOKEN_LBRACE,
		TOKEN_IDENT, TOKEN_LPAREN,
		TOKEN_IDENT, TOKEN_COLON, TOKEN_INT_LIT,
		TOKEN_RPAREN, TOKEN_RBRACE,
		TOKEN_RBRACKET, TOKEN_EOF,
	}
	for i, tt := range expected {
		if tokens[i].Type != tt {
			t.Errorf("token[%d]: want %s, got %s (%q)", i, tt, tokens[i].Type, tokens[i].Literal)
		}
	}
}

func TestIfToken(t *testing.T) {
	input := `If[check{le(hp,0)},True[Move(hp)],False[Drop(hp)]]`
	l := New(input)
	tokens := l.Tokenize()
	// 全トークンをデバッグ出力
	for i, tok := range tokens {
		t.Logf("token[%d]: %s %q", i, tok.Type, tok.Literal)
	}
	if tokens[0].Type != TOKEN_IF    { t.Errorf("want IF, got %s", tokens[0].Type) }
	if tokens[2].Type != TOKEN_CHECK { t.Errorf("want CHECK, got %s", tokens[2].Type) }
	if tokens[12].Type != TOKEN_TRUE { t.Errorf("want TRUE at 12, got %s", tokens[12].Type) }
}

func TestStringError(t *testing.T) {
	input := "Variable[let{String(name:\"hello"
	l := New(input)
	l.Tokenize()
	if len(l.Errors) == 0 {
		t.Error("クォーテーション閉じ忘れが検出されるはず")
	} else {
		t.Logf("エラー検出: %s", l.Errors[0])
	}
}

func TestExplanation(t *testing.T) {
	input := `Explanation[Application{Game(type:RPG)}]`
	l := New(input)
	tokens := l.Tokenize()
	if tokens[0].Type != TOKEN_EXPLANATION {
		t.Errorf("want EXPLANATION, got %s", tokens[0].Type)
	}
}

func TestFloatAndBool(t *testing.T) {
	input := `float(x:3.14) bool(flag:true)`
	l := New(input)
	tokens := l.Tokenize()
	hasFloat, hasBool := false, false
	for _, tok := range tokens {
		if tok.Type == TOKEN_FLOAT_LIT && tok.Literal == "3.14" { hasFloat = true }
		if tok.Type == TOKEN_BOOL_LIT  && tok.Literal == "true"  { hasBool = true }
	}
	if !hasFloat { t.Error("float 3.14 が検出されない") }
	if !hasBool  { t.Error("bool true が検出されない") }
}
