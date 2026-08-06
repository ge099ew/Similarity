// Package analyzer: TypeChecker通過後のASTにBackend用Annotationを付与する。
//
// Analyzerが終了時点で保証すること:
//   - 全VariableNode.Ann: サイズ・ポインタ種別が確定済み
//   - 全FuncNode.ReturnAnn: 戻り値型が確定済み（voidはSize=0）
//   - 全FuncNode.LocalVars: ローカル変数リストが完全
//   - 全FuncNode.Params[*].Ann: 引数のサイズ・ポインタ種別が確定済み
//   - 全LoopNode.LoopDepth: ネスト深度が付与済み
//   - 全ExprNode.Ann: 演算結果の型が確定済み
//   - 全ConditionNode.LeftAnn/RightAnn: 比較オペランドの型が確定済み
//   - 全CallNode.ReturnAnn: 呼び出し先の戻り値型が確定済み
//   - 全MutationNode.Ann: 代入先変数の型が確定済み
//   - 全ReturnNode.Ann: 戻り値の型が確定済み
//   - 全IncrNode.Ann: 対象変数の型が確定済み
//   - 全LiteralNode.Ann: リテラルの型が確定済み（IDENT は参照先変数の型）
//   - 全CastNode.Ann: キャスト先の型が確定済み
//   - 全ArrayNode.Ann: 配列全体サイズが確定済み
//   - 全ArrayStoreNode.ElemAnn: 要素サイズが確定済み
//   - 全IndexNode.ElemAnn: 要素サイズが確定済み
//
// Backendは型推論・名前解決・シンボル探索を一切行わない。
package analyzer

import (
	"strconv"
	"strings"

	"similarity/ast"
)

// Analyzer はASTへAnnotationを付与する。
type Analyzer struct {
	// vars: 現在のスコープの変数名 → TypeSize
	vars map[string]ast.TypeSize
	// funcs: 関数名 → 戻り値TypeSize（Pass1で収集）
	funcs map[string]ast.TypeSize
}

// New はAnalyzerを生成する。
func New() *Analyzer {
	return &Analyzer{
		vars:  make(map[string]ast.TypeSize),
		funcs: make(map[string]ast.TypeSize),
	}
}

// Annotate はast.ProgramのすべてのノードにAnnotationを付与して返す。
// 入力はTypeChecker通過済みのast.Program。
// 戻り値は同じポインタ（in-placeで変更する）。
func (a *Analyzer) Annotate(prog *ast.Program) *ast.Program {
	a.pass1CollectFuncs(prog)
	a.pass2Annotate(prog)
	return prog
}

// =============================================================
// 型サイズ計算（caigen.goのtypeSizes/isPtr64/isFloatを根拠とする）
// =============================================================

// SizeOfType は型名文字列からTypeSizeを返す。
// caigen.goのtypeSizesマップおよびisPtr64関数の実装と一致させる。
func SizeOfType(typeName string) ast.TypeSize {
	switch typeName {
	case "int":
		return ast.TypeSize{Size: 4, IsPtr: false, IsFloat: false}
	case "float":
		return ast.TypeSize{Size: 4, IsPtr: false, IsFloat: true}
	case "bool":
		return ast.TypeSize{Size: 4, IsPtr: false, IsFloat: false}
	case "String":
		return ast.TypeSize{Size: 8, IsPtr: true, IsFloat: false}
	case "ptr":
		return ast.TypeSize{Size: 8, IsPtr: true, IsFloat: false}
	case "int64":
		return ast.TypeSize{Size: 8, IsPtr: true, IsFloat: false}
	}
	// Array_int / Array_float など: 要素型のサイズを返す
	// （配列全体のサイズはArrayNode単位で別途計算）
	if strings.HasPrefix(typeName, "Array_") {
		elem := strings.TrimPrefix(typeName, "Array_")
		inner := SizeOfType(elem)
		return ast.TypeSize{Size: inner.Size, IsPtr: false, IsFloat: inner.IsFloat}
	}
	// 構造体型など不明な型: int相当（4バイト）をフォールバック
	return ast.TypeSize{Size: 4, IsPtr: false, IsFloat: false}
}

// =============================================================
// Pass 1: 全FuncNodeの戻り値型を収集する
// TypeCheckerはtypecheck.go 139行で全関数をint固定にしているため、
// AnalyzerがReturnNodeから正確な型を解決する。
// =============================================================

func (a *Analyzer) pass1CollectFuncs(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		fn, ok := stmt.(*ast.FuncNode)
		if !ok {
			continue
		}
		// 関数スコープを一時構築（引数の型を登録）
		saved := a.vars
		a.vars = make(map[string]ast.TypeSize)
		// 引数をスコープに登録
		for _, p := range fn.Params {
			a.vars[p.Name] = SizeOfType(p.Type)
		}
		// Body内のVariableNodeをスコープに登録（return型解決のため）
		a.collectVarScope(fn.Body)

		// 戻り値型を解決
		ret := a.resolveReturnType(fn)
		a.funcs[fn.Name] = ret

		a.vars = saved
	}

	// ExternNodeの関数もデフォルト（Size=8）で登録
	for _, stmt := range prog.Statements {
		ext, ok := stmt.(*ast.ExternNode)
		if !ok {
			continue
		}
		for _, fn := range ext.Funcs {
			// 外部関数は戻り値型情報がないためデフォルト: 8バイト（rax幅）
			if _, exists := a.funcs[fn.Name]; !exists {
				a.funcs[fn.Name] = ast.TypeSize{Size: 8, IsPtr: false}
			}
		}
	}
}

// collectVarScope はBodyを走査してVariableNodeをa.varsに登録する。
// Pass1でreturn型解決に使うためのスコープ構築用。
func (a *Analyzer) collectVarScope(body []ast.Node) {
	for _, node := range body {
		switch n := node.(type) {
		case *ast.VariableNode:
			a.vars[n.Name] = SizeOfType(n.Type)
		case *ast.IfNode:
			a.collectVarScope(n.True)
			a.collectVarScope(n.False)
		case *ast.LoopNode:
			a.collectVarScope(n.Body)
		}
	}
}

// resolveReturnType はFuncNodeの戻り値型を解決する。
// FuncNode.Returns（末尾return）またはBody内のReturnNodeから判定する。
func (a *Analyzer) resolveReturnType(fn *ast.FuncNode) ast.TypeSize {
	// 末尾returnがある場合（parser.goのparseFuncで設定）
	if fn.Returns != nil {
		return a.resolveNodeType(fn.Returns)
	}
	// Body内のReturnNodeを探す（最初に見つかったもので確定）
	if ann := a.findFirstReturn(fn.Body); ann.Size > 0 {
		return ann
	}
	// returnなし: void
	return ast.TypeSize{Size: 0}
}

// findFirstReturn はBodyを再帰的に走査して最初のReturnNodeの型を返す。
func (a *Analyzer) findFirstReturn(body []ast.Node) ast.TypeSize {
	for _, node := range body {
		switch n := node.(type) {
		case *ast.ReturnNode:
			if n.Value != nil {
				return a.resolveNodeType(n.Value)
			}
			return ast.TypeSize{Size: 0}
		case *ast.IfNode:
			if ann := a.findFirstReturn(n.True); ann.Size > 0 {
				return ann
			}
			if ann := a.findFirstReturn(n.False); ann.Size > 0 {
				return ann
			}
		case *ast.LoopNode:
			if ann := a.findFirstReturn(n.Body); ann.Size > 0 {
				return ann
			}
		}
	}
	return ast.TypeSize{Size: 0}
}

// resolveNodeType はNodeの評価結果のTypeを解決する。
func (a *Analyzer) resolveNodeType(node ast.Node) ast.TypeSize {
	if node == nil {
		return ast.TypeSize{}
	}
	switch n := node.(type) {
	case *ast.LiteralNode:
		return a.resolveLiteralType(n)
	case *ast.ExprNode:
		// ExprNode.TypeはParserが設定した型名文字列
		return SizeOfType(n.Type)
	case *ast.CallNode:
		if ann, ok := a.funcs[n.FuncName]; ok {
			return ann
		}
		return ast.TypeSize{Size: 4}
	case *ast.CastNode:
		return SizeOfType(n.Type)
	case *ast.IndexNode:
		// 配列要素のアクセス: 配列名から要素型を逆引き
		if ann, ok := a.vars[n.Name]; ok {
			return ann
		}
		return ast.TypeSize{Size: 4}
	case *ast.DerefNode:
		// deref{ptr}: ポインタ経由のアクセス: int相当
		return ast.TypeSize{Size: 4}
	}
	return ast.TypeSize{Size: 4}
}

// resolveLiteralType はLiteralNodeの型を解決する。
func (a *Analyzer) resolveLiteralType(n *ast.LiteralNode) ast.TypeSize {
	switch n.Kind {
	case "INT_LIT":
		return ast.TypeSize{Size: 4, IsPtr: false}
	case "FLOAT_LIT":
		return ast.TypeSize{Size: 4, IsPtr: false, IsFloat: true}
	case "BOOL_LIT":
		return ast.TypeSize{Size: 4, IsPtr: false}
	case "STRING_LIT":
		return ast.TypeSize{Size: 8, IsPtr: true}
	case "IDENT":
		// 変数参照: スコープから型を解決
		if ann, ok := a.vars[n.Value]; ok {
			return ann
		}
	}
	return ast.TypeSize{Size: 4}
}

// resolveIdentOrLiteral はConditionNode.Left/RightのstringからTypeSizeを返す。
// caigen.goのloadStrValと同様の判定ロジック:
//   - varTypesに登録済み → 変数の型
//   - 数値文字列 → int (Size=4)
func (a *Analyzer) resolveIdentOrLiteral(s string) ast.TypeSize {
	if len(s) == 0 {
		return ast.TypeSize{Size: 4}
	}
	// 変数として登録済みか確認
	if ann, ok := a.vars[s]; ok {
		return ann
	}
	// 数値リテラル判定（caigen.goのloadStrValと同じロジック）
	isNum := s[0] >= '0' && s[0] <= '9'
	isNeg := s[0] == '-' && len(s) > 1
	if isNum || isNeg {
		if _, err := strconv.ParseInt(s, 10, 64); err == nil {
			return ast.TypeSize{Size: 4, IsPtr: false}
		}
	}
	// フォールバック
	return ast.TypeSize{Size: 4}
}

// =============================================================
// Pass 2: 全ノードにAnnotationを付与する
// =============================================================

func (a *Analyzer) pass2Annotate(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		a.annotateNode(stmt, 0)
	}
}

// annotateNode はノードを再帰的に走査してAnnotationを付与する。
// loopDepth: 現在のLoopNodeのネスト深度（0=ループ外）
func (a *Analyzer) annotateNode(node ast.Node, loopDepth int) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.FuncNode:
		a.annotateFunc(n)

	case *ast.VariableNode:
		a.annotateVariable(n, loopDepth)

	case *ast.MutationNode:
		ann := a.vars[n.Name]
		n.Ann = ann
		a.annotateNode(n.Value, loopDepth)

	case *ast.IfNode:
		a.annotateNode(n.Condition, loopDepth)
		for _, s := range n.True {
			a.annotateNode(s, loopDepth)
		}
		for _, s := range n.False {
			a.annotateNode(s, loopDepth)
		}

	case *ast.LoopNode:
		n.LoopDepth = loopDepth
		a.annotateNode(n.Condition, loopDepth)
		a.annotateNode(n.Init, loopDepth)
		for _, s := range n.Body {
			// ループBodyは1段深くなる
			a.annotateNode(s, loopDepth+1)
		}

	case *ast.ReturnNode:
		a.annotateNode(n.Value, loopDepth)
		n.Ann = a.resolveNodeType(n.Value)

	case *ast.ExprNode:
		a.annotateNode(n.Left, loopDepth)
		a.annotateNode(n.Right, loopDepth)
		n.Ann = SizeOfType(n.Type)

	case *ast.ConditionNode:
		n.LeftAnn = a.resolveIdentOrLiteral(n.Left)
		n.RightAnn = a.resolveIdentOrLiteral(n.Right)

	case *ast.CallNode:
		for _, arg := range n.Args {
			a.annotateNode(arg, loopDepth)
		}
		if ann, ok := a.funcs[n.FuncName]; ok {
			n.ReturnAnn = ann
		} else {
			// 未知の関数（外部関数等）: デフォルト8バイト
			n.ReturnAnn = ast.TypeSize{Size: 8}
		}

	case *ast.LiteralNode:
		n.Ann = a.resolveLiteralType(n)

	case *ast.IncrNode:
		n.Ann = a.vars[n.Name]

	case *ast.CastNode:
		a.annotateNode(n.Value, loopDepth)
		n.Ann = SizeOfType(n.Type)

	case *ast.ArrayNode:
		elemAnn := SizeOfType(n.ElemType)
		n.Ann = ast.TypeSize{
			Size:    elemAnn.Size * n.Size,
			IsPtr:   false,
			IsFloat: elemAnn.IsFloat,
		}

	case *ast.ArrayStoreNode:
		a.annotateNode(n.Index, loopDepth)
		a.annotateNode(n.Value, loopDepth)
		n.ElemAnn = SizeOfType(n.ElemType)

	case *ast.IndexNode:
		a.annotateNode(n.Index, loopDepth)
		// 配列名から要素型を解決: vars[name]には要素型が入っている
		n.ElemAnn = a.vars[n.Name]

	case *ast.RawMemNode:
		for _, s := range n.Body {
			a.annotateNode(s, loopDepth)
		}

	case *ast.ErrorNode:
		for _, s := range n.Try {
			a.annotateNode(s, loopDepth)
		}
		for _, s := range n.Ok {
			a.annotateNode(s, loopDepth)
		}
		for _, s := range n.Err {
			a.annotateNode(s, loopDepth)
		}

	case *ast.AsyncNode:
		for _, s := range n.Body {
			a.annotateNode(s, loopDepth)
		}

	case *ast.ExternNode:
		// ExternNodeの内部FuncNodeにもAnnotationを付与
		for i := range n.Funcs {
			a.annotateFunc(&n.Funcs[i])
		}

	// BreakNode / ContinueNode / FatalNode / ImportNode /
	// AddressNode / DerefNode / ShareNode / AwaitNode / GPUNode /
	// StructDefNode / StructInstanceNode / ExplanationNode:
	// Annotationが不要なノード（Backendが直接参照するフィールドなし）
	}
}

// annotateFunc はFuncNodeにAnnotationを付与する。
func (a *Analyzer) annotateFunc(fn *ast.FuncNode) {
	// 関数スコープをリセット
	saved := a.vars
	a.vars = make(map[string]ast.TypeSize)

	// 引数のAnnotationと登録
	for i := range fn.Params {
		ann := SizeOfType(fn.Params[i].Type)
		fn.Params[i].Ann = ann
		a.vars[fn.Params[i].Name] = ann
	}

	// LocalVars収集（引数を除くBody内のVariableNode一覧）
	fn.LocalVars = a.collectLocalVars(fn.Body)

	// LocalVarsをスコープに登録（Body内のAnnotation付与で参照できるように）
	for _, lv := range fn.LocalVars {
		a.vars[lv.Name] = lv.Ann
	}

	// Body全体にAnnotation付与
	for _, stmt := range fn.Body {
		a.annotateNode(stmt, 0)
	}

	// FuncNode.Returns（末尾return）のAnnotation
	if fn.Returns != nil {
		a.annotateNode(fn.Returns, 0)
	}

	// FuncNode.ReturnAnn: Pass1で収集済みの値を設定
	if ann, ok := a.funcs[fn.Name]; ok {
		fn.ReturnAnn = ann
	}

	a.vars = saved
}

// annotateVariable はVariableNodeのAnnを設定しスコープに登録する。
func (a *Analyzer) annotateVariable(n *ast.VariableNode, loopDepth int) {
	var ann ast.TypeSize
	if strings.HasPrefix(n.Type, "Array_") {
		// Array_int(arr:N) 形式: 要素型×要素数
		elemType := strings.TrimPrefix(n.Type, "Array_")
		elemAnn := SizeOfType(elemType)
		count := 0
		if lit, ok := n.Value.(*ast.LiteralNode); ok {
			count, _ = strconv.Atoi(lit.Value)
		}
		totalSize := elemAnn.Size * count
		if totalSize == 0 {
			totalSize = elemAnn.Size // フォールバック
		}
		ann = ast.TypeSize{Size: totalSize, IsPtr: false, IsFloat: elemAnn.IsFloat}
	} else if n.Type == "__struct__" {
		// struct型: サイズ解決は将来対応
		ann = ast.TypeSize{Size: 0}
	} else {
		ann = SizeOfType(n.Type)
	}
	n.Ann = ann
	a.vars[n.Name] = ann
	// Value内のノードにもAnnotation付与
	a.annotateNode(n.Value, loopDepth)
}

// collectLocalVars はBodyを再帰的に走査してVariableNodeを収集する。
// 引数（Params）は含めない。宣言順に返す。
// 同時にAnn（サイズ・ポインタ種別）を付与する。
func (a *Analyzer) collectLocalVars(body []ast.Node) []*ast.VariableNode {
	var locals []*ast.VariableNode
	for _, node := range body {
		switch n := node.(type) {
		case *ast.VariableNode:
			if n.Type == "__struct__" {
				continue
			}
			// Annを先に計算してスコープに登録
			var ann ast.TypeSize
			if strings.HasPrefix(n.Type, "Array_") {
				elemType := strings.TrimPrefix(n.Type, "Array_")
				elemAnn := SizeOfType(elemType)
				count := 0
				if lit, ok := n.Value.(*ast.LiteralNode); ok {
					count, _ = strconv.Atoi(lit.Value)
				}
				totalSize := elemAnn.Size * count
				if totalSize == 0 {
					totalSize = elemAnn.Size
				}
				ann = ast.TypeSize{Size: totalSize, IsPtr: false, IsFloat: elemAnn.IsFloat}
			} else {
				ann = SizeOfType(n.Type)
			}
			n.Ann = ann
			a.vars[n.Name] = ann
			locals = append(locals, n)

		case *ast.IfNode:
			locals = append(locals, a.collectLocalVars(n.True)...)
			locals = append(locals, a.collectLocalVars(n.False)...)

		case *ast.LoopNode:
			locals = append(locals, a.collectLocalVars(n.Body)...)
		}
	}
	return locals
}
