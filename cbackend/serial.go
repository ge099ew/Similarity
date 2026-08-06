// Package cbackend: Annotated ASTをCバックエンドへ渡すためのシリアライザ。
//
// GoはAnnotated ASTを独自テキスト形式（.bir: Backend IR）にシリアライズし、
// Cバックエンド（backend）がそのファイルを読んでBackendFunctionを構築する。
//
// BIR形式の設計方針:
//   - 1行1要素。ネストはインデントで表現しない（キーワードで区切る）
//   - Cのパーサが sscanf/strcmp だけで読める単純な構造にする
//   - Analyzerが付与したAnnotation情報をそのまま出力する（再計算しない）
//   - 型推論・名前解決は行わない（Analyzerが保証済みの情報のみ使う）
//
// BIR形式の概要:
//
//	FUNC <name> <is_public:0|1> <return_size> <return_is_ptr:0|1>
//	PARAM <name> <size> <is_ptr:0|1>
//	...
//	LOCAL <name> <size> <is_ptr:0|1>
//	...
//	BODY
//	  <stmt> ...
//	ENDFUNC
//
// stmtの種類:
//
//	STORE <dst_name> <size> <is_ptr:0|1> <expr>
//	  expr: LIT_INT <val> | LIT_STR <val> | IDENT <name> <size> <is_ptr>
//	       | EXPR <op> <size> <is_ptr> <lhs_expr> <rhs_expr>
//	       | CALL <funcname> <return_size> <return_is_ptr> <argc> [<arg_expr>...]
//	INCR <name> <size> <is_ptr:0|1> <op:INC|DEC>
//	LOOP <depth>
//	  COND <op> <left_name> <left_size> <left_is_ptr> <right_name> <right_size> <right_is_ptr>
//	  LOOPBODY
//	    <stmt>...
//	ENDLOOP
//	IF
//	  COND <op> <left> <left_size> <left_is_ptr> <right> <right_size> <right_is_ptr>
//	  IFTRUE
//	    <stmt>...
//	  IFFALSE
//	    <stmt>...
//	ENDIF
//	RET <size> <is_ptr:0|1> <expr>
//	RET_VOID
package cbackend

import (
	"fmt"
	"strings"

	"similarity/ast"
)

// Serialize はAnnotated ast.ProgramをBIR形式テキストに変換して返す。
// Analyzerが付与したAnnotationをそのまま使用する。型推論・名前解決は行わない。
func Serialize(prog *ast.Program) string {
	var sb strings.Builder
	sb.WriteString("BIR 1\n") // バージョンヘッダ
	for _, stmt := range prog.Statements {
		fn, ok := stmt.(*ast.FuncNode)
		if !ok {
			continue
		}
		serializeFunc(&sb, fn)
	}
	return sb.String()
}

func serializeFunc(sb *strings.Builder, fn *ast.FuncNode) {
	isPublic := 0
	if fn.Public {
		isPublic = 1
	}
	isPtr := 0
	if fn.ReturnAnn.IsPtr {
		isPtr = 1
	}
	// FUNC <name> <is_public> <return_size> <return_is_ptr>
	fmt.Fprintf(sb, "FUNC %s %d %d %d\n", fn.Name, isPublic, fn.ReturnAnn.Size, isPtr)

	// PARAM lines（Analyzerが付与したAnnを使う）
	for _, p := range fn.Params {
		pip := 0
		if p.Ann.IsPtr {
			pip = 1
		}
		fmt.Fprintf(sb, "PARAM %s %d %d\n", p.Name, p.Ann.Size, pip)
	}

	// LOCAL lines（FuncNode.LocalVarsを使う。引数は含まない）
	for _, lv := range fn.LocalVars {
		lip := 0
		if lv.Ann.IsPtr {
			lip = 1
		}
		fmt.Fprintf(sb, "LOCAL %s %d %d\n", lv.Name, lv.Ann.Size, lip)
	}

	// BODY
	sb.WriteString("BODY\n")
	serializeStmts(sb, fn.Body)

	sb.WriteString("ENDFUNC\n\n")
}

func serializeStmts(sb *strings.Builder, stmts []ast.Node) {
	for _, s := range stmts {
		serializeStmt(sb, s)
	}
}

func serializeStmt(sb *strings.Builder, node ast.Node) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.VariableNode:
		// STORE: 変数の初期化（宣言と代入を同時に行う）
		ip := 0
		if n.Ann.IsPtr {
			ip = 1
		}
		fmt.Fprintf(sb, "STORE %s %d %d ", n.Name, n.Ann.Size, ip)
		serializeExpr(sb, n.Value, n.Ann)
		sb.WriteByte('\n')

	case *ast.MutationNode:
		// STORE: 変数への代入（MutationNode.Annを使う）
		ip := 0
		if n.Ann.IsPtr {
			ip = 1
		}
		fmt.Fprintf(sb, "STORE %s %d %d ", n.Name, n.Ann.Size, ip)
		serializeExpr(sb, n.Value, n.Ann)
		sb.WriteByte('\n')

	case *ast.IncrNode:
		// INCR: ++/--
		ip := 0
		if n.Ann.IsPtr {
			ip = 1
		}
		op := "INC"
		if n.Op == "--" {
			op = "DEC"
		}
		fmt.Fprintf(sb, "INCR %s %d %d %s\n", n.Name, n.Ann.Size, ip, op)

	case *ast.LoopNode:
		// LOOP <depth>
		fmt.Fprintf(sb, "LOOP %d\n", n.LoopDepth)
		serializeCond(sb, n.Condition)
		sb.WriteString("LOOPBODY\n")
		serializeStmts(sb, n.Body)
		sb.WriteString("ENDLOOP\n")

	case *ast.IfNode:
		sb.WriteString("IF\n")
		serializeCond(sb, n.Condition)
		sb.WriteString("IFTRUE\n")
		serializeStmts(sb, n.True)
		sb.WriteString("IFFALSE\n")
		serializeStmts(sb, n.False)
		sb.WriteString("ENDIF\n")

	case *ast.ReturnNode:
		if n.Value == nil {
			sb.WriteString("RET_VOID\n")
			return
		}
		ip := 0
		if n.Ann.IsPtr {
			ip = 1
		}
		fmt.Fprintf(sb, "RET %d %d ", n.Ann.Size, ip)
		serializeExpr(sb, n.Value, n.Ann)
		sb.WriteByte('\n')

	case *ast.RawMemNode:
		// riskブロック: 内部のstmtをそのまま出力
		for _, s := range n.Body {
			serializeStmt(sb, s)
		}

	case *ast.ArrayStoreNode:
		// 配列要素書き込み: ARRSTORE <name> <elem_size> <elem_is_ptr> <index_expr> <value_expr>
		ip := 0
		if n.ElemAnn.IsPtr {
			ip = 1
		}
		fmt.Fprintf(sb, "ARRSTORE %s %d %d ", n.Name, n.ElemAnn.Size, ip)
		serializeExpr(sb, n.Index, ast.TypeSize{Size: 4})
		sb.WriteByte(' ')
		serializeExpr(sb, n.Value, n.ElemAnn)
		sb.WriteByte('\n')

	// BreakNode / ContinueNode
	case *ast.BreakNode:
		sb.WriteString("BREAK\n")
	case *ast.ContinueNode:
		sb.WriteString("CONTINUE\n")
	}
}

// serializeCond はConditionNodeをCOND行として出力する。
// ConditionNode.LeftAnn / RightAnn はAnalyzerが付与済み。
func serializeCond(sb *strings.Builder, node ast.Node) {
	cond, ok := node.(*ast.ConditionNode)
	if !ok {
		// ConditionNodeでない場合（ExprNodeが条件に来るケース）は非0チェックとして扱う
		sb.WriteString("COND ne ")
		serializeExpr(sb, node, ast.TypeSize{Size: 4})
		sb.WriteString(" LIT_INT 0\n")
		return
	}
	lip := 0
	if cond.LeftAnn.IsPtr {
		lip = 1
	}
	rip := 0
	if cond.RightAnn.IsPtr {
		rip = 1
	}
	// COND <op> <left> <left_size> <left_is_ptr> <right> <right_size> <right_is_ptr>
	fmt.Fprintf(sb, "COND %s %s %d %d %s %d %d\n",
		cond.Op,
		cond.Left, cond.LeftAnn.Size, lip,
		cond.Right, cond.RightAnn.Size, rip,
	)
}

// serializeExpr は式ノードをインライン形式で出力する。
// 形式: <kind> [<fields>...]
// Cのパーサはこれをscanfでトークンとして読む。
func serializeExpr(sb *strings.Builder, node ast.Node, hint ast.TypeSize) {
	if node == nil {
		// 空の式: サイズhintでゼロリテラルを出力
		fmt.Fprintf(sb, "LIT_INT 0")
		return
	}
	switch n := node.(type) {
	case *ast.LiteralNode:
		serializeLiteral(sb, n)

	case *ast.ExprNode:
		// EXPR <op> <result_size> <result_is_ptr> <lhs> <rhs>
		ip := 0
		if n.Ann.IsPtr {
			ip = 1
		}
		fmt.Fprintf(sb, "EXPR %s %d %d ", n.Op, n.Ann.Size, ip)
		serializeExpr(sb, n.Left, n.Ann)
		sb.WriteByte(' ')
		serializeExpr(sb, n.Right, n.Ann)

	case *ast.CallNode:
		// CALL <funcname> <return_size> <return_is_ptr> <argc> [<arg_expr>...]
		ip := 0
		if n.ReturnAnn.IsPtr {
			ip = 1
		}
		fmt.Fprintf(sb, "CALL %s %d %d %d", n.FuncName, n.ReturnAnn.Size, ip, len(n.Args))
		for _, arg := range n.Args {
			sb.WriteByte(' ')
			serializeExpr(sb, arg, ast.TypeSize{Size: 4})
		}

	case *ast.CastNode:
		// CAST <result_size> <result_is_ptr> <inner_expr>
		ip := 0
		if n.Ann.IsPtr {
			ip = 1
		}
		fmt.Fprintf(sb, "CAST %d %d ", n.Ann.Size, ip)
		serializeExpr(sb, n.Value, n.Ann)

	case *ast.IndexNode:
		// ARRLOAD <name> <elem_size> <elem_is_ptr> <index_expr>
		ip := 0
		if n.ElemAnn.IsPtr {
			ip = 1
		}
		fmt.Fprintf(sb, "ARRLOAD %s %d %d ", n.Name, n.ElemAnn.Size, ip)
		serializeExpr(sb, n.Index, ast.TypeSize{Size: 4})

	case *ast.AddressNode:
		// ADDR <name>
		fmt.Fprintf(sb, "ADDR %s", n.Name)

	case *ast.DerefNode:
		// DEREF <size> <name>  (DerefNodeはNameフィールドのみ持つ)
		fmt.Fprintf(sb, "DEREF %d %s", hint.Size, n.Name)

	default:
		// フォールバック: ゼロ
		sb.WriteString("LIT_INT 0")
	}
}

func serializeLiteral(sb *strings.Builder, n *ast.LiteralNode) {
	switch n.Kind {
	case "INT_LIT":
		fmt.Fprintf(sb, "LIT_INT %s", n.Value)
	case "FLOAT_LIT":
		fmt.Fprintf(sb, "LIT_FLOAT %s", n.Value)
	case "BOOL_LIT":
		v := "0"
		if n.Value == "true" {
			v = "1"
		}
		fmt.Fprintf(sb, "LIT_INT %s", v)
	case "STRING_LIT":
		// 文字列リテラルは .rodata へ。値をエスケープして出力
		escaped := strings.ReplaceAll(n.Value, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\n", "\\n")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		fmt.Fprintf(sb, "LIT_STR \"%s\"", escaped)
	case "IDENT":
		// 変数参照: IDENT <name> <size> <is_ptr>
		ip := 0
		if n.Ann.IsPtr {
			ip = 1
		}
		fmt.Fprintf(sb, "IDENT %s %d %d", n.Value, n.Ann.Size, ip)
	default:
		sb.WriteString("LIT_INT 0")
	}
}
