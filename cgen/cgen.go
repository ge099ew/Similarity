// Package cgen: AST → C コードを生成する（QBEまでの暫定バックエンド）
package cgen

import (
	"fmt"
	"similarity/ast"
	"similarity/stdlib"
	"strings"
)

var typeMapC = map[string]string{
	"int":         "int",
	"float":       "float",
	"bool":        "int",
	"String":      "char*",
	"Box_int":     "int*",
	"Box_float":   "float*",
	"Array_int":   "int*",
	"Array_float": "float*",
	"Array_bool":  "int*",
}

type CGen struct {
	buf     strings.Builder
	counter int
	vars    map[string]string // name → C type
}

func New() *CGen {
	return &CGen{vars: make(map[string]string)}
}

func (c *CGen) fresh(prefix string) string {
	c.counter++
	return fmt.Sprintf("%s%d", prefix, c.counter)
}

func (c *CGen) emit(format string, args ...interface{}) {
	fmt.Fprintf(&c.buf, format+"\n", args...)
}

func (c *CGen) Generate(prog *ast.Program) string {
	c.buf.Reset()
	c.emit("#include <stdio.h>")
	c.emit("#include <stdlib.h>")
	c.emit("#include <time.h>")
	c.emit("")

	if prog.Explanation != nil {
		c.emit("// Similarity - %s", prog.Explanation.Category)
		for k, v := range prog.Explanation.Args {
			c.emit("// %s: %s", k, v)
		}
		c.emit("")
	}

	hasMain := false
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*ast.FuncNode); ok && fn.Name == "main" {
			hasMain = true
		}
	}

	// main()がある場合、結果を表示するラッパーを作る
	if hasMain {
		for _, stmt := range prog.Statements {
			if fn, ok := stmt.(*ast.FuncNode); ok && fn.Name == "main" {
				c.genFuncAs(fn, "sim_main")
			} else {
				c.genTopLevel(stmt)
			}
		}
		c.emit("int main() {")
		c.emit("    struct timespec start, end;")
		c.emit("    clock_gettime(CLOCK_MONOTONIC, &start);")
		c.emit("    long result = sim_main();")
		c.emit("    clock_gettime(CLOCK_MONOTONIC, &end);")
		c.emit("    double ms = (end.tv_sec - start.tv_sec) * 1000.0 + (end.tv_nsec - start.tv_nsec) / 1e6;")
		c.emit("    printf(\"Similarity result: %%ld  time: %%.2fms\\n\", result, ms);")
		c.emit("    return 0;")
		c.emit("}")
	} else {
		for _, stmt := range prog.Statements {
			c.genTopLevel(stmt)
		}
	}

	return c.buf.String()
}

func (c *CGen) genTopLevel(node ast.Node) {
	switch n := node.(type) {
	case *ast.FuncNode:
		c.genFunc(n)
	case *ast.VariableNode:
		ct := c.cType(n.Type)
		c.vars[n.Name] = ct
		if n.Value != nil {
			c.emit("%s %s = %s;", ct, n.Name, c.evalLiteral(n.Value))
		} else {
			c.emit("%s %s = 0;", ct, n.Name)
		}
	case *ast.ImportNode:
		if cImpl, ok := stdlib.AvailableLibsC[n.Module]; ok {
			c.emit("%s", cImpl)
		} else {
			c.emit("// import %s (external)", n.Module)
		}
	case *ast.ExternNode:
		for _, lib := range n.Libs {
			c.emit("// extern: %s", lib)
		}
	}
}

func (c *CGen) genFunc(fn *ast.FuncNode) {
	c.genFuncAs(fn, fn.Name)
}

func (c *CGen) genFuncAs(fn *ast.FuncNode, name string) {
	retType := "long"

	// 関数スコープをリセット（前の関数の変数が残らないように）
	savedVars := c.vars
	c.vars = make(map[string]string)

	var params []string
	for _, p := range fn.Params {
		ct := c.cType(p.Type)
		c.vars[p.Name] = ct
		params = append(params, fmt.Sprintf("%s %s", ct, p.Name))
	}

	c.emit("%s %s(%s) {", retType, name, strings.Join(params, ", "))

	for _, stmt := range fn.Body {
		c.genStmt(stmt, "    ")
	}

	// return()はBodyにReturnNodeとして入っている。
	// fn.Returnsが残っている場合（旧構文互換）はそちらも処理。
	if fn.Returns != nil {
		c.emit("    return %s;", c.evalLiteral(fn.Returns))
	}
	c.emit("}")
	c.emit("")
	c.vars = savedVars
}

func (c *CGen) genStmt(node ast.Node, indent string) {
	switch n := node.(type) {
	case *ast.VariableNode:
		// struct定義はスキップ
		if n.Type == "__struct__" {
			return
		}
		ct := c.cType(n.Type)
		if _, exists := c.vars[n.Name]; exists {
			// 既に宣言済み → 再代入（再宣言しない、シャドーイング防止）
			if n.Value != nil {
				c.emit("%s%s = %s;", indent, n.Name, c.evalLiteral(n.Value))
			}
		} else {
			c.vars[n.Name] = ct
			if n.Value != nil {
				c.emit("%s%s %s = %s;", indent, ct, n.Name, c.evalLiteral(n.Value))
			} else {
				c.emit("%s%s %s = 0;", indent, ct, n.Name)
			}
		}
	case *ast.ReturnNode:
		if n.Value != nil {
			c.emit("%sreturn %s;", indent, c.evalLiteral(n.Value))
		} else {
			c.emit("%sreturn 0;", indent)
		}
	case *ast.IfNode:
		c.genIf(n, indent)
	case *ast.LoopNode:
		c.genLoop(n, indent)
	case *ast.CallNode:
		c.genCall(n, indent)
	case *ast.FatalNode:
		c.emit("%s// Fatal: %s - %s", indent, n.ErrType, n.Msg)
		c.emit("%sfprintf(stderr, \"Fatal: %s\\n\");", indent, n.ErrType)
		c.emit("%sexit(1);", indent)
	case *ast.ErrorNode:
		c.genError(n, indent)
	case *ast.FuncNode:
		c.genFunc(n)
	}
}

func (c *CGen) genIf(n *ast.IfNode, indent string) {
	if cond, ok := n.Condition.(*ast.ConditionNode); ok {
		c.emit("%sif (%s %s %s) {", indent, cond.Left, c.cCompare(cond.Op), cond.Right)
	} else {
		c.emit("%sif (1) {", indent)
	}
	for _, stmt := range n.True {
		c.genStmt(stmt, indent+"    ")
	}
	if len(n.False) > 0 {
		c.emit("%s} else {", indent)
		for _, stmt := range n.False {
			c.genStmt(stmt, indent+"    ")
		}
	}
	c.emit("%s}", indent)
}

func (c *CGen) genLoop(n *ast.LoopNode, indent string) {
	if n.Kind == "for" {
		if init, ok := n.Init.(*ast.VariableNode); ok {
			ct := c.cType(init.Type)
			initVal := "0"
			if init.Value != nil {
				initVal = c.evalLiteral(init.Value)
			}
			var condStr string
			if cond, ok := n.Condition.(*ast.ConditionNode); ok {
				condStr = fmt.Sprintf("%s %s %s", cond.Left, c.cCompare(cond.Op), cond.Right)
			}
			c.emit("%sfor (%s %s = %s; %s; %s += %d) {",
				indent, ct, init.Name, initVal, condStr, init.Name, n.Step)
		}
	} else { // count
		if init, ok := n.Init.(*ast.VariableNode); ok {
			ct := c.cType(init.Type)
			countVal := c.evalLiteral(init.Value)
			tmp := c.fresh("i")
			c.emit("%sfor (%s %s = 0; %s < %s; %s++) {", indent, ct, tmp, tmp, countVal, tmp)
		}
	}
	for _, stmt := range n.Body {
		c.genStmt(stmt, indent+"    ")
	}
	c.emit("%s}", indent)
}

func (c *CGen) genCall(n *ast.CallNode, indent string) {
	var args []string
	for _, arg := range n.Args {
		args = append(args, c.evalLiteral(arg))
	}
	c.emit("%s%s(%s);", indent, n.FuncName, strings.Join(args, ", "))
}

func (c *CGen) genError(n *ast.ErrorNode, indent string) {
	c.emit("%s{", indent)
	for _, stmt := range n.Try {
		c.genStmt(stmt, indent+"    ")
	}
	c.emit("%s}", indent)
}

func (c *CGen) evalLiteral(node ast.Node) string {
	if node == nil {
		return "0"
	}
	switch n := node.(type) {
	case *ast.LiteralNode:
		if n.Kind == "STRING_LIT" {
			return fmt.Sprintf("%q", n.Value)
		}
		return n.Value
	case *ast.ExprNode:
		left := c.evalLiteral(n.Left)
		right := c.evalLiteral(n.Right)
		return fmt.Sprintf("(%s %s %s)", left, n.Op, right)
	case *ast.CallNode:
		var args []string
		for _, arg := range n.Args {
			args = append(args, c.evalLiteral(arg))
		}
		return fmt.Sprintf("%s(%s)", n.FuncName, strings.Join(args, ", "))
	case *ast.CastNode:
		inner := c.evalLiteral(n.Value)
		return fmt.Sprintf("((%s)%s)", c.cType(n.Type), inner)
	case *ast.AddressNode:
		return fmt.Sprintf("&%s", n.Name)
	case *ast.DerefNode:
		return fmt.Sprintf("*%s", n.Name)
	case *ast.IndexNode:
		idx := c.evalLiteral(n.Index)
		return fmt.Sprintf("%s[%s]", n.Name, idx)
	}
	return "0"
}

func (c *CGen) cType(t string) string {
	if ct, ok := typeMapC[t]; ok {
		return ct
	}
	return "int"
}

func (c *CGen) cCompare(op string) string {
	switch op {
	case "equal":
		return "=="
	case "notequal":
		return "!="
	case "less":
		return "<"
	case "lesseq":
		return "<="
	case "greater":
		return ">"
	case "greatereq":
		return ">="
	}
	return "=="
}
