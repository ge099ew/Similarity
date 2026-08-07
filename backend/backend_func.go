// backend_func.go: BackendFunctionのGo側表現。
//
// Annotated ASTから変換するダンプ専用の軽量構造体。
// 型推論・名前解決は一切行わない。
// Analyzerが付与したAnn（size/is_ptr/LoopDepth等）のみを使用する。
package backend

import (
	"similarity/ast"
)

// ===== Variable =====

// BFVariable はBackendFunctionが扱う変数（引数・ローカル変数）。
// Analyzerが付与した Ann から生成する。
type BFVariable struct {
	Index  int    // v0, v1, ... の番号
	Name   string // 元の変数名
	Type   string // 元の型名文字列（表示用のみ）
	Size   int    // Ann.Size
	IsPtr  bool   // Ann.IsPtr
}

// ===== ブロック種別 =====

type BFBlockKind int

const (
	BFBlockEntry BFBlockKind = iota // 関数エントリ（通常文）
	BFBlockLoop                     // Loopブロック
	BFBlockIf                       // Ifブロック
	BFBlockTrue                     // Ifの真ブロック
	BFBlockFalse                    // Ifの偽ブロック
)

func (k BFBlockKind) String() string {
	switch k {
	case BFBlockEntry:
		return "entry"
	case BFBlockLoop:
		return "loop"
	case BFBlockIf:
		return "if"
	case BFBlockTrue:
		return "if.true"
	case BFBlockFalse:
		return "if.false"
	}
	return "unknown"
}

// ===== Statement =====

type BFStmtKind int

const (
	BFStmtVariable BFStmtKind = iota // Variable宣言（初期化あり）
	BFStmtMutation                   // Mutation（代入）
	BFStmtIncr                       // ++/--
	BFStmtLoop                       // Loopブロック
	BFStmtIf                         // Ifブロック
	BFStmtReturn                     // return
	BFStmtReturnVoid                 // return（void）
	BFStmtCall                       // 式文としてのCall
	BFStmtArrStore                   // 配列要素書き込み
	BFStmtBreak
	BFStmtContinue
	BFStmtRawMem // Mem[risk{}]
)

// BFCond はConditionNodeから生成した条件情報。
type BFCond struct {
	Op       string // le/lt/eq/ge/gt/ne
	Left     string // 左辺の名前（変数名 or リテラル）
	LeftSize int
	LeftPtr  bool
	Right    string // 右辺の名前（変数名 or リテラル）
	RightSize int
	RightPtr  bool
}

// BFStmt はBackendFunctionの文。
type BFStmt struct {
	Kind BFStmtKind

	// BFStmtVariable / BFStmtMutation
	VarName string
	VarSize int
	VarPtr  bool
	Expr    string // 式の文字列表現（表示用）

	// BFStmtIncr
	IncrName string
	IncrSize int
	IncrPtr  bool
	IncrOp   string // "++" or "--"

	// BFStmtLoop
	LoopDepth int
	LoopCond  BFCond
	LoopBody  []BFStmt

	// BFStmtIf
	IfCond     BFCond
	IfTrue     []BFStmt
	IfFalse    []BFStmt

	// BFStmtReturn
	RetSize int
	RetPtr  bool
	RetExpr string // 式の文字列表現（表示用）

	// BFStmtCall（式文として単独で呼ばれる場合）
	CallName      string
	CallRetSize   int
	CallRetPtr    bool

	// BFStmtArrStore
	ArrName     string
	ElemSize    int
	ElemPtr     bool

	// BFStmtRawMem
	RawMemBody []BFStmt
}

// ===== BackendFunc =====

// BackendFunc はAnnotated ASTの1関数に対応するGo側のダンプ用構造体。
// Cバックエンドが持つ BackendFunction と1対1で対応する。
type BackendFunc struct {
	Name      string
	IsPublic  bool
	Params    []BFVariable
	Locals    []BFVariable // LocalVarsから生成。v0, v1, ...と番号付け
	RetSize   int
	RetPtr    bool
	Stmts     []BFStmt
}

// ===== 変換: ast.FuncNode → BackendFunc =====

// BuildBackendFunc はAnnotated ast.FuncNodeからBackendFuncを構築する。
// Analyzerが付与したAnnotationのみを使用する。型推論・名前解決は行わない。
func BuildBackendFunc(fn *ast.FuncNode) BackendFunc {
	bf := BackendFunc{
		Name:     fn.Name,
		IsPublic: fn.Public,
		RetSize:  fn.ReturnAnn.Size,
		RetPtr:   fn.ReturnAnn.IsPtr,
	}

	// 引数: Params[i].Ann はAnalyzerが付与済み
	for i, p := range fn.Params {
		bf.Params = append(bf.Params, BFVariable{
			Index: i,
			Name:  p.Name,
			Type:  p.Type,
			Size:  p.Ann.Size,
			IsPtr: p.Ann.IsPtr,
		})
	}

	// ローカル変数: LocalVarsはAnalyzerが収集済み
	for i, lv := range fn.LocalVars {
		bf.Locals = append(bf.Locals, BFVariable{
			Index: i,
			Name:  lv.Name,
			Type:  lv.Type,
			Size:  lv.Ann.Size,
			IsPtr: lv.Ann.IsPtr,
		})
	}

	// Body → BFStmt列
	bf.Stmts = buildStmts(fn.Body)
	return bf
}

// BuildBackendProgram はast.ProgramからBackendFunc一覧を構築する。
func BuildBackendProgram(prog *ast.Program) []BackendFunc {
	var funcs []BackendFunc
	for _, stmt := range prog.Statements {
		fn, ok := stmt.(*ast.FuncNode)
		if !ok {
			continue
		}
		funcs = append(funcs, BuildBackendFunc(fn))
	}
	return funcs
}

// ===== Bodyのstmt変換 =====

func buildStmts(nodes []ast.Node) []BFStmt {
	var stmts []BFStmt
	for _, n := range nodes {
		if s, ok := buildStmt(n); ok {
			stmts = append(stmts, s)
		}
	}
	return stmts
}

func buildStmt(node ast.Node) (BFStmt, bool) {
	if node == nil {
		return BFStmt{}, false
	}
	switch n := node.(type) {
	case *ast.VariableNode:
		return BFStmt{
			Kind:    BFStmtVariable,
			VarName: n.Name,
			VarSize: n.Ann.Size,
			VarPtr:  n.Ann.IsPtr,
			Expr:    exprStr(n.Value),
		}, true

	case *ast.MutationNode:
		return BFStmt{
			Kind:    BFStmtMutation,
			VarName: n.Name,
			VarSize: n.Ann.Size,
			VarPtr:  n.Ann.IsPtr,
			Expr:    exprStr(n.Value),
		}, true

	case *ast.IncrNode:
		return BFStmt{
			Kind:     BFStmtIncr,
			IncrName: n.Name,
			IncrSize: n.Ann.Size,
			IncrPtr:  n.Ann.IsPtr,
			IncrOp:   n.Op,
		}, true

	case *ast.LoopNode:
		return BFStmt{
			Kind:      BFStmtLoop,
			LoopDepth: n.LoopDepth, // Analyzerが付与済み
			LoopCond:  buildCond(n.Condition),
			LoopBody:  buildStmts(n.Body),
		}, true

	case *ast.IfNode:
		return BFStmt{
			Kind:    BFStmtIf,
			IfCond:  buildCond(n.Condition),
			IfTrue:  buildStmts(n.True),
			IfFalse: buildStmts(n.False),
		}, true

	case *ast.ReturnNode:
		if n.Value == nil {
			return BFStmt{Kind: BFStmtReturnVoid}, true
		}
		return BFStmt{
			Kind:    BFStmtReturn,
			RetSize: n.Ann.Size,
			RetPtr:  n.Ann.IsPtr,
			RetExpr: exprStr(n.Value),
		}, true

	case *ast.ArrayStoreNode:
		return BFStmt{
			Kind:     BFStmtArrStore,
			ArrName:  n.Name,
			ElemSize: n.ElemAnn.Size,
			ElemPtr:  n.ElemAnn.IsPtr,
		}, true

	case *ast.RawMemNode:
		return BFStmt{
			Kind:       BFStmtRawMem,
			RawMemBody: buildStmts(n.Body),
		}, true

	case *ast.BreakNode:
		return BFStmt{Kind: BFStmtBreak}, true

	case *ast.ContinueNode:
		return BFStmt{Kind: BFStmtContinue}, true

	case *ast.CallNode:
		// 式文としてのCall（戻り値を使わない場合）
		return BFStmt{
			Kind:        BFStmtCall,
			CallName:    n.FuncName,
			CallRetSize: n.ReturnAnn.Size,
			CallRetPtr:  n.ReturnAnn.IsPtr,
		}, true
	}
	return BFStmt{}, false
}

// buildCond はConditionNodeからBFCondを構築する。
// ConditionNode.LeftAnn / RightAnn はAnalyzerが付与済み。
func buildCond(node ast.Node) BFCond {
	cond, ok := node.(*ast.ConditionNode)
	if !ok {
		return BFCond{Op: "ne", Left: "expr", Right: "0"}
	}
	return BFCond{
		Op:        cond.Op,
		Left:      cond.Left,
		LeftSize:  cond.LeftAnn.Size,
		LeftPtr:   cond.LeftAnn.IsPtr,
		Right:     cond.Right,
		RightSize: cond.RightAnn.Size,
		RightPtr:  cond.RightAnn.IsPtr,
	}
}

// ===== 式の文字列表現（表示用のみ） =====

func exprStr(node ast.Node) string {
	if node == nil {
		return "(none)"
	}
	switch n := node.(type) {
	case *ast.LiteralNode:
		switch n.Kind {
		case "IDENT":
			return n.Value
		case "INT_LIT", "FLOAT_LIT", "BOOL_LIT":
			return n.Value
		case "STRING_LIT":
			return "\"" + n.Value + "\""
		}
		return n.Value
	case *ast.ExprNode:
		return n.Op + "{" + n.Type + "(" + exprStr(n.Left) + "," + exprStr(n.Right) + ")}"
	case *ast.CallNode:
		return "call{" + n.FuncName + "(...)}"
	case *ast.CastNode:
		return "cast{" + n.Type + "(" + exprStr(n.Value) + ")}"
	case *ast.IndexNode:
		return "index{" + n.Name + "(...)}"
	case *ast.AddressNode:
		return "addr{" + n.Name + "}"
	case *ast.DerefNode:
		return "deref{" + n.Name + "}"
	}
	return "(...)"
}
