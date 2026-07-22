// Package caigen: Similarity AST → CAI テキスト形式 生成
package caigen

import (
	"fmt"
	"similarity/ast"
	"similarity/stdlib"
	"strings"
)

type CAIGen struct {
	out     strings.Builder
	tmpIdx  int
	labelIdx int
	structs map[string][]ast.StructField
}

func New() *CAIGen {
	return &CAIGen{
		structs: make(map[string][]ast.StructField),
	}
}

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

func (c *CAIGen) Generate(prog *ast.Program) string {
	// コメントヘッダ
	if prog.Explanation != nil {
		c.emit("# Similarity - %s", prog.Explanation.Category)
		for k, v := range prog.Explanation.Args {
			c.emit("# %s: %s", k, v)
		}
	}
	c.emit("")

	// Pass1: extern宣言 + stdlib展開
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

	// Pass2: struct定義収集
	for _, stmt := range prog.Statements {
		if v, ok := stmt.(*ast.VariableNode); ok && v.Type == "__struct__" {
			if def, ok := v.Value.(*ast.StructDefNode); ok {
				c.structs[def.Name] = def.Fields
			}
		}
	}

	// Pass3: 関数生成
	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *ast.FuncNode:
			c.genFunc(n)
		}
	}

	return c.out.String()
}

func (c *CAIGen) genFunc(fn *ast.FuncNode) {
	if fn.Public {
		c.emit("export func $%s", fn.Name)
	} else {
		c.emit("func $%s", fn.Name)
	}

	// 引数をスタックに確保
	for i, param := range fn.Params {
		c.emit("  alloc  %%%s.ptr 4", param.Name)
		c.emit("  store  %%%s.ptr %%arg%d", param.Name, i)
	}

	// ボディ
	for _, stmt := range fn.Body {
		c.genStmt(stmt, "  ")
	}

	// fn.Returns（旧構文互換）
	if fn.Returns != nil {
		val := c.genExpr(fn.Returns, "  ")
		c.emit("  ret    %s", val)
	}

	c.emit("endfunc")
	c.emit("")
}

func (c *CAIGen) genStmt(node ast.Node, indent string) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.VariableNode:
		if n.Type == "__struct__" {
			return
		}
		c.emit("%salloc  %%%s.ptr 4", indent, n.Name)
		if n.Value != nil {
			val := c.genExpr(n.Value, indent)
			c.emit("%sstore  %%%s.ptr %s", indent, n.Name, val)
		}

	case *ast.MutationNode:
		if n.Value != nil {
			val := c.genExpr(n.Value, indent)
			c.emit("%sstore  %%%s.ptr %s", indent, n.Name, val)
		}

	case *ast.ReturnNode:
		if n.Value != nil {
			val := c.genExpr(n.Value, indent)
			c.emit("%sret    %s", indent, val)
		} else {
			c.emit("%sret", indent)
		}

	case *ast.IfNode:
		c.genIf(n, indent)

	case *ast.LoopNode:
		c.genLoop(n, indent)

	case *ast.CallNode:
		t := c.tmp()
		args := c.genArgs(n.Args, indent)
		c.emit("%scall   %s $%s%s", indent, t, n.FuncName, args)

	case *ast.RawMemNode:
		c.emit("%s# risk block begin", indent)
		for _, s := range n.Body {
			c.genStmt(s, indent)
		}
		c.emit("%s# risk block end", indent)

	case *ast.FatalNode:
		c.emit("%s# Fatal: %s - %s", indent, n.ErrType, n.Msg)
		c.emit("%scall   %%_ $abort", indent)

	case *ast.AsyncNode:
		c.emit("%s# Async block (pthread)", indent)
		for _, s := range n.Body {
			c.genStmt(s, indent)
		}

	case *ast.BreakNode:
		c.emit("%sjmp    __break__", indent)

	case *ast.ContinueNode:
		c.emit("%sjmp    __continue__", indent)
	}
}

func (c *CAIGen) genIf(n *ast.IfNode, indent string) {
	cond := c.genCond(n.Condition, indent)
	lTrue := c.label()
	lFalse := c.label()
	lEnd := c.label()

	c.emit("%sjnz    %s %s %s", indent, cond, lTrue, lFalse)

	c.emit("%slabel  %s", indent, lTrue)
	for _, s := range n.True {
		c.genStmt(s, indent+"  ")
	}
	c.emit("%sjmp    %s", indent, lEnd)

	c.emit("%slabel  %s", indent, lFalse)
	for _, s := range n.False {
		c.genStmt(s, indent+"  ")
	}

	c.emit("%slabel  %s", indent, lEnd)
}

func (c *CAIGen) genLoop(n *ast.LoopNode, indent string) {
	lStart := c.label()
	lBody := c.label()
	lEnd := c.label()

	if n.Init != nil {
		c.genStmt(n.Init, indent)
	}

	c.emit("%slabel  %s", indent, lStart)

	if n.Condition != nil {
		cond := c.genCond(n.Condition, indent)
		c.emit("%sjnz    %s %s %s", indent, cond, lBody, lEnd)
	}

	c.emit("%slabel  %s", indent, lBody)
	for _, s := range n.Body {
		c.genStmt(s, indent+"  ")
	}

	if n.Step != 0 {
		// Step は int なので直接加算命令を生成
		if n.Init != nil {
			if v, ok := n.Init.(*ast.VariableNode); ok {
				dst := c.tmp()
				t := c.tmp()
				c.emit("%sload   %s %%%s.ptr", indent, dst, v.Name)
				c.emit("%sadd    %s %s %d", indent, t, dst, n.Step)
				c.emit("%sstore  %%%s.ptr %s", indent, v.Name, t)
			}
		}
	}

	c.emit("%sjmp    %s", indent, lStart)
	c.emit("%slabel  %s", indent, lEnd)
}

func (c *CAIGen) genCond(node ast.Node, indent string) string {
	cond, ok := node.(*ast.ConditionNode)
	if !ok {
		return c.genExpr(node, indent)
	}
	// ConditionNode.Left と Right は string（変数名またはリテラル）
	left := c.loadStrVal(cond.Left, indent)
	right := c.loadStrVal(cond.Right, indent)
	dst := c.tmp()
	switch cond.Op {
	case "less":
		c.emit("%sclt    %s %s %s", indent, dst, left, right)
	case "lesseq":
		c.emit("%scle    %s %s %s", indent, dst, left, right)
	case "equal":
		c.emit("%sceq    %s %s %s", indent, dst, left, right)
	case "notequal":
		c.emit("%scne    %s %s %s", indent, dst, left, right)
	case "greater":
		c.emit("%scgt    %s %s %s", indent, dst, left, right)
	case "greatereq":
		c.emit("%scge    %s %s %s", indent, dst, left, right)
	}
	return dst
}

// loadStrVal: 文字列（変数名またはリテラル）をCAI値として返す
func (c *CAIGen) loadStrVal(s string, indent string) string {
	// 数値リテラルならそのまま
	if len(s) > 0 && (s[0] >= '0' && s[0] <= '9' || s[0] == '-') {
		return s
	}
	// 変数名 → load
	dst := c.tmp()
	c.emit("%sload   %s %%%s.ptr", indent, dst, s)
	return dst
}

func (c *CAIGen) genExpr(node ast.Node, indent string) string {
	if node == nil {
		return "0"
	}
	switch n := node.(type) {
	case *ast.LiteralNode:
		if n.Kind == "IDENT" {
			// 変数参照 → load
			dst := c.tmp()
			c.emit("%sload   %s %%%s.ptr", indent, dst, n.Value)
			return dst
		}
		return n.Value

	case *ast.ExprNode:
		left := c.genExprLoad(n.Left, indent)
		right := c.genExprLoad(n.Right, indent)
		dst := c.tmp()
		switch n.Op {
		case "+":
			c.emit("%sadd    %s %s %s", indent, dst, left, right)
		case "-":
			c.emit("%ssub    %s %s %s", indent, dst, left, right)
		case "*":
			c.emit("%smul    %s %s %s", indent, dst, left, right)
		case "/":
			c.emit("%sdiv    %s %s %s", indent, dst, left, right)
		}
		return dst

	case *ast.CallNode:
		dst := c.tmp()
		args := c.genArgs(n.Args, indent)
		c.emit("%scall   %s $%s%s", indent, dst, n.FuncName, args)
		return dst

	case *ast.CastNode:
		src := c.genExprLoad(n.Value, indent)
		dst := c.tmp()
		if n.Type == "float" {
			c.emit("%sitof   %s %s", indent, dst, src)
		} else {
			c.emit("%sftoi   %s %s", indent, dst, src)
		}
		return dst

	case *ast.AddressNode:
		dst := c.tmp()
		c.emit("%smov    %s %%%s.ptr", indent, dst, n.Name)
		return dst

	case *ast.DerefNode:
		ptr := c.tmp()
		c.emit("%sloadp  %s %%%s.ptr", indent, ptr, n.Name)
		dst := c.tmp()
		c.emit("%sload   %s %s", indent, dst, ptr)
		return dst

	case *ast.IndexNode:
		idx := c.genExprLoad(n.Index, indent)
		dst := c.tmp()
		c.emit("%sload   %s %%%s.ptr[%s]", indent, dst, n.Name, idx)
		return dst
	}
	return "0"
}

func (c *CAIGen) genExprLoad(node ast.Node, indent string) string {
	if lit, ok := node.(*ast.LiteralNode); ok && lit.Kind == "INT" {
		return lit.Value
	}
	if lit, ok := node.(*ast.LiteralNode); ok && lit.Kind == "FLOAT" {
		return lit.Value
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
