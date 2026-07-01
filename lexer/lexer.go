package lexer

import "fmt"

type TokenType string

const (
	TOKEN_VARIABLE     TokenType = "VARIABLE"
	TOKEN_VARIABLE_KEY TokenType = "VARIABLE_KEY"
	TOKEN_FUNC         TokenType = "FUNC"
	TOKEN_IF           TokenType = "IF"
	TOKEN_LOOP         TokenType = "LOOP"
	TOKEN_ERROR        TokenType = "ERROR"
	TOKEN_FATAL        TokenType = "FATAL"
	TOKEN_IMPORT       TokenType = "IMPORT"
	TOKEN_EXTERN       TokenType = "EXTERN"
	TOKEN_EXPLANATION  TokenType = "EXPLANATION"
	TOKEN_ASYNC        TokenType = "ASYNC"
	TOKEN_AWAIT        TokenType = "AWAIT"
	TOKEN_GPU          TokenType = "GPU"
	TOKEN_MEM          TokenType = "MEM"
	TOKEN_LE           TokenType = "LE"
	TOKEN_LT           TokenType = "LT"
	TOKEN_GE           TokenType = "GE"
	TOKEN_GT           TokenType = "GT"
	TOKEN_EQ           TokenType = "EQ"
	TOKEN_NE           TokenType = "NE"
	TOKEN_FUNC_PUBLIC  TokenType = "FUNC_PUBLIC"
	TOKEN_MUTATION     TokenType = "MUTATION"

	TOKEN_LET      TokenType = "LET"
	TOKEN_UNCLET   TokenType = "UNCLET"
	TOKEN_FOR      TokenType = "FOR"
	TOKEN_COUNT    TokenType = "COUNT"
	TOKEN_CHECK    TokenType = "CHECK"
	TOKEN_TRUE     TokenType = "TRUE"
	TOKEN_FALSE    TokenType = "FALSE"
	TOKEN_BODY     TokenType = "BODY"
	TOKEN_STEP     TokenType = "STEP"
	TOKEN_RECEIVE  TokenType = "RECEIVE"
	TOKEN_RETURN   TokenType = "RETURN"
	TOKEN_CALL     TokenType = "CALL"
	TOKEN_TRY      TokenType = "TRY"
	TOKEN_OK       TokenType = "OK"
	TOKEN_ERR      TokenType = "ERR"
	TOKEN_PASS     TokenType = "PASS"
	TOKEN_DEF      TokenType = "DEF"
	TOKEN_MOVE     TokenType = "MOVE"
	TOKEN_DROP     TokenType = "DROP"
	TOKEN_RAW      TokenType = "RAW"
	TOKEN_LIB      TokenType = "LIB"
	TOKEN_ADDR     TokenType = "ADDR"
	TOKEN_DEREF    TokenType = "DEREF"
	TOKEN_CAST     TokenType = "CAST"
	TOKEN_INDEX    TokenType = "INDEX"
	TOKEN_BREAK    TokenType = "BREAK"
	TOKEN_CONTINUE TokenType = "CONTINUE"

	TOKEN_LBRACKET TokenType = "["
	TOKEN_RBRACKET TokenType = "]"
	TOKEN_LBRACE   TokenType = "{"
	TOKEN_RBRACE   TokenType = "}"
	TOKEN_LPAREN   TokenType = "("
	TOKEN_RPAREN   TokenType = ")"
	TOKEN_COMMA    TokenType = ","
	TOKEN_COLON    TokenType = ":"
	TOKEN_SEMI     TokenType = ";"

	TOKEN_PLUS  TokenType = "+"
	TOKEN_MINUS TokenType = "-"
	TOKEN_STAR  TokenType = "*"
	TOKEN_SLASH TokenType = "/"

	TOKEN_INT_LIT    TokenType = "INT_LIT"
	TOKEN_FLOAT_LIT  TokenType = "FLOAT_LIT"
	TOKEN_STRING_LIT TokenType = "STRING_LIT"
	TOKEN_BOOL_LIT   TokenType = "BOOL_LIT"
	TOKEN_IDENT      TokenType = "IDENT"

	TOKEN_EOF TokenType = "EOF"
)

var keywords = map[string]TokenType{
	"Variable":        TOKEN_VARIABLE,
	"variable":        TOKEN_VARIABLE_KEY, // 小文字のvariable
	"Function":        TOKEN_FUNC,
	"Function_public": TOKEN_FUNC_PUBLIC,
	"If":              TOKEN_IF,
	"Loop":            TOKEN_LOOP,
	"Error":           TOKEN_ERROR,
	"Fatal":           TOKEN_FATAL,
	"Import":          TOKEN_IMPORT,
	"not_clear":       TOKEN_IDENT,
	"Extern":          TOKEN_EXTERN,
	"Explanation":     TOKEN_EXPLANATION,
	"Async":           TOKEN_ASYNC,
	"Await":           TOKEN_AWAIT,
	"GPU":             TOKEN_GPU,
	"Mem":             TOKEN_MEM,
	"let":             TOKEN_LET,
	"unchanging_let":  TOKEN_UNCLET,
	"for":             TOKEN_FOR,
	"Count":           TOKEN_COUNT,
	"check":           TOKEN_CHECK,
	"True":            TOKEN_TRUE,
	"False":           TOKEN_FALSE,
	"Body":            TOKEN_BODY,
	"step":            TOKEN_STEP,
	"receive":         TOKEN_RECEIVE,
	"return":          TOKEN_RETURN,
	"call":            TOKEN_CALL,
	"try":             TOKEN_TRY,
	"Ok":              TOKEN_OK,
	"Err":             TOKEN_ERR,
	"pass":            TOKEN_PASS,
	"def":             TOKEN_DEF,
	"Move":            TOKEN_MOVE,
	"Drop":            TOKEN_DROP,
	"Raw":             TOKEN_RAW,
	"lib":             TOKEN_LIB,
	"true":            TOKEN_BOOL_LIT,
	"false":           TOKEN_BOOL_LIT,
	"Mutation":        TOKEN_MUTATION,
	"addr":            TOKEN_ADDR,
	"deref":           TOKEN_DEREF,
	"cast":            TOKEN_CAST,
	"index":           TOKEN_INDEX,
	"break":           TOKEN_BREAK,
	"continue":        TOKEN_CONTINUE,

	"lesseq":    TOKEN_LE,
	"less":      TOKEN_LT,
	"greatereq": TOKEN_GE,
	"greater":   TOKEN_GT,
	"equal":     TOKEN_EQ,
	"notequal":  TOKEN_NE,
}

type Token struct {
	Type    TokenType
	Literal string
	Line    int
}

func (t Token) String() string {
	return fmt.Sprintf("[%s:%q line:%d]", t.Type, t.Literal, t.Line)
}

type Lexer struct {
	input  string
	pos    int
	line   int
	Errors []string
}

func New(input string) *Lexer {
	return &Lexer{input: input, pos: 0, line: 1}
}

func (l *Lexer) Tokenize() []Token {
	var tokens []Token
	for {
		tok := l.nextToken()
		tokens = append(tokens, tok)
		if tok.Type == TOKEN_EOF {
			break
		}
	}
	return tokens
}

func (l *Lexer) nextToken() Token {
	l.skipWhitespaceAndComments()

	if l.pos >= len(l.input) {
		return Token{TOKEN_EOF, "", l.line}
	}

	ch := l.input[l.pos]

	switch ch {
	case '[':
		l.pos++
		return Token{TOKEN_LBRACKET, "[", l.line}
	case ']':
		l.pos++
		return Token{TOKEN_RBRACKET, "]", l.line}
	case '{':
		l.pos++
		return Token{TOKEN_LBRACE, "{", l.line}
	case '}':
		l.pos++
		return Token{TOKEN_RBRACE, "}", l.line}
	case '(':
		l.pos++
		return Token{TOKEN_LPAREN, "(", l.line}
	case ')':
		l.pos++
		return Token{TOKEN_RPAREN, ")", l.line}
	case ',':
		l.pos++
		return Token{TOKEN_COMMA, ",", l.line}
	case ':':
		l.pos++
		return Token{TOKEN_COLON, ":", l.line}
	case ';':
		l.pos++
		return Token{TOKEN_SEMI, ";", l.line}
	case '+':
		l.pos++
		return Token{TOKEN_PLUS, "+", l.line}
	case '-':
		l.pos++
		return Token{TOKEN_MINUS, "-", l.line}
	case '*':
		l.pos++
		return Token{TOKEN_STAR, "*", l.line}
	case '/':
		l.pos++
		return Token{TOKEN_SLASH, "/", l.line}
	case '"':
		return l.readString()
	}

	if isDigit(ch) {
		return l.readNumber()
	}

	if isLetter(ch) {
		return l.readIdentOrKeyword()
	}

	l.Errors = append(l.Errors,
		fmt.Sprintf("Error: %d line, %q(不明な文字). errornumber10001", l.line, ch))
	l.pos++
	return l.nextToken()
}

func (l *Lexer) readString() Token {
	l.pos++ // skip "
	start := l.pos
	for {
		if l.pos >= len(l.input) {
			l.Errors = append(l.Errors,
				fmt.Sprintf("Error: %d line, String(ダブルクォーテーションが閉じられていない). errornumber10010", l.line))
			break
		}
		if l.input[l.pos] == '"' {
			break
		}
		if l.input[l.pos] == '\n' {
			l.Errors = append(l.Errors,
				fmt.Sprintf("Error: %d line, String(ダブルクォーテーションが閉じられていない). errornumber10010", l.line))
			break
		}
		l.pos++
	}
	val := l.input[start:l.pos]
	if l.pos < len(l.input) && l.input[l.pos] == '"' {
		l.pos++ // skip closing "
	}
	return Token{TOKEN_STRING_LIT, val, l.line}
}

func (l *Lexer) readNumber() Token {
	start := l.pos
	isFloat := false
	for l.pos < len(l.input) && (isDigit(l.input[l.pos]) || l.input[l.pos] == '.') {
		if l.input[l.pos] == '.' {
			isFloat = true
		}
		l.pos++
	}
	lit := l.input[start:l.pos]
	if isFloat {
		return Token{TOKEN_FLOAT_LIT, lit, l.line}
	}
	return Token{TOKEN_INT_LIT, lit, l.line}
}

func (l *Lexer) readIdentOrKeyword() Token {
	start := l.pos
	for l.pos < len(l.input) && (isLetter(l.input[l.pos]) || isDigit(l.input[l.pos]) || l.input[l.pos] == '_') {
		l.pos++
	}
	lit := l.input[start:l.pos]
	if tt, ok := keywords[lit]; ok {
		return Token{tt, lit, l.line}
	}
	return Token{TOKEN_IDENT, lit, l.line}
}

func (l *Lexer) skipWhitespaceAndComments() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\n' {
			l.line++
			l.pos++
		} else if ch == ' ' || ch == '\t' || ch == '\r' {
			l.pos++
		} else if l.pos+1 < len(l.input) && ch == '/' && l.input[l.pos+1] == '/' {
			for l.pos < len(l.input) && l.input[l.pos] != '\n' {
				l.pos++
			}
		} else {
			break
		}
	}
}

func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }
func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch > 127
}
