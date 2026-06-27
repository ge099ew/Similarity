// Package codegen: AST → QBE IR を生成する
package codegen

import (
	"fmt"
	"similarity/ast"
	"strings"
)

// Similarityの型 → QBEの型
var typeMap = map[string]string{
	"int":         "w",
	"float":       "s",
	"bool":        "w",
	"String":      "l",
	"Box_int":     "l",
	"Box_float":   "l",
	"Array_int":   "l",
	"Array_float": "l",
	"Array_bool":  "l",
}

type Codegen struct {
	buf     strings.Builder
	counter int
	// 変数名 → スタックポインタ変数名（メモリベース）
	// 例: "sum" → "%sum.ptr"
	vars map[string]string
}

func New() *Codegen {
	return &Codegen{vars: make(map[string]string)}
}

func (c *Codegen) fresh(prefix string) string {
	c.counter++
	return fmt.Sprintf("%s%d", prefix, c.counter)
}

func (c *Codegen) emit(format string, args ...interface{}) {
	fmt.Fprintf(&c.buf, format+"\n", args...)
}

func (c *Codegen) Generate(prog *ast.Program) string {
	c.buf.Reset()

	if prog.Explanation != nil {
		c.emit("# Similarity - %s", prog.Explanation.Category)
		for k, v := range prog.Explanation.Args {
			c.emit("# %s: %s", k, v)
		}
		if prog.Explanation.Category == "System" {
			if v := prog.Explanation.Args["value"]; v == "HFT" {
				c.emit("# HFTモード: 自動Wait挿入なし")
			}
		}
		c.emit("")
	}

	for _, stmt := range prog.Statements {
		c.genTopLevel(stmt)
	}

	return c.buf.String()
}

func (c *Codegen) genTopLevel(node ast.Node) {
	switch n := node.(type) {
	case *ast.FuncNode:
		c.genFunc(n)
	case *ast.ImportNode:
		c.emit("# import %s", n.Module)
	case *ast.ExternNode:
		for _, lib := range n.Libs {
			c.emit("# extern lib: %s", lib)
		}
	}
}

// 関数生成
func (c *Codegen) genFunc(fn *ast.FuncNode) {
	// 関数ごとに変数マップをリセット
	c.vars = make(map[string]string)

	var params []string
	for _, p := range fn.Params {
		qt := c.qbeType(p.Type)
		pv := "%" + p.Name
		// パラメータはSSA変数として直接使う（変更不可前提）
		c.vars[p.Name] = pv
		params = append(params, fmt.Sprintf("%s %s", qt, pv))
	}

	export := ""
	if fn.Public {
		export = "export "
	}

	c.emit("%sfunction w $%s(%s) {", export, fn.Name, strings.Join(params, ", "))
	c.emit("@start")

	for _, stmt := range fn.Body {
		c.genStmt(stmt, "    ")
	}

	// return
	if fn.Returns != nil {
		if lit, ok := fn.Returns.(*ast.LiteralNode); ok {
			retVal := c.emitLoad(lit.Value, "    ")
			c.emit("    ret %s", retVal)
		} else {
			c.emit("    ret 0")
		}
	} else {
		c.emit("    ret 0")
	}

	c.emit("}")
	c.emit("")
}

// 文を生成
func (c *Codegen) genStmt(node ast.Node, indent string) {
	switch n := node.(type) {
	case *ast.VariableNode:
		c.genVariable(n, indent)
	case *ast.IfNode:
		c.genIf(n, indent)
	case *ast.LoopNode:
		c.genLoop(n, indent)
	case *ast.CallNode:
		c.genCall(n, indent)
	case *ast.FatalNode:
		c.emit("%s# Fatal: %s - %s", indent, n.ErrType, n.Msg)
		c.emit("%scall $exit(w 1)", indent)
	case *ast.ErrorNode:
		c.genError(n, indent)
	case *ast.FuncNode:
		c.genFunc(n)
	}
}

// Variable[let{int(x:10)}]
// メモリベース: alloc4でスタック確保、storew/loadwでアクセス
func (c *Codegen) genVariable(v *ast.VariableNode, indent string) {
	qt := c.qbeType(v.Type)
	ptrVar := "%" + v.Name + ".ptr"

	if _, exists := c.vars[v.Name]; !exists {
		// 初回宣言: スタック領域を確保
		c.vars[v.Name] = ptrVar
		c.emit("%s%s =l alloc4 4", indent, ptrVar)
	}

	// 値を計算してストア
	ptr := c.vars[v.Name]
	if v.Value != nil {
		val := c.evalToTemp(v.Value, qt, indent)
		c.emit("%sstorew %s, %s", indent, val, ptr)
	} else {
		c.emit("%sstorew 0, %s", indent, ptr)
	}

	_ = qt
}

// 変数をロードしてテンポラリに入れる
func (c *Codegen) emitLoad(name, indent string) string {
	if ptr, ok := c.vars[name]; ok {
		tmp := "%" + c.fresh("t")
		c.emit("%s%s =w loadw %s", indent, tmp, ptr)
		return tmp
	}
	// 数値リテラルはそのまま
	return name
}

// 値を評価してQBE変数名を返す
func (c *Codegen) evalToTemp(node ast.Node, qt string, indent string) string {
	if node == nil {
		return "0"
	}
	switch n := node.(type) {
	case *ast.LiteralNode:
		// 変数ならloadw、数値リテラルならcopy
		if ptr, ok := c.vars[n.Value]; ok {
			tmp := "%" + c.fresh("t")
			c.emit("%s%s =w loadw %s", indent, tmp, ptr)
			return tmp
		}
		return n.Value
	case *ast.ExprNode:
		return c.genExprNode(n, qt, indent)
	}
	return "0"
}

// 演算式のIR生成
func (c *Codegen) genExprNode(expr *ast.ExprNode, qt string, indent string) string {
	if expr.Type != "" {
		qt = c.qbeType(expr.Type)
	}

	result := "%" + c.fresh("r")
	op := c.qbeOp(expr.Op, qt)

	left := c.evalToTemp(expr.Left, qt, indent)
	right := c.evalToTemp(expr.Right, qt, indent)

	c.emit("%s%s =%s %s %s, %s", indent, result, qt, op, left, right)
	return result
}

// If[check{le(hp,0)}, True[...], False[...]]
func (c *Codegen) genIf(n *ast.IfNode, indent string) {
	trueLabel  := "@" + c.fresh("true")
	falseLabel := "@" + c.fresh("false")
	endLabel   := "@" + c.fresh("end")

	if cond, ok := n.Condition.(*ast.ConditionNode); ok {
		condVar := "%" + c.fresh("cond")
		qbeOp   := c.qbeCompare(cond.Op)
		left    := c.emitLoad(cond.Left, indent)
		right   := c.emitLoad(cond.Right, indent)
		c.emit("%s%s =w %s %s, %s", indent, condVar, qbeOp, left, right)
		c.emit("%sjnz %s, %s, %s", indent, condVar, trueLabel, falseLabel)
	}

	c.emit("%s", trueLabel)
	for _, stmt := range n.True {
		c.genStmt(stmt, indent+"    ")
	}
	c.emit("%sjmp %s", indent+"    ", endLabel)

	c.emit("%s", falseLabel)
	for _, stmt := range n.False {
		c.genStmt(stmt, indent+"    ")
	}
	c.emit("%sjmp %s", indent+"    ", endLabel)

	c.emit("%s", endLabel)
}

// Loop
func (c *Codegen) genLoop(n *ast.LoopNode, indent string) {
	loopLabel := "@" + c.fresh("loop")
	bodyLabel := "@" + c.fresh("body")
	endLabel  := "@" + c.fresh("lend")

	if n.Kind == "for" {
		if init, ok := n.Init.(*ast.VariableNode); ok {
			c.genVariable(init, indent)
			c.emit("%sjmp %s", indent, loopLabel)
			c.emit("%s", loopLabel)

			if cond, ok := n.Condition.(*ast.ConditionNode); ok {
				condVar := "%" + c.fresh("cond")
				qbeOp   := c.qbeCompare(cond.Op)
				left    := c.emitLoad(cond.Left, indent)
				right   := c.emitLoad(cond.Right, indent)
				c.emit("%s%s =w %s %s, %s", indent, condVar, qbeOp, left, right)
				c.emit("%sjnz %s, %s, %s", indent, condVar, bodyLabel, endLabel)
			}

			c.emit("%s", bodyLabel)
			for _, stmt := range n.Body {
				c.genStmt(stmt, indent+"    ")
			}

			// step: i += step
			iCur := c.emitLoad(init.Name, indent+"    ")
			iNew := "%" + c.fresh("step")
			c.emit("%s%s =w add %s, %d", indent+"    ", iNew, iCur, n.Step)
			c.emit("%sstorew %s, %s", indent+"    ", iNew, c.vars[init.Name])

			c.emit("%sjmp %s", indent+"    ", loopLabel)
		}
	} else { // Count
		if init, ok := n.Init.(*ast.VariableNode); ok {
			c.genVariable(init, indent)
			c.emit("%sjmp %s", indent, loopLabel)
			c.emit("%s", loopLabel)

			cnt := c.emitLoad(init.Name, indent)
			condVar := "%" + c.fresh("cond")
			c.emit("%s%s =w csgtw %s, 0", indent, condVar, cnt)
			c.emit("%sjnz %s, %s, %s", indent, condVar, bodyLabel, endLabel)

			c.emit("%s", bodyLabel)
			for _, stmt := range n.Body {
				c.genStmt(stmt, indent+"    ")
			}

			cntCur := c.emitLoad(init.Name, indent+"    ")
			cntNew := "%" + c.fresh("step")
			c.emit("%s%s =w sub %s, 1", indent+"    ", cntNew, cntCur)
			c.emit("%sstorew %s, %s", indent+"    ", cntNew, c.vars[init.Name])
			c.emit("%sjmp %s", indent+"    ", loopLabel)
		}
	}
	c.emit("%s", endLabel)
}

// call{funcName(args)}
func (c *Codegen) genCall(n *ast.CallNode, indent string) {
	var args []string
	for _, arg := range n.Args {
		val := c.evalToTemp(arg, "w", indent)
		args = append(args, "w "+val)
	}
	result := "%" + c.fresh("ret")
	c.emit("%s%s =w call $%s(%s)", indent, result, n.FuncName, strings.Join(args, ", "))
}

// Error[try{...}, Ok[...], Err[...]]
func (c *Codegen) genError(n *ast.ErrorNode, indent string) {
	okLabel  := "@" + c.fresh("ok")
	errLabel := "@" + c.fresh("err")
	endLabel := "@" + c.fresh("erend")

	errFlag := "%" + c.fresh("eflag")
	c.emit("%s%s =w copy 0", indent, errFlag)
	for _, stmt := range n.Try {
		c.genStmt(stmt, indent)
	}
	c.emit("%sjnz %s, %s, %s", indent, errFlag, errLabel, okLabel)

	c.emit("%s", okLabel)
	for _, stmt := range n.Ok {
		c.genStmt(stmt, indent+"    ")
	}
	c.emit("%sjmp %s", indent+"    ", endLabel)

	c.emit("%s", errLabel)
	if n.Pass {
		c.emit("%sret 1", indent+"    ")
	} else {
		c.emit("%s# Err: %s - %s", indent+"    ", n.ErrType, n.Msg)
	}
	c.emit("%sjmp %s", indent+"    ", endLabel)
	c.emit("%s", endLabel)
}

// QBE比較命令（符号付き整数）
func (c *Codegen) qbeCompare(op string) string {
	switch op {
	case "eq": return "ceqw"
	case "ne": return "cnew"
	case "lt": return "csltw"
	case "le": return "cslew"
	case "gt": return "csgtw"
	case "ge": return "csgew"
	default:   return "ceqw"
	}
}

// Similarity演算子 → QBE命令
func (c *Codegen) qbeOp(op string, qt string) string {
	switch op {
	case "+": if qt == "s" { return "adds" }; return "add"
	case "-": if qt == "s" { return "subs" }; return "sub"
	case "*": if qt == "s" { return "muls" }; return "mul"
	case "/": if qt == "s" { return "divs" }; return "div"
	default:  return "add"
	}
}

// Similarity型 → QBE型
func (c *Codegen) qbeType(t string) string {
	if qt, ok := typeMap[t]; ok { return qt }
	return "w"
}
