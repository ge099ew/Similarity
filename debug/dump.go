// Package debug: コンパイル各ステージのデバッグ出力を担当する。
//
// 対応するCLIオプションと出力ステージ:
//
//	--dump-tokens    Lexer      → DumpTokens()
//	--dump-ast       Parser     → DumpAST()
//	--dump-types     TypeChecker→ DumpTypes()
//	--dump-analyzer  Analyzer   → DumpAnalyzer()
//	--dump-backend   Backend    → DumpBackend()   BackendFunction生成直後を表示
//	--dump-cfg       Backend    → DumpCFG()        CFG生成直後を表示
//	--dump-regalloc  Backend    → DumpRegAlloc()  ※RegAlloc実装後に有効化
//	--dump-machine   Backend    → DumpMachine()   ※Backend実装後に有効化
//
// 通常のコンパイル処理には一切影響を与えない。
package debug

import (
	"fmt"
	"strings"

	"similarity/ast"
	"similarity/backend"
	"similarity/lexer"
)

// =============================================================
// --dump-tokens: Lexer出力
// =============================================================

// DumpTokens はLexerが生成したトークン列を標準出力に表示する。
func DumpTokens(tokens []lexer.Token) {
	fmt.Println("===== DUMP: tokens =====")
	for _, t := range tokens {
		if t.Literal != "" && t.Literal != string(t.Type) {
			fmt.Printf("  %-20s %q  (line:%d col:%d)\n", t.Type, t.Literal, t.Line, t.Col)
		} else {
			fmt.Printf("  %-20s        (line:%d col:%d)\n", t.Type, t.Line, t.Col)
		}
	}
	fmt.Println()
}

// =============================================================
// --dump-ast: Parser出力（AST）
// =============================================================

// DumpAST はParserが生成したASTを標準出力にツリー形式で表示する。
// Analyzerのアノテーションは付与前なので表示しない。
func DumpAST(prog *ast.Program) {
	fmt.Println("===== DUMP: ast =====")
	if prog.Explanation != nil {
		fmt.Printf("  Explanation[%s]\n", prog.Explanation.Category)
	}
	for _, stmt := range prog.Statements {
		dumpNode(stmt, "  ", true)
	}
	fmt.Println()
}

func dumpNode(node ast.Node, indent string, raw bool) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.FuncNode:
		pub := ""
		if n.Public {
			pub = "_public"
		}
		fmt.Printf("%sFunction%s [%s]\n", indent, pub, n.Name)
		if len(n.Params) > 0 {
			fmt.Printf("%s  params:\n", indent)
			for _, p := range n.Params {
				fmt.Printf("%s    %s %s\n", indent, p.Type, p.Name)
			}
		}
		for _, s := range n.Body {
			dumpNode(s, indent+"  ", raw)
		}
		if n.Returns != nil {
			fmt.Printf("%s  return:\n", indent)
			dumpNode(n.Returns, indent+"    ", raw)
		}
	case *ast.VariableNode:
		mut := "let"
		if !n.Mutable {
			mut = "unclet"
		}
		fmt.Printf("%sVariable[%s{%s(%s: ...)}]\n", indent, mut, n.Type, n.Name)
	case *ast.MutationNode:
		fmt.Printf("%sMutation[%s]\n", indent, n.Name)
	case *ast.IfNode:
		fmt.Printf("%sIf\n", indent)
		dumpNode(n.Condition, indent+"  cond: ", raw)
		fmt.Printf("%s  True:\n", indent)
		for _, s := range n.True {
			dumpNode(s, indent+"    ", raw)
		}
		if len(n.False) > 0 {
			fmt.Printf("%s  False:\n", indent)
			for _, s := range n.False {
				dumpNode(s, indent+"    ", raw)
			}
		}
	case *ast.LoopNode:
		fmt.Printf("%sLoop[%s]\n", indent, n.Kind)
		for _, s := range n.Body {
			dumpNode(s, indent+"  ", raw)
		}
	case *ast.ReturnNode:
		fmt.Printf("%sReturn\n", indent)
		dumpNode(n.Value, indent+"  ", raw)
	case *ast.CallNode:
		fmt.Printf("%sCall[%s]\n", indent, n.FuncName)
	case *ast.ExprNode:
		fmt.Printf("%sExpr[%s{%s(...)}]\n", indent, n.Op, n.Type)
	case *ast.ConditionNode:
		fmt.Printf("%sCondition[%s(%s : %s)]\n", indent, n.Op, n.Left, n.Right)
	case *ast.LiteralNode:
		fmt.Printf("%sLiteral[%s: %s]\n", indent, n.Kind, n.Value)
	case *ast.IncrNode:
		fmt.Printf("%sIncr[%s %s]\n", indent, n.Op, n.Name)
	case *ast.ArrayNode:
		fmt.Printf("%sArray[%s %s[%d]]\n", indent, n.ElemType, n.Name, n.Size)
	case *ast.ArrayStoreNode:
		fmt.Printf("%sArrayStore[%s[...]]\n", indent, n.Name)
	case *ast.CastNode:
		fmt.Printf("%sCast[%s]\n", indent, n.Type)
	case *ast.IndexNode:
		fmt.Printf("%sIndex[%s[...]]\n", indent, n.Name)
	case *ast.BreakNode:
		fmt.Printf("%sBreak\n", indent)
	case *ast.ContinueNode:
		fmt.Printf("%sContinue\n", indent)
	default:
		fmt.Printf("%s%T\n", indent, node)
	}
}

// =============================================================
// --dump-types: TypeChecker通過後の型情報
// =============================================================

// DumpTypes はAST内のFuncNode/VariableNodeから型情報を収集して表示する。
// TypeCheckerのvars/funcsは非公開のため、ASTから直接収集する。
func DumpTypes(prog *ast.Program) {
	fmt.Println("===== DUMP: types =====")
	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *ast.FuncNode:
			fmt.Printf("  Function %-20s : %s\n", n.Name, resolveReturnTypeStr(n))
			collectVarTypes(n.Params, n.Body, "    ")
		case *ast.ExternNode:
			for _, fn := range n.Funcs {
				fmt.Printf("  Extern   %-20s : (external)\n", fn.Name)
			}
		}
	}
	fmt.Println()
}

func resolveReturnTypeStr(fn *ast.FuncNode) string {
	// FuncNode.Returns があれば末尾return
	if fn.Returns != nil {
		return exprTypeStr(fn.Returns)
	}
	// Body内のReturnNodeを探す
	if t := findReturnTypeStr(fn.Body); t != "" {
		return t
	}
	return "void"
}

func findReturnTypeStr(body []ast.Node) string {
	for _, n := range body {
		switch s := n.(type) {
		case *ast.ReturnNode:
			if s.Value != nil {
				return exprTypeStr(s.Value)
			}
			return "void"
		case *ast.IfNode:
			if t := findReturnTypeStr(s.True); t != "" {
				return t
			}
			if t := findReturnTypeStr(s.False); t != "" {
				return t
			}
		case *ast.LoopNode:
			if t := findReturnTypeStr(s.Body); t != "" {
				return t
			}
		}
	}
	return ""
}

func exprTypeStr(node ast.Node) string {
	switch n := node.(type) {
	case *ast.LiteralNode:
		switch n.Kind {
		case "INT_LIT":
			return "int"
		case "FLOAT_LIT":
			return "float"
		case "BOOL_LIT":
			return "bool"
		case "STRING_LIT":
			return "String"
		case "IDENT":
			return n.Value + " (ident)"
		}
	case *ast.ExprNode:
		return n.Type
	case *ast.CallNode:
		return "call:" + n.FuncName
	case *ast.CastNode:
		return n.Type
	}
	return "?"
}

func collectVarTypes(params []ast.VariableNode, body []ast.Node, indent string) {
	for _, p := range params {
		fmt.Printf("%sParam    %-20s : %s\n", indent, p.Name, p.Type)
	}
	collectBodyVarTypes(body, indent)
}

func collectBodyVarTypes(body []ast.Node, indent string) {
	for _, node := range body {
		switch n := node.(type) {
		case *ast.VariableNode:
			fmt.Printf("%sVariable %-20s : %s\n", indent, n.Name, n.Type)
		case *ast.IfNode:
			collectBodyVarTypes(n.True, indent)
			collectBodyVarTypes(n.False, indent)
		case *ast.LoopNode:
			collectBodyVarTypes(n.Body, indent)
		}
	}
}

// =============================================================
// --dump-analyzer: Annotation付きAST
// =============================================================

// DumpAnalyzer はAnalyzer通過後のAnnotation付きASTを標準出力に表示する。
// これはBackend開発の基盤となる出力であり、
// BackendはこのDump内容だけを前提として実装できる状態を目標とする。
func DumpAnalyzer(prog *ast.Program) {
	fmt.Println("===== DUMP: analyzer =====")
	for _, stmt := range prog.Statements {
		fn, ok := stmt.(*ast.FuncNode)
		if !ok {
			continue
		}
		dumpAnalyzerFunc(fn)
	}
	fmt.Println()
}

func dumpAnalyzerFunc(fn *ast.FuncNode) {
	pub := ""
	if fn.Public {
		pub = " [public]"
	}
	fmt.Printf("===== Function: %s%s =====\n\n", fn.Name, pub)

	// Return
	fmt.Println("Return")
	fmt.Printf("  size=%d\n", fn.ReturnAnn.Size)
	fmt.Printf("  ptr=%v\n", fn.ReturnAnn.IsPtr)
	if fn.ReturnAnn.IsFloat {
		fmt.Printf("  float=true\n")
	}
	fmt.Println()

	// Parameters
	fmt.Println("Parameters")
	if len(fn.Params) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, p := range fn.Params {
			fmt.Printf("  %s %s\n", p.Type, p.Name)
			fmt.Printf("    size=%d  ptr=%v\n", p.Ann.Size, p.Ann.IsPtr)
		}
	}
	fmt.Println()

	// Local Variables
	fmt.Println("Local Variables")
	if len(fn.LocalVars) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, v := range fn.LocalVars {
			fmt.Printf("  %s\n", v.Name)
			fmt.Printf("    type=%-10s  size=%d  ptr=%v\n", v.Type, v.Ann.Size, v.Ann.IsPtr)
		}
	}
	fmt.Println()

	// Loops
	loops := collectLoops(fn.Body)
	fmt.Println("Loops")
	if len(loops) == 0 {
		fmt.Println("  (none)")
	} else {
		for i, l := range loops {
			fmt.Printf("  Loop #%d  depth=%d\n", i, l.LoopDepth)
		}
	}
	fmt.Println()

	// Calls
	calls := collectCalls(fn.Body)
	fmt.Println("Calls")
	if len(calls) == 0 {
		fmt.Println("  (none)")
	} else {
		// 同じ関数名は1度だけ表示
		seen := map[string]bool{}
		for _, c := range calls {
			if seen[c.FuncName] {
				continue
			}
			seen[c.FuncName] = true
			fmt.Printf("  %s\n", c.FuncName)
			fmt.Printf("    return_size=%d  ptr=%v\n", c.ReturnAnn.Size, c.ReturnAnn.IsPtr)
		}
	}
	fmt.Println()

	// Expressions
	exprs := collectExprs(fn.Body)
	fmt.Println("Expressions")
	if len(exprs) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, e := range exprs {
			fmt.Printf("  %s{%s(...)}\n", e.Op, e.Type)
			fmt.Printf("    size=%d  ptr=%v\n", e.Ann.Size, e.Ann.IsPtr)
		}
	}
	fmt.Println()

	// Conditions
	conds := collectConds(fn.Body)
	fmt.Println("Conditions")
	if len(conds) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, c := range conds {
			fmt.Printf("  %s(%s : %s)\n", c.Op, c.Left, c.Right)
			fmt.Printf("    left:  size=%d  ptr=%v\n", c.LeftAnn.Size, c.LeftAnn.IsPtr)
			fmt.Printf("    right: size=%d  ptr=%v\n", c.RightAnn.Size, c.RightAnn.IsPtr)
		}
	}
	fmt.Println()

	// Mutations
	muts := collectMutations(fn.Body)
	fmt.Println("Mutations")
	if len(muts) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, m := range muts {
			fmt.Printf("  %s\n", m.Name)
			fmt.Printf("    size=%d  ptr=%v\n", m.Ann.Size, m.Ann.IsPtr)
		}
	}
	fmt.Println()

	fmt.Println(strings.Repeat("-", 40))
	fmt.Println()
}

// --- 収集ヘルパー ---

func collectLoops(body []ast.Node) []*ast.LoopNode {
	var result []*ast.LoopNode
	for _, node := range body {
		switch n := node.(type) {
		case *ast.LoopNode:
			result = append(result, n)
			result = append(result, collectLoops(n.Body)...)
		case *ast.IfNode:
			result = append(result, collectLoops(n.True)...)
			result = append(result, collectLoops(n.False)...)
		}
	}
	return result
}

func collectCalls(body []ast.Node) []*ast.CallNode {
	var result []*ast.CallNode
	for _, node := range body {
		result = append(result, collectCallsFromNode(node)...)
	}
	return result
}

func collectCallsFromNode(node ast.Node) []*ast.CallNode {
	if node == nil {
		return nil
	}
	var result []*ast.CallNode
	switch n := node.(type) {
	case *ast.CallNode:
		result = append(result, n)
		for _, arg := range n.Args {
			result = append(result, collectCallsFromNode(arg)...)
		}
	case *ast.VariableNode:
		result = append(result, collectCallsFromNode(n.Value)...)
	case *ast.MutationNode:
		result = append(result, collectCallsFromNode(n.Value)...)
	case *ast.ReturnNode:
		result = append(result, collectCallsFromNode(n.Value)...)
	case *ast.ExprNode:
		result = append(result, collectCallsFromNode(n.Left)...)
		result = append(result, collectCallsFromNode(n.Right)...)
	case *ast.IfNode:
		result = append(result, collectCallsFromNode(n.Condition)...)
		for _, s := range n.True {
			result = append(result, collectCallsFromNode(s)...)
		}
		for _, s := range n.False {
			result = append(result, collectCallsFromNode(s)...)
		}
	case *ast.LoopNode:
		result = append(result, collectCallsFromNode(n.Condition)...)
		for _, s := range n.Body {
			result = append(result, collectCallsFromNode(s)...)
		}
	}
	return result
}

func collectExprs(body []ast.Node) []*ast.ExprNode {
	var result []*ast.ExprNode
	for _, node := range body {
		result = append(result, collectExprsFromNode(node)...)
	}
	return result
}

func collectExprsFromNode(node ast.Node) []*ast.ExprNode {
	if node == nil {
		return nil
	}
	var result []*ast.ExprNode
	switch n := node.(type) {
	case *ast.ExprNode:
		result = append(result, n)
		result = append(result, collectExprsFromNode(n.Left)...)
		result = append(result, collectExprsFromNode(n.Right)...)
	case *ast.VariableNode:
		result = append(result, collectExprsFromNode(n.Value)...)
	case *ast.MutationNode:
		result = append(result, collectExprsFromNode(n.Value)...)
	case *ast.ReturnNode:
		result = append(result, collectExprsFromNode(n.Value)...)
	case *ast.CallNode:
		for _, arg := range n.Args {
			result = append(result, collectExprsFromNode(arg)...)
		}
	case *ast.IfNode:
		result = append(result, collectExprsFromNode(n.Condition)...)
		for _, s := range n.True {
			result = append(result, collectExprsFromNode(s)...)
		}
		for _, s := range n.False {
			result = append(result, collectExprsFromNode(s)...)
		}
	case *ast.LoopNode:
		for _, s := range n.Body {
			result = append(result, collectExprsFromNode(s)...)
		}
	}
	return result
}

func collectConds(body []ast.Node) []*ast.ConditionNode {
	var result []*ast.ConditionNode
	for _, node := range body {
		result = append(result, collectCondsFromNode(node)...)
	}
	return result
}

func collectCondsFromNode(node ast.Node) []*ast.ConditionNode {
	if node == nil {
		return nil
	}
	var result []*ast.ConditionNode
	switch n := node.(type) {
	case *ast.ConditionNode:
		result = append(result, n)
	case *ast.IfNode:
		result = append(result, collectCondsFromNode(n.Condition)...)
		for _, s := range n.True {
			result = append(result, collectCondsFromNode(s)...)
		}
		for _, s := range n.False {
			result = append(result, collectCondsFromNode(s)...)
		}
	case *ast.LoopNode:
		result = append(result, collectCondsFromNode(n.Condition)...)
		for _, s := range n.Body {
			result = append(result, collectCondsFromNode(s)...)
		}
	}
	return result
}

func collectMutations(body []ast.Node) []*ast.MutationNode {
	var result []*ast.MutationNode
	for _, node := range body {
		switch n := node.(type) {
		case *ast.MutationNode:
			result = append(result, n)
		case *ast.IfNode:
			result = append(result, collectMutations(n.True)...)
			result = append(result, collectMutations(n.False)...)
		case *ast.LoopNode:
			result = append(result, collectMutations(n.Body)...)
		}
	}
	return result
}

// =============================================================
// --dump-cfg / --dump-regalloc / --dump-machine
// Backend実装後に有効化する。現時点では「未実装」を表示するのみ。
// =============================================================

// DumpCFG はC BackendのCFGをダンプする。
// CFG構築はC Backend（sim_backend）の責務のため、
// 現時点ではC Backend実装待ちのプレースホルダ。
func DumpCFG() {
	fmt.Println("===== DUMP: cfg =====")
	fmt.Println("  (C Backend実装後に有効化 - sim_backend --dump-cfg 経由)")
	fmt.Println()
}

// DumpRegAlloc はレジスタ割り当て結果を表示する。
// Backend実装後に内容を実装する。
func DumpRegAlloc() {
	fmt.Println("===== DUMP: regalloc =====")
	fmt.Println("  (Backend未実装 - Stage 3以降で有効化)")
	fmt.Println()
}

// DumpMachine は最終機械語/逆アセンブルを表示する。
// Backend実装後に内容を実装する。
func DumpMachine() {
	fmt.Println("===== DUMP: machine =====")
	fmt.Println("  (Backend未実装 - Stage 2以降で有効化)")
	fmt.Println()
}

// =============================================================
// --dump-backend: BackendFunction生成直後の内容を表示
// =============================================================

// DumpBackend はBackendFunction生成直後の内容を表示する。
// BackendFunctionの内容を変更しない。表示専用。
// Analyzerが付与したAnnotation情報（size/ptr/LoopDepth等）をそのまま表示する。
func DumpBackend(funcs []backend.BackendFunc) {
	fmt.Println("===== DUMP: backend =====")
	fmt.Println()
	for _, fn := range funcs {
		dumpBackendFunc(fn)
	}
}

func dumpBackendFunc(fn backend.BackendFunc) {
	pub := ""
	if fn.IsPublic {
		pub = " [public]"
	}
	fmt.Printf("===== Function: %s%s =====\n\n", fn.Name, pub)

	// Return
	fmt.Println("Return")
	fmt.Printf("  size=%d\n", fn.RetSize)
	fmt.Printf("  ptr=%v\n", fn.RetPtr)
	fmt.Println()

	// Parameters
	fmt.Println("Parameters")
	if len(fn.Params) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, p := range fn.Params {
			fmt.Printf("  p%d\n", p.Index)
			fmt.Printf("    name=%-12s  type=%-8s  size=%d  ptr=%v\n",
				p.Name, p.Type, p.Size, p.IsPtr)
		}
	}
	fmt.Println()

	// Local Variables
	fmt.Println("Local Variables")
	if len(fn.Locals) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, v := range fn.Locals {
			fmt.Printf("  v%d\n", v.Index)
			fmt.Printf("    name=%-12s  type=%-8s  size=%d  ptr=%v\n",
				v.Name, v.Type, v.Size, v.IsPtr)
		}
	}
	fmt.Println()

	// Statements（Backend Blocksとして表示）
	fmt.Println("Backend Blocks")
	blockIdx := 0
	dumpBFStmts(fn.Stmts, 0, &blockIdx)
	fmt.Println()

	fmt.Println(strings.Repeat("=", 40))
	fmt.Println()
}

func dumpBFStmts(stmts []backend.BFStmt, depth int, blockIdx *int) {
	ind := strings.Repeat("  ", depth)
	for _, s := range stmts {
		dumpBFStmt(s, depth, ind, blockIdx)
	}
}

func dumpBFStmt(s backend.BFStmt, depth int, ind string, blockIdx *int) {
	switch s.Kind {
	case backend.BFStmtVariable:
		fmt.Printf("%sVariable(%s)  size=%d  ptr=%v  = %s\n",
			ind, s.VarName, s.VarSize, s.VarPtr, s.Expr)

	case backend.BFStmtMutation:
		fmt.Printf("%sMutation(%s)  size=%d  ptr=%v  = %s\n",
			ind, s.VarName, s.VarSize, s.VarPtr, s.Expr)

	case backend.BFStmtIncr:
		fmt.Printf("%sIncr(%s %s)  size=%d  ptr=%v\n",
			ind, s.IncrName, s.IncrOp, s.IncrSize, s.IncrPtr)

	case backend.BFStmtLoop:
		fmt.Printf("%sBlock #%d  kind=loop  depth=%d\n", ind, *blockIdx, s.LoopDepth)
		*blockIdx++
		c := s.LoopCond
		fmt.Printf("%s  Cond: %s(%s[sz=%d ptr=%v] : %s[sz=%d ptr=%v])\n",
			ind, c.Op,
			c.Left, c.LeftSize, c.LeftPtr,
			c.Right, c.RightSize, c.RightPtr)
		fmt.Printf("%s  Statements\n", ind)
		dumpBFStmts(s.LoopBody, depth+2, blockIdx)

	case backend.BFStmtIf:
		fmt.Printf("%sBlock #%d  kind=if\n", ind, *blockIdx)
		*blockIdx++
		c := s.IfCond
		fmt.Printf("%s  Cond: %s(%s[sz=%d ptr=%v] : %s[sz=%d ptr=%v])\n",
			ind, c.Op,
			c.Left, c.LeftSize, c.LeftPtr,
			c.Right, c.RightSize, c.RightPtr)
		if len(s.IfTrue) > 0 {
			fmt.Printf("%s  True:\n", ind)
			dumpBFStmts(s.IfTrue, depth+2, blockIdx)
		}
		if len(s.IfFalse) > 0 {
			fmt.Printf("%s  False:\n", ind)
			dumpBFStmts(s.IfFalse, depth+2, blockIdx)
		}

	case backend.BFStmtReturn:
		fmt.Printf("%sReturn  size=%d  ptr=%v  = %s\n",
			ind, s.RetSize, s.RetPtr, s.RetExpr)

	case backend.BFStmtReturnVoid:
		fmt.Printf("%sReturn(void)\n", ind)

	case backend.BFStmtCall:
		fmt.Printf("%sCall(%s)  ret_size=%d  ret_ptr=%v\n",
			ind, s.CallName, s.CallRetSize, s.CallRetPtr)

	case backend.BFStmtArrStore:
		fmt.Printf("%sArrStore(%s)  elem_size=%d  elem_ptr=%v\n",
			ind, s.ArrName, s.ElemSize, s.ElemPtr)

	case backend.BFStmtBreak:
		fmt.Printf("%sBreak\n", ind)

	case backend.BFStmtContinue:
		fmt.Printf("%sContinue\n", ind)

	case backend.BFStmtRawMem:
		fmt.Printf("%sRawMem[risk]\n", ind)
		dumpBFStmts(s.RawMemBody, depth+1, blockIdx)
	}
}
