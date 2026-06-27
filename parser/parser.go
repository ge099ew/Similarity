package parser

import (
	"fmt"
	"similarity/ast"
	"similarity/lexer"
	"strconv"
)

type Parser struct {
	tokens []lexer.Token
	pos    int
	Errors []string
}

func New(tokens []lexer.Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

func (p *Parser) cur() lexer.Token { return p.tokens[p.pos] }
func (p *Parser) peek() lexer.Token {
	if p.pos+1 < len(p.tokens) {
		return p.tokens[p.pos+1]
	}
	return lexer.Token{Type: lexer.TOKEN_EOF}
}

func (p *Parser) advance() lexer.Token {
	tok := p.tokens[p.pos]
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return tok
}

func (p *Parser) expect(tt lexer.TokenType) (lexer.Token, bool) {
	if p.cur().Type != tt {
		p.errorf("want %s, got %s (%q)", tt, p.cur().Type, p.cur().Literal)
		return p.cur(), false
	}
	return p.advance(), true
}

func (p *Parser) errorf(format string, args ...interface{}) {
	line := p.cur().Line
	msg := fmt.Sprintf(format, args...)
	p.Errors = append(p.Errors,
		fmt.Sprintf("Error: %d line, %s(%s). errornumber10002", line, p.cur().Literal, msg))
}

// ParseProgram エントリーポイント
func (p *Parser) ParseProgram() *ast.Program {
	prog := &ast.Program{}

	if p.cur().Type == lexer.TOKEN_EXPLANATION {
		prog.Explanation = p.parseExplanation()
	}

	for p.cur().Type != lexer.TOKEN_EOF {
		if stmt := p.parseStatement(); stmt != nil {
			prog.Statements = append(prog.Statements, stmt)
		}
	}
	return prog
}

func (p *Parser) parseStatement() ast.Node {
	switch p.cur().Type {
	case lexer.TOKEN_VARIABLE:
		return p.parseVariable()
	case lexer.TOKEN_IF:
		return p.parseIf()
	case lexer.TOKEN_RETURN:
		return p.parseReturn()
	case lexer.TOKEN_LOOP:
		return p.parseLoop()
	case lexer.TOKEN_FUNC:
		return p.parseFunc(false)
	case lexer.TOKEN_FUNC_PUB:
		return p.parseFunc(true)
	case lexer.TOKEN_ERROR:
		return p.parseError()
	case lexer.TOKEN_FATAL:
		return p.parseFatal()
	case lexer.TOKEN_IMPORT:
		return p.parseImport()
	case lexer.TOKEN_EXTERN:
		return p.parseExtern()
	case lexer.TOKEN_CALL:
		return p.parseCall()
	default:
		p.errorf("文として解釈できません")
		p.advance()
		return nil
	}
}

// Explanation[Application{Game(type:RPG)}]
func (p *Parser) parseExplanation() *ast.ExplanationNode {
	p.advance() // skip Explanation
	p.expect(lexer.TOKEN_LBRACKET)

	node := &ast.ExplanationNode{Args: make(map[string]string)}
	node.Category = p.cur().Literal
	p.advance()

	p.expect(lexer.TOKEN_LBRACE)
	for p.cur().Type != lexer.TOKEN_RBRACE && p.cur().Type != lexer.TOKEN_EOF {
		if p.cur().Type == lexer.TOKEN_LPAREN {
			p.advance()
			for p.cur().Type != lexer.TOKEN_RPAREN && p.cur().Type != lexer.TOKEN_EOF {
				key := p.cur().Literal
				p.advance()
				p.expect(lexer.TOKEN_COLON)
				val := p.cur().Literal
				p.advance()
				node.Args[key] = val
				if p.cur().Type == lexer.TOKEN_COMMA {
					p.advance()
				}
			}
			p.expect(lexer.TOKEN_RPAREN)
		} else {
			// Explanation[System{HFT}] のような直値
			node.Args["value"] = p.cur().Literal
			p.advance()
		}
	}
	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Variable[let{int(x:10)}] / Variable[unclet{float(PI:3.14)}]
func (p *Parser) parseVariable() *ast.VariableNode {
	p.advance() // skip Variable
	p.expect(lexer.TOKEN_LBRACKET)

	mutable := p.cur().Type == lexer.TOKEN_LET
	p.advance() // skip let/unclet
	p.expect(lexer.TOKEN_LBRACE)

	node := &ast.VariableNode{Mutable: mutable}
	node.Type = p.cur().Literal
	p.advance() // int / float / bool / String / Box_int ...
	p.expect(lexer.TOKEN_LPAREN)
	node.Name = p.cur().Literal
	p.advance() // 変数名
	p.expect(lexer.TOKEN_COLON)
	node.Value = p.parseLiteral()
	p.expect(lexer.TOKEN_RPAREN)

	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// If[check{le(hp,0)}, True[...], False[...]]
func (p *Parser) parseIf() *ast.IfNode {
	p.advance() // skip If
	p.expect(lexer.TOKEN_LBRACKET)

	node := &ast.IfNode{}
	p.expect(lexer.TOKEN_CHECK)
	p.expect(lexer.TOKEN_LBRACE)
	node.Condition = p.parseCondition()
	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_COMMA)

	// True[...]
	p.expect(lexer.TOKEN_TRUE)
	p.expect(lexer.TOKEN_LBRACKET)
	node.True = p.parseBlock()
	p.expect(lexer.TOKEN_RBRACKET)

	// False[...] (任意)
	if p.cur().Type == lexer.TOKEN_COMMA {
		p.advance()
	}
	if p.cur().Type == lexer.TOKEN_FALSE {
		p.advance()
		p.expect(lexer.TOKEN_LBRACKET)
		node.False = p.parseBlock()
		p.expect(lexer.TOKEN_RBRACKET)
	}

	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Loop[for{int(i:0),lt(i,10),step{1}}, Body[...]]
// Loop[Count{int(i:10)}, Body[...]]
func (p *Parser) parseLoop() *ast.LoopNode {
	p.advance() // skip Loop
	p.expect(lexer.TOKEN_LBRACKET)

	node := &ast.LoopNode{}
	if p.cur().Type == lexer.TOKEN_FOR {
		node.Kind = "for"
		p.advance()
		p.expect(lexer.TOKEN_LBRACE)
		// int(i:0)
		node.Init = p.parseTypedValue()
		p.expect(lexer.TOKEN_COMMA)
		// lt(i,10)
		node.Condition = p.parseCondition()
		p.expect(lexer.TOKEN_COMMA)
		// step{1}
		p.expect(lexer.TOKEN_STEP)
		p.expect(lexer.TOKEN_LBRACE)
		stepVal, _ := strconv.Atoi(p.cur().Literal)
		p.advance()
		node.Step = stepVal
		p.expect(lexer.TOKEN_RBRACE)
		p.expect(lexer.TOKEN_RBRACE)
	} else if p.cur().Type == lexer.TOKEN_COUNT {
		node.Kind = "count"
		p.advance()
		p.expect(lexer.TOKEN_LBRACE)
		node.Init = p.parseTypedValue()
		p.expect(lexer.TOKEN_RBRACE)
	}

	p.expect(lexer.TOKEN_COMMA)
	p.expect(lexer.TOKEN_BODY)
	p.expect(lexer.TOKEN_LBRACKET)
	node.Body = p.parseBlock()
	p.expect(lexer.TOKEN_RBRACKET)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Func[name{receive{...}, 処理, return{...}}] / Func_pub[...]
func (p *Parser) parseFunc(pub bool) *ast.FuncNode {
	p.advance() // skip Func
	p.expect(lexer.TOKEN_LBRACKET)

	node := &ast.FuncNode{Public: pub}
	node.Name = p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_LBRACE)

	// receive{...}
	if p.cur().Type == lexer.TOKEN_RECEIVE {
		p.advance()
		p.expect(lexer.TOKEN_LBRACE)
		for p.cur().Type != lexer.TOKEN_RBRACE && p.cur().Type != lexer.TOKEN_EOF {
			typeName := p.cur().Literal
			p.advance()
			p.expect(lexer.TOKEN_LPAREN)
			paramName := p.cur().Literal
			p.advance()
			p.expect(lexer.TOKEN_RPAREN)
			node.Params = append(node.Params, ast.VariableNode{Type: typeName, Name: paramName})
			if p.cur().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
		p.expect(lexer.TOKEN_RBRACE)
		if p.cur().Type == lexer.TOKEN_COMMA {
			p.advance()
		}
	}

	// 処理
	for p.cur().Type != lexer.TOKEN_RBRACE && p.cur().Type != lexer.TOKEN_EOF {
		if stmt := p.parseStatement(); stmt != nil {
			node.Body = append(node.Body, stmt)
		}
		if p.cur().Type == lexer.TOKEN_COMMA {
			p.advance()
		}
	}

	// return{...}
	if p.cur().Type == lexer.TOKEN_RETURN {
		p.advance()
		p.expect(lexer.TOKEN_LBRACE)
		if p.cur().Type != lexer.TOKEN_RBRACE {
			node.Returns = p.parseLiteral()
		}
		p.expect(lexer.TOKEN_RBRACE)
	}

	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Error[try{...}, Ok[...], Err[type{...},msg{...}]]
func (p *Parser) parseError() *ast.ErrorNode {
	p.advance() // skip Error
	p.expect(lexer.TOKEN_LBRACKET)

	// def{TypeName} の場合
	if p.cur().Type == lexer.TOKEN_DEF {
		p.advance()
		p.expect(lexer.TOKEN_LBRACE)
		node := &ast.ErrorNode{ErrType: p.cur().Literal}
		p.advance()
		p.expect(lexer.TOKEN_RBRACE)
		p.expect(lexer.TOKEN_RBRACKET)
		return node
	}

	node := &ast.ErrorNode{}
	p.expect(lexer.TOKEN_TRY)
	p.expect(lexer.TOKEN_LBRACE)
	node.Try = p.parseBlock()
	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_COMMA)

	p.expect(lexer.TOKEN_OK)
	p.expect(lexer.TOKEN_LBRACKET)
	node.Ok = p.parseBlock()
	p.expect(lexer.TOKEN_RBRACKET)
	p.expect(lexer.TOKEN_COMMA)

	p.expect(lexer.TOKEN_ERR)
	p.expect(lexer.TOKEN_LBRACKET)
	if p.cur().Type == lexer.TOKEN_PASS {
		node.Pass = true
		p.advance()
	} else {
		// type{...}
		if p.cur().Type == lexer.TOKEN_IDENT && p.cur().Literal == "type" {
			p.advance()
			p.expect(lexer.TOKEN_LBRACE)
			node.ErrType = p.cur().Literal
			p.advance()
			p.expect(lexer.TOKEN_RBRACE)
			if p.cur().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
		// msg{...}
		if p.cur().Type == lexer.TOKEN_IDENT && p.cur().Literal == "msg" {
			p.advance()
			p.expect(lexer.TOKEN_LBRACE)
			node.Msg = p.cur().Literal
			p.advance()
			p.expect(lexer.TOKEN_RBRACE)
		}
		node.Err = p.parseBlock()
	}
	p.expect(lexer.TOKEN_RBRACKET)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Fatal[type{...}, msg{...}]
func (p *Parser) parseFatal() *ast.FatalNode {
	p.advance() // skip Fatal
	p.expect(lexer.TOKEN_LBRACKET)
	node := &ast.FatalNode{}
	if p.cur().Type == lexer.TOKEN_IDENT && p.cur().Literal == "type" {
		p.advance()
		p.expect(lexer.TOKEN_LBRACE)
		node.ErrType = p.cur().Literal
		p.advance()
		p.expect(lexer.TOKEN_RBRACE)
		if p.cur().Type == lexer.TOKEN_COMMA {
			p.advance()
		}
	}
	if p.cur().Type == lexer.TOKEN_IDENT && p.cur().Literal == "msg" {
		p.advance()
		p.expect(lexer.TOKEN_LBRACE)
		node.Msg = p.cur().Literal
		p.advance()
		p.expect(lexer.TOKEN_RBRACE)
	}
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Import[discord{token}]
func (p *Parser) parseImport() *ast.ImportNode {
	p.advance() // skip Import
	p.expect(lexer.TOKEN_LBRACKET)
	node := &ast.ImportNode{Module: p.cur().Literal}
	p.advance()
	p.expect(lexer.TOKEN_LBRACE)
	for p.cur().Type != lexer.TOKEN_RBRACE && p.cur().Type != lexer.TOKEN_EOF {
		node.Symbols = append(node.Symbols, p.cur().Literal)
		p.advance()
		if p.cur().Type == lexer.TOKEN_COMMA {
			p.advance()
		}
	}
	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Extern[C{lib{"SDL2"}, func{...}}]
func (p *Parser) parseExtern() *ast.ExternNode {
	p.advance() // skip Extern
	p.expect(lexer.TOKEN_LBRACKET)
	p.advance() // skip C
	p.expect(lexer.TOKEN_LBRACE)
	node := &ast.ExternNode{}
	// lib{...}
	if p.cur().Type == lexer.TOKEN_LIB {
		p.advance()
		p.expect(lexer.TOKEN_LBRACE)
		for p.cur().Type != lexer.TOKEN_RBRACE && p.cur().Type != lexer.TOKEN_EOF {
			node.Libs = append(node.Libs, p.cur().Literal)
			p.advance()
			if p.cur().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
		p.expect(lexer.TOKEN_RBRACE)
		if p.cur().Type == lexer.TOKEN_COMMA {
			p.advance()
		}
	}
	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// call{funcName(args)}
func (p *Parser) parseCall() *ast.CallNode {
	p.advance() // skip call
	p.expect(lexer.TOKEN_LBRACE)
	node := &ast.CallNode{FuncName: p.cur().Literal}
	p.advance()
	p.expect(lexer.TOKEN_LPAREN)
	for p.cur().Type != lexer.TOKEN_RPAREN && p.cur().Type != lexer.TOKEN_EOF {
		node.Args = append(node.Args, p.parseLiteral())
		if p.cur().Type == lexer.TOKEN_COMMA {
			p.advance()
		}
	}
	p.expect(lexer.TOKEN_RPAREN)
	p.expect(lexer.TOKEN_RBRACE)
	return node
}

// return{value}
func (p *Parser) parseReturn() *ast.ReturnNode {
	p.advance() // skip return
	p.expect(lexer.TOKEN_LBRACE)
	node := &ast.ReturnNode{}
	if p.cur().Type != lexer.TOKEN_RBRACE {
		node.Value = p.parseLiteral()
	}
	p.expect(lexer.TOKEN_RBRACE)
	return node
}

// ブロック内の文を繰り返しパース
func (p *Parser) parseBlock() []ast.Node {
	var nodes []ast.Node
	for {
		switch p.cur().Type {
		case lexer.TOKEN_RBRACKET, lexer.TOKEN_RBRACE, lexer.TOKEN_EOF:
			return nodes
		default:
			if stmt := p.parseStatement(); stmt != nil {
				nodes = append(nodes, stmt)
			}
			if p.cur().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
	}
}

// int(x:10) / float(x:3.14) / bool(x:true) / String(x:"hi")
func (p *Parser) parseTypedValue() *ast.VariableNode {
	node := &ast.VariableNode{Type: p.cur().Literal, Mutable: true}
	p.advance()
	p.expect(lexer.TOKEN_LPAREN)
	node.Name = p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_COLON)
	node.Value = p.parseLiteral()
	p.expect(lexer.TOKEN_RPAREN)
	return node
}

// リテラル値または演算式をパース
func (p *Parser) parseLiteral() ast.Node {
	// 演算式 +{...} -{...} *{...} /{...}
	switch p.cur().Type {
	case lexer.TOKEN_PLUS, lexer.TOKEN_MINUS, lexer.TOKEN_STAR, lexer.TOKEN_SLASH:
		return p.parseExpr()
	}
	tok := p.cur()
	p.advance()
	return &ast.LiteralNode{Kind: string(tok.Type), Value: tok.Literal, Line: tok.Line}
}

// 演算式をパース: +{int(a, b)} → ExprNode{Op:"+", Type:"int", Left:a, Right:b}
func (p *Parser) parseExpr() *ast.ExprNode {
	op := p.cur().Literal
	p.advance() // skip + - * /
	p.expect(lexer.TOKEN_LBRACE)

	node := &ast.ExprNode{Op: op}

	// 型名 (int, float, ...) を読む
	node.Type = p.cur().Literal
	p.advance()

	// (a, b) → 2つのオペランドを取り出す
	p.expect(lexer.TOKEN_LPAREN)
	node.Left = p.parseArg()
	if p.cur().Type == lexer.TOKEN_COMMA {
		p.advance()
		node.Right = p.parseArg()
	}
	p.expect(lexer.TOKEN_RPAREN)

	// *{int(c), +{int(a,b)}} のような第2引数がある場合
	if p.cur().Type == lexer.TOKEN_COMMA {
		p.advance()
		node.Right = p.parseLiteral()
	}

	p.expect(lexer.TOKEN_RBRACE)
	return node
}

// 演算子の引数をパース: 識別子またはネストした演算式
func (p *Parser) parseArg() ast.Node {
	switch p.cur().Type {
	case lexer.TOKEN_PLUS, lexer.TOKEN_MINUS, lexer.TOKEN_STAR, lexer.TOKEN_SLASH:
		return p.parseExpr()
	}
	tok := p.cur()
	p.advance()
	return &ast.LiteralNode{Kind: string(tok.Type), Value: tok.Literal}
}

// le(hp,0) / lt(i,10) / eq(a:10)
func (p *Parser) parseCondition() *ast.ConditionNode {
	op := p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_LPAREN)
	left := p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_COMMA)
	right := p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_RPAREN)
	return &ast.ConditionNode{Op: op, Left: left, Right: right}
}
