package parser

import (
	"fmt"
	"similarity/ast"
	"similarity/lexer"
	"strconv"
	"strings"
	"sync"
)

type Parser struct {
	tokens []lexer.Token
	pos    int
	Errors []string
	arena  *ast.Arena // ← 追加
}

func New(tokens []lexer.Token) *Parser {
	return &Parser{
		tokens: tokens,
		pos:    0,
		arena:  ast.NewArena(),
	}
}

func (p *Parser) cur() lexer.Token { return p.tokens[p.pos] }

// curPos は現在トークンの位置情報を ast.Pos として返す
func (p *Parser) curPos() ast.Pos {
	t := p.tokens[p.pos]
	return ast.Pos{Line: t.Line, Col: t.Col}
}

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

	// トップレベルの文を先に全部収集
	var rawStmts []lexer.Token
	startPositions := []int{}
	for p.cur().Type != lexer.TOKEN_EOF {
		startPositions = append(startPositions, p.pos)
		p.skipTopLevel() // 一つのトップレベル文をスキップ
	}

	// goroutineで並列パース
	results := make([]ast.Node, len(startPositions))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, startPos := range startPositions {
		wg.Add(1)
		go func(idx, pos int) {
			defer wg.Done()
			subParser := New(p.tokens[pos:])
			stmt := subParser.parseStatement()
			mu.Lock()
			results[idx] = stmt
			if len(subParser.Errors) > 0 {
				p.Errors = append(p.Errors, subParser.Errors...)
			}
			mu.Unlock()
		}(i, startPos)
	}
	wg.Wait()

	for _, stmt := range results {
		if stmt != nil {
			prog.Statements = append(prog.Statements, stmt)
		}
	}

	_ = rawStmts
	return prog
}

// トップレベルの文を一つスキップ
func (p *Parser) skipTopLevel() {
	depth := 0
	for p.cur().Type != lexer.TOKEN_EOF {
		switch p.cur().Type {
		case lexer.TOKEN_LBRACKET, lexer.TOKEN_LBRACE, lexer.TOKEN_LPAREN:
			depth++
		case lexer.TOKEN_RBRACKET, lexer.TOKEN_RBRACE, lexer.TOKEN_RPAREN:
			depth--
			if depth == 0 {
				p.advance()
				return
			}
		}
		p.advance()
	}
}

func (p *Parser) parseStatement() ast.Node {
	switch p.cur().Type {
	case lexer.TOKEN_VARIABLE:
		return p.parseVariable()
	case lexer.TOKEN_MUTATION:
		return p.parseMutation()
	case lexer.TOKEN_IF:
		return p.parseIf()
	case lexer.TOKEN_RETURN:
		return p.parseReturn()
	case lexer.TOKEN_LOOP:
		return p.parseLoop()
	case lexer.TOKEN_FUNC:
		return p.parseFunc(false)
	case lexer.TOKEN_FUNC_PUBLIC:
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
	case lexer.TOKEN_ASYNC:
		return p.parseAsync()
	case lexer.TOKEN_SHARE:
		return p.parseShare()
	case lexer.TOKEN_AWAIT:
		return p.parseAwait()
	case lexer.TOKEN_GPU:
		return p.parseGPU()
	case lexer.TOKEN_MEM:
		return p.parseMem()
	case lexer.TOKEN_BREAK:
		return p.parseBreak()
	case lexer.TOKEN_CONTINUE:
		return p.parseContinue()
	case lexer.TOKEN_INC, lexer.TOKEN_DEC:
		return p.parseIncr()
	case lexer.TOKEN_CAST:
		return p.parseCast()
	case lexer.TOKEN_INDEX:
		return p.parseIndex()
	case lexer.TOKEN_ADDR:
		return p.parseAddress()
	case lexer.TOKEN_DEREF:
		return p.parseDeref()
	default:
		p.errorf("文として解釈できません")
		p.advance()
		return nil
	}
}

// Explanation[Application{Game(type:RPG)}]
func (p *Parser) parseExplanation() *ast.ExplanationNode {
	pos := p.curPos()
	p.advance() // skip Explanation
	p.expect(lexer.TOKEN_LBRACKET)

	node := &ast.ExplanationNode{Args: make(map[string]string), Pos: pos}
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

// Mutation[variable{int(x:30)}]
// Mutation[array{int(arr:i:val)}]
func (p *Parser) parseMutation() ast.Node {
	pos := p.curPos()
	p.advance() // skip Mutation
	p.expect(lexer.TOKEN_LBRACKET)

	// array書き込み: Mutation[array{int(arr:i:val)}]
	if p.cur().Type == lexer.TOKEN_ARRAY {
		p.advance() // skip array
		p.expect(lexer.TOKEN_LBRACE)
		elemType := p.cur().Literal
		p.advance()
		p.expect(lexer.TOKEN_LPAREN)
		arrName := p.cur().Literal
		p.advance()
		p.expect(lexer.TOKEN_COLON)
		idx := p.parseLiteral()
		p.expect(lexer.TOKEN_COLON)
		val := p.parseLiteral()
		p.expect(lexer.TOKEN_RPAREN)
		p.expect(lexer.TOKEN_RBRACE)
		p.expect(lexer.TOKEN_RBRACKET)
		return &ast.ArrayStoreNode{
			ElemType: elemType,
			Name:     arrName,
			Index:    idx,
			Value:    val,
			Pos:      pos,
		}
	}

	// variable をスキップ
	if p.cur().Type == lexer.TOKEN_IDENT ||
		p.cur().Type == lexer.TOKEN_VARIABLE_KEY {
		p.advance()
	}
	p.expect(lexer.TOKEN_LBRACE)

	node := &ast.MutationNode{Pos: pos}
	node.Type = p.cur().Literal
	p.advance() // int
	p.expect(lexer.TOKEN_LPAREN)
	node.Name = p.cur().Literal
	p.advance() // x
	p.expect(lexer.TOKEN_COLON)
	node.Value = p.parseLiteral()
	p.expect(lexer.TOKEN_RPAREN)
	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Variable[let{int(x:10)}] / Variable[unclet{float(PI:3.14)}]
// Variable[struct{User:String(name), int(age)}]
// Variable[let{user:User(name:"John", age:25)}]
func (p *Parser) parseVariable() *ast.VariableNode {
	pos := p.curPos()
	p.advance() // skip Variable
	p.expect(lexer.TOKEN_LBRACKET)

	// struct定義: Variable[struct{...}]
	if p.cur().Type == lexer.TOKEN_STRUCT {
		return p.parseStructDef()
	}

	mutable := p.cur().Type == lexer.TOKEN_LET
	p.advance() // skip let/unclet
	p.expect(lexer.TOKEN_LBRACE)

	// 型名を読む
	typeName := p.cur().Literal
	p.advance()

	// 配列宣言: Variable[let{Array_int(arr:N)}]
	if strings.HasPrefix(typeName, "Array_") {
		_ = strings.TrimPrefix(typeName, "Array_") // 型情報はcaigenで使用
		p.expect(lexer.TOKEN_LPAREN)
		arrName := p.cur().Literal
		p.advance()
		p.expect(lexer.TOKEN_COLON)
		sizeVal, _ := strconv.Atoi(p.cur().Literal)
		p.advance()
		p.expect(lexer.TOKEN_RPAREN)
		p.expect(lexer.TOKEN_RBRACE)
		p.expect(lexer.TOKEN_RBRACKET)
		return p.arena.Add(&ast.VariableNode{
			Mutable: mutable,
			Type:    typeName,
			Name:    arrName,
			Value:   &ast.LiteralNode{Kind: "INT_LIT", Value: strconv.Itoa(sizeVal)},
			Pos:     pos,
		}).(*ast.VariableNode)
	}

	p.expect(lexer.TOKEN_LPAREN)
	varName := p.cur().Literal
	p.advance() // 変数名
	p.expect(lexer.TOKEN_COLON)

	// 次のトークンが識別子で、その後に ( が来る場合はstructインスタンス
	// e.g. User(name:"John", age:25)
	if p.cur().Type == lexer.TOKEN_IDENT && p.peek().Type == lexer.TOKEN_LPAREN {
		si := p.parseStructInstance()
		p.expect(lexer.TOKEN_RPAREN)
		p.expect(lexer.TOKEN_RBRACE)
		p.expect(lexer.TOKEN_RBRACKET)
		// StructInstanceをVariableNodeのValueとして包む
		wrapper := p.arena.Add(&ast.VariableNode{
			Mutable: mutable,
			Type:    typeName,
			Name:    varName,
			Value:   si,
			Pos:     pos,
		}).(*ast.VariableNode)
		return wrapper
	}

	node := p.arena.Add(&ast.VariableNode{Mutable: mutable, Pos: pos}).(*ast.VariableNode)
	node.Type = typeName
	node.Name = varName
	node.Value = p.parseLiteral()
	p.expect(lexer.TOKEN_RPAREN)

	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Variable[struct{User:String(name), int(age)}]
// → VariableNode{Type:"__struct__", Name:"User", Value:StructDefNode{...}}
func (p *Parser) parseStructDef() *ast.VariableNode {
	p.advance() // skip struct
	p.expect(lexer.TOKEN_LBRACE)

	// 構造体名: e.g. "User"
	structName := p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_COLON)

	var fields []ast.StructField
	// フィールドをカンマ区切りで読む: String(name), int(age)
	for p.cur().Type != lexer.TOKEN_RBRACE && p.cur().Type != lexer.TOKEN_EOF {
		fieldType := p.cur().Literal
		p.advance()
		p.expect(lexer.TOKEN_LPAREN)
		fieldName := p.cur().Literal
		p.advance()
		p.expect(lexer.TOKEN_RPAREN)
		fields = append(fields, ast.StructField{Type: fieldType, Name: fieldName})
		if p.cur().Type == lexer.TOKEN_COMMA {
			p.advance()
		}
	}

	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_RBRACKET)

	def := &ast.StructDefNode{Name: structName, Fields: fields}
	return p.arena.Add(&ast.VariableNode{
		Mutable: false,
		Type:    "__struct__",
		Name:    structName,
		Value:   def,
	}).(*ast.VariableNode)
}

// User(name:"John", age:25) → StructInstanceNode
func (p *Parser) parseStructInstance() *ast.StructInstanceNode {
	typeName := p.cur().Literal
	p.advance() // skip type name
	p.expect(lexer.TOKEN_LPAREN)

	si := &ast.StructInstanceNode{TypeName: typeName}
	for p.cur().Type != lexer.TOKEN_RPAREN && p.cur().Type != lexer.TOKEN_EOF {
		fieldName := p.cur().Literal
		p.advance()
		p.expect(lexer.TOKEN_COLON)
		val := p.parseLiteral()
		si.Fields = append(si.Fields, ast.FieldValue{Name: fieldName, Value: val})
		if p.cur().Type == lexer.TOKEN_COMMA {
			p.advance()
		}
	}
	p.expect(lexer.TOKEN_RPAREN)
	return si
}

// If[check{le(hp,0)}, True[...], False[...]]
func (p *Parser) parseIf() *ast.IfNode {
	pos := p.curPos()
	p.advance() // skip If
	p.expect(lexer.TOKEN_LBRACKET)

	node := &ast.IfNode{Pos: pos}
	p.expect(lexer.TOKEN_CHECK)
	p.expect(lexer.TOKEN_LBRACE)
	node.Condition = p.parseCondition()
	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_COMMA)

	// True{...}
	p.expect(lexer.TOKEN_TRUE)
	p.expect(lexer.TOKEN_LBRACE)
	node.True = p.parseBlock()
	p.expect(lexer.TOKEN_RBRACE)

	// False{...} (任意)
	if p.cur().Type == lexer.TOKEN_COMMA {
		p.advance()
	}
	if p.cur().Type == lexer.TOKEN_FALSE {
		p.advance()
		p.expect(lexer.TOKEN_LBRACE)
		node.False = p.parseBlock()
		p.expect(lexer.TOKEN_RBRACE)
	}

	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Loop[check{less(i,10)}, for{...}]
func (p *Parser) parseLoop() *ast.LoopNode {
	pos := p.curPos()
	p.advance() // skip Loop
	p.expect(lexer.TOKEN_LBRACKET)

	node := p.arena.Add(&ast.LoopNode{Pos: pos}).(*ast.LoopNode)
	node.Kind = "for"

	// check{条件}
	p.expect(lexer.TOKEN_CHECK)
	p.expect(lexer.TOKEN_LBRACE)
	node.Condition = p.parseCondition()
	p.expect(lexer.TOKEN_RBRACE)

	p.expect(lexer.TOKEN_COMMA)

	// for{本体}
	p.expect(lexer.TOKEN_FOR)
	p.expect(lexer.TOKEN_LBRACE)
	node.Body = p.parseBlock()
	p.expect(lexer.TOKEN_RBRACE)

	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Func[name{receive{...}, 処理, return{...}}] / Func_pub[...]
func (p *Parser) parseFunc(pub bool) *ast.FuncNode {
	pos := p.curPos()
	p.advance() // skip Func
	p.expect(lexer.TOKEN_LBRACKET)

	node := p.arena.Add(&ast.FuncNode{Public: pub, Pos: pos}).(*ast.FuncNode)
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
	pos := p.curPos()
	p.advance() // skip Error
	p.expect(lexer.TOKEN_LBRACKET)

	// def{TypeName} の場合
	if p.cur().Type == lexer.TOKEN_DEF {
		p.advance()
		p.expect(lexer.TOKEN_LBRACE)
		node := &ast.ErrorNode{ErrType: p.cur().Literal, Pos: pos}
		p.advance()
		p.expect(lexer.TOKEN_RBRACE)
		p.expect(lexer.TOKEN_RBRACKET)
		return node
	}

	node := p.arena.Add(&ast.ErrorNode{Pos: pos}).(*ast.ErrorNode)
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
	pos := p.curPos()
	p.advance() // skip Fatal
	p.expect(lexer.TOKEN_LBRACKET)
	node := p.arena.Add(&ast.FatalNode{Pos: pos}).(*ast.FatalNode)
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
	pos := p.curPos()
	p.advance() // skip Import
	p.expect(lexer.TOKEN_LBRACKET)
	node := p.arena.Add(&ast.ImportNode{Module: p.cur().Literal, Pos: pos}).(*ast.ImportNode)
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
	pos := p.curPos()
	p.advance() // skip Extern
	p.expect(lexer.TOKEN_LBRACKET)
	p.advance() // skip C
	p.expect(lexer.TOKEN_LBRACE)
	node := p.arena.Add(&ast.ExternNode{Pos: pos}).(*ast.ExternNode)
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
	pos := p.curPos()
	p.advance() // skip call
	p.expect(lexer.TOKEN_LBRACE)
	node := p.arena.Add(&ast.CallNode{FuncName: p.cur().Literal, Pos: pos}).(*ast.CallNode)
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
	pos := p.curPos()
	p.advance() // skip return
	p.expect(lexer.TOKEN_LPAREN)
	node := p.arena.Add(&ast.ReturnNode{Pos: pos}).(*ast.ReturnNode)
	if p.cur().Type != lexer.TOKEN_RPAREN {
		node.Value = p.parseLiteral()
	}
	p.expect(lexer.TOKEN_RPAREN)
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
	node := p.arena.Add(&ast.VariableNode{Type: p.cur().Literal, Mutable: true}).(*ast.VariableNode)
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
	// ただし -5 や -3.14 のような負数リテラルは演算式ではなくリテラルとして扱う
	switch p.cur().Type {
	case lexer.TOKEN_MINUS:
		next := p.peek()
		if next.Type == lexer.TOKEN_INT_LIT || next.Type == lexer.TOKEN_FLOAT_LIT {
			p.advance() // skip -
			tok := p.cur()
			p.advance()
			return &ast.LiteralNode{Kind: string(tok.Type), Value: "-" + tok.Literal, Line: tok.Line, Pos: ast.Pos{Line: tok.Line, Col: tok.Col}}
		}
		return p.parseExpr()
	case lexer.TOKEN_PLUS, lexer.TOKEN_STAR, lexer.TOKEN_SLASH:
		return p.parseExpr()
	case lexer.TOKEN_CALL:
		return p.parseCall()
	case lexer.TOKEN_ADDR:
		return p.parseAddress()
	case lexer.TOKEN_DEREF:
		return p.parseDeref()
	case lexer.TOKEN_CAST:
		return p.parseCast()
	case lexer.TOKEN_INDEX:
		return p.parseIndex()
	}
	tok := p.cur()
	p.advance()
	return &ast.LiteralNode{Kind: string(tok.Type), Value: tok.Literal, Line: tok.Line, Pos: ast.Pos{Line: tok.Line, Col: tok.Col}}
}

// 演算式をパース: +{int(a, b)} → ExprNode{Op:"+", Type:"int", Left:a, Right:b}
func (p *Parser) parseExpr() *ast.ExprNode {
	pos := p.curPos()
	op := p.cur().Literal
	p.advance() // skip + - * /
	p.expect(lexer.TOKEN_LBRACE)

	node := p.arena.Add(&ast.ExprNode{Op: op, Pos: pos}).(*ast.ExprNode)

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

	p.expect(lexer.TOKEN_RBRACE)
	return node
}

// 演算子の引数をパース: 識別子またはネストした演算式
func (p *Parser) parseArg() ast.Node {
	switch p.cur().Type {
	case lexer.TOKEN_MINUS:
		next := p.peek()
		if next.Type == lexer.TOKEN_INT_LIT || next.Type == lexer.TOKEN_FLOAT_LIT {
			p.advance() // skip -
			tok := p.cur()
			p.advance()
			return &ast.LiteralNode{Kind: string(tok.Type), Value: "-" + tok.Literal}
		}
		return p.parseExpr()
	case lexer.TOKEN_PLUS, lexer.TOKEN_STAR, lexer.TOKEN_SLASH:
		return p.parseExpr()
	}
	tok := p.cur()
	p.advance()
	return &ast.LiteralNode{Kind: string(tok.Type), Value: tok.Literal}
}

// le(hp,0) / lt(i,10) / eq(a:10)
func (p *Parser) parseCondition() *ast.ConditionNode {
	pos := p.curPos()
	op := p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_LPAREN)
	left := p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_COLON) // カンマ→コロン
	right := p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_RPAREN)
	return &ast.ConditionNode{Op: op, Left: left, Right: right, Pos: pos}
}

// Async[{処理}]
func (p *Parser) parseAsync() *ast.AsyncNode {
	pos := p.curPos()
	p.advance() // skip Async
	p.expect(lexer.TOKEN_LBRACKET)
	p.expect(lexer.TOKEN_LBRACE)
	node := p.arena.Add(&ast.AsyncNode{Pos: pos}).(*ast.AsyncNode)
	node.Body = p.parseBlock()
	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Await[task]
func (p *Parser) parseAwait() *ast.AwaitNode {
	pos := p.curPos()
	p.advance() // skip Await
	p.expect(lexer.TOKEN_LBRACKET)
	node := p.arena.Add(&ast.AwaitNode{Target: p.cur().Literal, Pos: pos}).(*ast.AwaitNode)
	p.advance()
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// GPU[{処理}]
func (p *Parser) parseGPU() *ast.GPUNode {
	pos := p.curPos()
	p.advance() // skip GPU
	p.expect(lexer.TOKEN_LBRACKET)
	p.expect(lexer.TOKEN_LBRACE)
	node := p.arena.Add(&ast.GPUNode{Pos: pos}).(*ast.GPUNode)
	node.Body = p.parseBlock()
	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// Mem[risk{...}] / Mem[Raw{...}]
func (p *Parser) parseMem() *ast.RawMemNode {
	pos := p.curPos()
	p.advance() // skip Mem
	p.expect(lexer.TOKEN_LBRACKET)
	p.advance() // skip risk/Raw
	p.expect(lexer.TOKEN_LBRACE)
	lineStart := p.cur().Line
	node := p.arena.Add(&ast.RawMemNode{Pos: pos}).(*ast.RawMemNode)
	node.LineStart = lineStart
	node.Body = p.parseBlock()
	node.LineEnd = p.cur().Line
	// ブロック内のunsafe操作を収集
	node.Ops = collectRiskOps(node.Body)
	p.expect(lexer.TOKEN_RBRACE)
	p.expect(lexer.TOKEN_RBRACKET)
	return node
}

// riskブロック内のunsafe操作を収集
func collectRiskOps(body []ast.Node) []string {
	opsMap := map[string]bool{}
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		if n == nil {
			return
		}
		switch v := n.(type) {
		case *ast.DerefNode:
			opsMap["deref"] = true
		case *ast.AddressNode:
			opsMap["addr"] = true
		case *ast.VariableNode:
			walk(v.Value)
		case *ast.MutationNode:
			walk(v.Value)
		case *ast.RawMemNode:
			for _, s := range v.Body {
				walk(s)
			}
		}
	}
	for _, s := range body {
		walk(s)
	}
	var ops []string
	for op := range opsMap {
		ops = append(ops, op)
	}
	return ops
}

// share(x) → Async間で共有する変数を明示
func (p *Parser) parseShare() *ast.ShareNode {
	pos := p.curPos()
	p.advance() // skip share
	p.expect(lexer.TOKEN_LPAREN)
	name := p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_RPAREN)
	return &ast.ShareNode{Name: name, Pos: pos}
}

// ++{i} / --{i}
func (p *Parser) parseIncr() *ast.IncrNode {
	pos := p.curPos()
	op := p.cur().Literal // "++" or "--"
	p.advance()
	p.expect(lexer.TOKEN_LBRACE)
	name := p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_RBRACE)
	return &ast.IncrNode{Name: name, Op: op, Pos: pos}
}

// break{}
func (p *Parser) parseBreak() *ast.BreakNode {
	pos := p.curPos()
	p.advance() // skip break
	p.expect(lexer.TOKEN_LBRACE)
	p.expect(lexer.TOKEN_RBRACE)
	return &ast.BreakNode{Pos: pos}
}

// continue{}
func (p *Parser) parseContinue() *ast.ContinueNode {
	pos := p.curPos()
	p.advance() // skip continue
	p.expect(lexer.TOKEN_LBRACE)
	p.expect(lexer.TOKEN_RBRACE)
	return &ast.ContinueNode{Pos: pos}
}

// cast{int(x)}
func (p *Parser) parseCast() *ast.CastNode {
	pos := p.curPos()
	p.advance() // skip cast
	p.expect(lexer.TOKEN_LBRACE)
	node := p.arena.Add(&ast.CastNode{Pos: pos}).(*ast.CastNode)
	node.Type = p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_LPAREN)
	node.Value = p.parseLiteral()
	p.expect(lexer.TOKEN_RPAREN)
	p.expect(lexer.TOKEN_RBRACE)
	return node
}

// index{arr(i)}
func (p *Parser) parseIndex() *ast.IndexNode {
	pos := p.curPos()
	p.advance() // skip index
	p.expect(lexer.TOKEN_LBRACE)
	node := p.arena.Add(&ast.IndexNode{Pos: pos}).(*ast.IndexNode)
	node.Name = p.cur().Literal
	p.advance()
	p.expect(lexer.TOKEN_LPAREN)
	node.Index = p.parseLiteral()
	p.expect(lexer.TOKEN_RPAREN)
	p.expect(lexer.TOKEN_RBRACE)
	return node
}

// addr{x}
func (p *Parser) parseAddress() *ast.AddressNode {
	pos := p.curPos()
	p.advance() // skip addr
	p.expect(lexer.TOKEN_LBRACE)
	node := p.arena.Add(&ast.AddressNode{Name: p.cur().Literal, Pos: pos}).(*ast.AddressNode)
	p.advance()
	p.expect(lexer.TOKEN_RBRACE)
	return node
}

// deref{ptr}
func (p *Parser) parseDeref() *ast.DerefNode {
	pos := p.curPos()
	p.advance() // skip deref
	p.expect(lexer.TOKEN_LBRACE)
	node := p.arena.Add(&ast.DerefNode{Name: p.cur().Literal, Pos: pos}).(*ast.DerefNode)
	p.advance()
	p.expect(lexer.TOKEN_RBRACE)
	return node
}
