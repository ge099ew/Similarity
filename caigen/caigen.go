// Package caigen: Similarity AST → CAI テキスト形式 生成
package caigen

import (
	"fmt"
	"similarity/ast"
	"similarity/stdlib"
	"strings"
)

// ===== 定数（enum化） =====
// LiteralNode.Kind
const (
	KindInt    = "INT"
	KindFloat  = "FLOAT"
	KindBool   = "BOOL"
	KindString = "STRING_LIT"
	KindIdent  = "IDENT"
)

// ConditionNode.Op
const (
	OpLess     = "less"
	OpLessEq   = "lesseq"
	OpEqual    = "equal"
	OpNotEqual = "notequal"
	OpGreater  = "greater"
	OpGreaterEq = "greatereq"
)

// ExprNode.Op
const (
	OpAdd = "+"
	OpSub = "-"
	OpMul = "*"
	OpDiv = "/"
)

// CAI命令名
const (
	CAIAlloc   = "alloc"
	CAIStore   = "store"
	CAILoad    = "load"
	CAIAdd     = "add"
	CAISub     = "sub"
	CAIMul     = "mul"
	CAIDiv     = "div"
	CAIClt     = "clt"
	CAICle     = "cle"
	CAICeq     = "ceq"
	CAICne     = "cne"
	CAICgt     = "cgt"
	CAICge     = "cge"
	CAILabel   = "label"
	CAIJmp     = "jmp"
	CAIJnz     = "jnz"
	CAICall    = "call"
	CAIRet     = "ret"
	CAIItof    = "itof"
	CAIFtoi    = "ftoi"
	CAILoadP   = "loadp"
	CAIMov     = "mov"
	CAISyscall = "syscall"
)

// ===== 型サイズ管理 =====

// typeSizes は各型のスタック上のバイトサイズ
// 将来のABI/Target切替に備えてここで一元管理する
var typeSizes = map[string]int{
	"int":    4,
	"float":  4,
	"bool":   4,
	"String": 8, // ポインタサイズ
	"ptr":    8,
	"int64":  8,
}

// sizeOf は型名からバイトサイズを返す
// 未知の型はデフォルト4バイト
func sizeOf(typeName string) int {
	if s, ok := typeSizes[typeName]; ok {
		return s
	}
	return 4
}

// ===== StructInfo =====

// StructInfo は struct型の情報を保持する
// Size/Alignmentは将来のABI対応で使用する
type StructInfo struct {
	Fields    []ast.StructField
	Size      int // 構造体全体のバイトサイズ
	Alignment int // アライメント
}

func newStructInfo(fields []ast.StructField) StructInfo {
	size := 0
	align := 1
	for _, f := range fields {
		fs := sizeOf(f.Type)
		size += fs
		if fs > align {
			align = fs
		}
	}
	return StructInfo{Fields: fields, Size: size, Alignment: align}
}

// ===== CAIGen =====

// CAIGen は AST → CAI IR テキストを生成するコードジェネレータ
type CAIGen struct {
	out      strings.Builder
	tmpIdx   int
	labelIdx int
	structs  map[string]StructInfo
}

// New は CAIGen を生成する
func New() *CAIGen {
	return &CAIGen{
		structs: make(map[string]StructInfo),
	}
}

// ===== Emitter =====

func (c *CAIGen) emit(format string, args ...interface{}) {
	fmt.Fprintf(&c.out, format+"\n", args...)
}

func (c *CAIGen) tmp() string {
	c.tmpIdx++
	return fmt.Sprintf("%%t%d", c.tmpIdx)
}

func (c *CAIGen) label() string {
	c.labelIdx++
	return fmt.Sprintf("lbl%d", c.labelIdx)
}

// ===== Generate（分割済み） =====

// Generate はASTからCAI IRテキストを生成するエントリーポイント
func (c *CAIGen) Generate(prog *ast.Program) string {
	c.emitHeader(prog)
	c.emitImports(prog)
	c.collectStructs(prog)
	c.emitFunctions(prog)
	return c.out.String()
}

// emitHeader はファイル先頭のコメントヘッダを出力する
func (c *CAIGen) emitHeader(prog *ast.Program) {
	if prog.Explanation != nil {
		c.emit("# Similarity - %s", prog.Explanation.Category)
		for k, v := range prog.Explanation.Args {
			c.emit("# %s: %s", k, v)
		}
	}
	c.emit("")
}

// emitImports はextern宣言とstdlib展開を出力する（Pass1）
func (c *CAIGen) emitImports(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *ast.ImportNode:
			if lib, ok := stdlib.AvailableLibsCAI[n.Module]; ok {
				c.emit("# stdlib: %s", n.Module)
				c.out.WriteString(lib)
				c.emit("")
			}
		case *ast.ExternNode:
			for _, fn := range n.Funcs {
				c.emit("extern $%s", fn.Name)
			}
		}
	}
}

// collectStructs はstruct定義を収集する（Pass2）
func (c *CAIGen) collectStructs(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		v, ok := stmt.(*ast.VariableNode)
		if !ok || v.Type != "__struct__" {
			continue
		}
		def, ok := v.Value.(*ast.StructDefNode)
		if !ok {
			continue
		}
		c.structs[def.Name] = newStructInfo(def.Fields)
	}
}

// emitFunctions は関数定義のCAI IRを出力する（Pass3）
func (c *CAIGen) emitFunctions(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*ast.FuncNode); ok {
			c.genFunc(fn)
		}
	}
}

// ===== 関数生成 =====

func (c *CAIGen) genFunc(fn *ast.FuncNode) {
	if fn.Public {
		c.emit("export func $%s", fn.Name)
	} else {
		c.emit("func $%s", fn.Name)
	}

	// 引数をスタックに確保（型サイズ管理を使用）
	for i, param := range fn.Params {
		size := sizeOf(param.Type)
		c.emit("  %s  %%%s.ptr %d", CAIAlloc, param.Name, size)
		c.emit("  %s  %%%s.ptr %%arg%d", CAIStore, param.Name, i)
	}

	// ボディ
	for _, stmt := range fn.Body {
		c.genStmt(stmt, "  ")
	}

	// fn.Returns（旧構文互換）
	if fn.Returns != nil {
		val := c.genExpr(fn.Returns, "  ")
		c.emit("  %s    %s", CAIRet, val)
	}

	c.emit("endfunc")
	c.emit("")
}

// ===== 文生成 =====

func (c *CAIGen) genStmt(node ast.Node, indent string) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.VariableNode:
		if n.Type == "__struct__" {
			return
		}
		size := sizeOf(n.Type)
		c.emit("%s%s  %%%s.ptr %d", indent, CAIAlloc, n.Name, size)
		if n.Value != nil {
			val := c.genExpr(n.Value, indent)
			c.emit("%s%s  %%%s.ptr %s", indent, CAIStore, n.Name, val)
		}

	case *ast.MutationNode:
		if n.Value != nil {
			val := c.genExpr(n.Value, indent)
			c.emit("%s%s  %%%s.ptr %s", indent, CAIStore, n.Name, val)
		}

	case *ast.ReturnNode:
		if n.Value != nil {
			val := c.genExpr(n.Value, indent)
			c.emit("%s%s    %s", indent, CAIRet, val)
		} else {
			c.emit("%s%s", indent, CAIRet)
		}

	case *ast.IfNode:
		c.genIf(n, indent)

	case *ast.LoopNode:
		c.genLoop(n, indent)

	case *ast.CallNode:
		t := c.tmp()
		args := c.genArgs(n.Args, indent)
		c.emit("%s%s   %s $%s%s", indent, CAICall, t, n.FuncName, args)

	case *ast.RawMemNode:
		c.emit("%s# risk block begin", indent)
		for _, s := range n.Body {
			c.genStmt(s, indent)
		}
		c.emit("%s# risk block end", indent)

	case *ast.FatalNode:
		c.emit("%s# Fatal: %s - %s", indent, n.ErrType, n.Msg)
		c.emit("%s%s   %%_ $abort", indent, CAICall)

	case *ast.AsyncNode:
		c.emit("%s# Async block (pthread)", indent)
		for _, s := range n.Body {
			c.genStmt(s, indent)
		}

	case *ast.BreakNode:
		c.emit("%s%s    __break__", indent, CAIJmp)

	case *ast.ContinueNode:
		c.emit("%s%s    __continue__", indent, CAIJmp)
	}
}

// ===== If生成 =====

func (c *CAIGen) genIf(n *ast.IfNode, indent string) {
	cond := c.genCond(n.Condition, indent)
	lTrue := c.label()
	lFalse := c.label()
	lEnd := c.label()

	c.emit("%s%s    %s %s %s", indent, CAIJnz, cond, lTrue, lFalse)

	c.emit("%s%s  %s", indent, CAILabel, lTrue)
	for _, s := range n.True {
		c.genStmt(s, indent+"  ")
	}
	c.emit("%s%s    %s", indent, CAIJmp, lEnd)

	c.emit("%s%s  %s", indent, CAILabel, lFalse)
	for _, s := range n.False {
		c.genStmt(s, indent+"  ")
	}

	c.emit("%s%s  %s", indent, CAILabel, lEnd)
}

// ===== Loop生成 =====

func (c *CAIGen) genLoop(n *ast.LoopNode, indent string) {
	lStart := c.label()
	lBody := c.label()
	lEnd := c.label()

	if n.Init != nil {
		c.genStmt(n.Init, indent)
	}

	c.emit("%s%s  %s", indent, CAILabel, lStart)

	if n.Condition != nil {
		cond := c.genCond(n.Condition, indent)
		c.emit("%s%s    %s %s %s", indent, CAIJnz, cond, lBody, lEnd)
	}

	c.emit("%s%s  %s", indent, CAILabel, lBody)
	for _, s := range n.Body {
		c.genStmt(s, indent+"  ")
	}

	// Step生成（将来的にはStepNodeで柔軟化予定）
	if n.Step != 0 {
		if n.Init != nil {
			if v, ok := n.Init.(*ast.VariableNode); ok {
				dst := c.tmp()
				t := c.tmp()
				c.emit("%s%s   %s %%%s.ptr", indent, CAILoad, dst, v.Name)
				c.emit("%s%s    %s %s %d", indent, CAIAdd, t, dst, n.Step)
				c.emit("%s%s  %%%s.ptr %s", indent, CAIStore, v.Name, t)
			}
		}
	}

	c.emit("%s%s    %s", indent, CAIJmp, lStart)
	c.emit("%s%s  %s", indent, CAILabel, lEnd)
}

// ===== 条件生成 =====

func (c *CAIGen) genCond(node ast.Node, indent string) string {
	cond, ok := node.(*ast.ConditionNode)
	if !ok {
		return c.genExpr(node, indent)
	}
	left := c.loadStrVal(cond.Left, indent)
	right := c.loadStrVal(cond.Right, indent)
	dst := c.tmp()

	var op string
	switch cond.Op {
	case OpLess:
		op = CAIClt
	case OpLessEq:
		op = CAICle
	case OpEqual:
		op = CAICeq
	case OpNotEqual:
		op = CAICne
	case OpGreater:
		op = CAICgt
	case OpGreaterEq:
		op = CAICge
	default:
		op = CAICeq
	}
	c.emit("%s%s    %s %s %s", indent, op, dst, left, right)
	return dst
}

// loadStrVal は変数名または数値リテラルをCAI値として返す
func (c *CAIGen) loadStrVal(s string, indent string) string {
	if len(s) > 0 && (s[0] >= '0' && s[0] <= '9' || s[0] == '-') {
		return s
	}
	dst := c.tmp()
	c.emit("%s%s   %s %%%s.ptr", indent, CAILoad, dst, s)
	return dst
}

// ===== 式生成 =====

func (c *CAIGen) genExpr(node ast.Node, indent string) string {
	if node == nil {
		return "0"
	}
	switch n := node.(type) {
	case *ast.LiteralNode:
		if n.Kind == KindIdent {
			dst := c.tmp()
			c.emit("%s%s   %s %%%s.ptr", indent, CAILoad, dst, n.Value)
			return dst
		}
		return n.Value

	case *ast.ExprNode:
		left := c.genExprLoad(n.Left, indent)
		right := c.genExprLoad(n.Right, indent)
		dst := c.tmp()
		var op string
		switch n.Op {
		case OpAdd:
			op = CAIAdd
		case OpSub:
			op = CAISub
		case OpMul:
			op = CAIMul
		case OpDiv:
			op = CAIDiv
		}
		c.emit("%s%s    %s %s %s", indent, op, dst, left, right)
		return dst

	case *ast.CallNode:
		dst := c.tmp()
		args := c.genArgs(n.Args, indent)
		c.emit("%s%s   %s $%s%s", indent, CAICall, dst, n.FuncName, args)
		return dst

	case *ast.CastNode:
		src := c.genExprLoad(n.Value, indent)
		dst := c.tmp()
		if n.Type == "float" {
			c.emit("%s%s   %s %s", indent, CAIItof, dst, src)
		} else {
			c.emit("%s%s   %s %s", indent, CAIFtoi, dst, src)
		}
		return dst

	case *ast.AddressNode:
		dst := c.tmp()
		c.emit("%s%s    %s %%%s.ptr", indent, CAIMov, dst, n.Name)
		return dst

	case *ast.DerefNode:
		ptr := c.tmp()
		c.emit("%s%s  %s %%%s.ptr", indent, CAILoadP, ptr, n.Name)
		dst := c.tmp()
		c.emit("%s%s   %s %s", indent, CAILoad, dst, ptr)
		return dst

	case *ast.IndexNode:
		idx := c.genExprLoad(n.Index, indent)
		dst := c.tmp()
		c.emit("%s%s   %s %%%s.ptr[%s]", indent, CAILoad, dst, n.Name, idx)
		return dst
	}
	return "0"
}

func (c *CAIGen) genExprLoad(node ast.Node, indent string) string {
	if lit, ok := node.(*ast.LiteralNode); ok {
		if lit.Kind == KindInt || lit.Kind == KindFloat {
			return lit.Value
		}
	}
	return c.genExpr(node, indent)
}

func (c *CAIGen) genArgs(args []ast.Node, indent string) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for _, arg := range args {
		v := c.genExprLoad(arg, indent)
		parts = append(parts, v)
	}
	return " " + strings.Join(parts, " ")
}
