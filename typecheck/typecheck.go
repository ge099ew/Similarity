// Package typecheck: Similarityのコンパイル時安全性チェック
// 1. null安全性（Nullable型の明示宣言チェック）
// 2. 型チェック（コンパイル時型整合性）
// 3. 整数オーバーフロー検出（定数畳み込み範囲）
package typecheck

import (
	"fmt"
	"math"
	"similarity/ast"
	"strconv"
)

// ===== エラー型 =====

type CheckError struct {
	Line    int
	Message string
	Code    string
}

func (e *CheckError) Error() string {
	return fmt.Sprintf("TypeCheck Error: %s (%s)", e.Message, e.Code)
}

// ===== 型情報 =====

// Similarityの型カテゴリ
type TypeKind int

const (
	KindInt      TypeKind = iota
	KindFloat
	KindBool
	KindString
	KindBoxInt
	KindBoxFloat
	KindArrayInt
	KindArrayFloat
	KindArrayBool
	KindPtr
	KindNullable // ?型（null許容）
	KindUnknown
)

type TypeInfo struct {
	Kind     TypeKind
	Nullable bool   // ?int のような null許容型
	Base     string // 元の型名
}

func typeFromString(t string) TypeInfo {
	switch t {
	case "int":
		return TypeInfo{Kind: KindInt, Base: t}
	case "float":
		return TypeInfo{Kind: KindFloat, Base: t}
	case "bool":
		return TypeInfo{Kind: KindBool, Base: t}
	case "String":
		return TypeInfo{Kind: KindString, Base: t}
	case "Box_int":
		return TypeInfo{Kind: KindBoxInt, Base: t}
	case "Box_float":
		return TypeInfo{Kind: KindBoxFloat, Base: t}
	case "Array_int":
		return TypeInfo{Kind: KindArrayInt, Base: t}
	case "Array_float":
		return TypeInfo{Kind: KindArrayFloat, Base: t}
	case "Array_bool":
		return TypeInfo{Kind: KindArrayBool, Base: t}
	default:
		// ?int のようなnull許容型
		if len(t) > 1 && t[0] == '?' {
			inner := typeFromString(t[1:])
			inner.Nullable = true
			return inner
		}
		return TypeInfo{Kind: KindUnknown, Base: t}
	}
}

func (t TypeInfo) String() string {
	if t.Nullable {
		return "?" + t.Base
	}
	return t.Base
}

func (t TypeInfo) IsNumeric() bool {
	return t.Kind == KindInt || t.Kind == KindFloat
}

func (t TypeInfo) CompatibleWith(other TypeInfo) bool {
	// 同じ型はOK
	if t.Kind == other.Kind {
		return true
	}
	// int ↔ float は明示的castが必要（暗黙変換なし）
	return false
}

// ===== Checker =====

type Checker struct {
	errors  []*CheckError
	vars    map[string]TypeInfo // 変数名 → 型情報
	funcs   map[string]TypeInfo // 関数名 → 戻り値型
	inRisk  bool                // Mem[risk{}]内かどうか
}

func New() *Checker {
	return &Checker{
		vars:  make(map[string]TypeInfo),
		funcs: make(map[string]TypeInfo),
	}
}

func (c *Checker) addError(line int, code, msg string) {
	c.errors = append(c.errors, &CheckError{Line: line, Message: msg, Code: code})
}

// Check: プログラム全体をチェックしてエラー一覧を返す
func (c *Checker) Check(prog *ast.Program) []*CheckError {
	// Phase1: 関数シグネチャ収集
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*ast.FuncNode); ok {
			c.funcs[fn.Name] = TypeInfo{Kind: KindInt, Base: "int"} // デフォルトint
		}
	}

	// Phase2: 本体チェック
	for _, stmt := range prog.Statements {
		c.checkNode(stmt)
	}

	return c.errors
}

func (c *Checker) checkNode(node ast.Node) TypeInfo {
	if node == nil {
		return TypeInfo{Kind: KindUnknown}
	}
	switch n := node.(type) {
	case *ast.FuncNode:
		return c.checkFunc(n)
	case *ast.VariableNode:
		return c.checkVariable(n)
	case *ast.MutationNode:
		return c.checkMutation(n)
	case *ast.IfNode:
		return c.checkIf(n)
	case *ast.LoopNode:
		return c.checkLoop(n)
	case *ast.ReturnNode:
		return c.checkNode(n.Value)
	case *ast.CallNode:
		return c.checkCall(n)
	case *ast.ExprNode:
		return c.checkExpr(n)
	case *ast.LiteralNode:
		return c.checkLiteral(n)
	case *ast.CastNode:
		return c.checkCast(n)
	case *ast.AddressNode:
		return TypeInfo{Kind: KindPtr, Base: "ptr"}
	case *ast.DerefNode:
		return c.checkDeref(n)
	case *ast.IndexNode:
		return c.checkIndex(n)
	case *ast.RawMemNode:
		return c.checkRawMem(n)
	case *ast.AsyncNode:
		c.checkAsync(n)
	case *ast.ShareNode:
		// share(x): 宣言済み変数かチェック
		if _, ok := c.vars[n.Name]; !ok {
			c.addError(0, "TC5001", fmt.Sprintf(
				"share: '%s' は未宣言の変数です。Asyncブロックより前に宣言してください",
				n.Name,
			))
		}
	case *ast.AwaitNode:
		// Await対象の変数が存在するか
		if _, ok := c.vars[n.Target]; !ok {
			c.addError(0, "TC3001", fmt.Sprintf("Await: '%s' は未宣言の変数です", n.Target))
		}
	case *ast.FatalNode:
		// Fatal は常にOK（回復不能エラー）
	case *ast.ErrorNode:
		for _, s := range n.Try {
			c.checkNode(s)
		}
		for _, s := range n.Ok {
			c.checkNode(s)
		}
		for _, s := range n.Err {
			c.checkNode(s)
		}
	}
	return TypeInfo{Kind: KindUnknown}
}

// ===== Async =====

func (c *Checker) checkAsync(n *ast.AsyncNode) {
	// share(x)されてない変数への書き込みを検出
	sharedVars := map[string]bool{}
	for _, s := range n.Body {
		if sh, ok := s.(*ast.ShareNode); ok {
			sharedVars[sh.Name] = true
		}
	}
	for _, s := range n.Body {
		if mut, ok := s.(*ast.MutationNode); ok {
			if _, shared := sharedVars[mut.Name]; !shared {
				// share宣言なしにAsyncブロック内でMutationしようとしている
				c.addError(0, "TC5002", fmt.Sprintf(
					"データ競合: '%s' はAsyncブロック内で変更されていますが share(%s) が宣言されていません",
					mut.Name, mut.Name,
				))
			}
		}
		c.checkNode(s)
	}
}

// ===== 関数 =====

func (c *Checker) checkFunc(fn *ast.FuncNode) TypeInfo {
	// スコープ退避
	savedVars := c.vars
	c.vars = make(map[string]TypeInfo)
	for k, v := range savedVars {
		c.vars[k] = v
	}

	// パラメータを登録
	for _, p := range fn.Params {
		ti := typeFromString(p.Type)
		c.vars[p.Name] = ti
	}

	for _, stmt := range fn.Body {
		c.checkNode(stmt)
	}

	c.vars = savedVars
	return TypeInfo{Kind: KindInt, Base: "int"}
}

// ===== 変数宣言 =====

func (c *Checker) checkVariable(v *ast.VariableNode) TypeInfo {
	declType := typeFromString(v.Type)

	// 値が存在する場合、型整合性チェック
	if v.Value != nil {
		valType := c.checkNode(v.Value)

		// null許容でない型にnullを代入しようとしていないか
		if valType.Kind == KindUnknown && !declType.Nullable {
			// 値がnullの可能性がある場合
			// （現状ではnullリテラルがないので将来対応）
		}

		// 型ミスマッチチェック（KindUnknown=変数参照は許容）
		if valType.Kind != KindUnknown && !declType.CompatibleWith(valType) {
			c.addError(0, "TC2001", fmt.Sprintf(
				"型ミスマッチ: '%s' は %s 型ですが %s 型の値を代入しようとしています",
				v.Name, declType.String(), valType.String(),
			))
		}

		// 整数オーバーフロー検出（定数の場合）
		if declType.Kind == KindInt {
			if lit, ok := v.Value.(*ast.LiteralNode); ok && lit.Kind == "INT_LIT" {
				c.checkIntOverflow(lit)
			}
		}
	}

	// null許容でない型への宣言でnullが来ないかはパーサー側で保証（将来）
	c.vars[v.Name] = declType
	return declType
}

// ===== Mutation =====

func (c *Checker) checkMutation(m *ast.MutationNode) TypeInfo {
	varType, exists := c.vars[m.Name]
	if !exists {
		c.addError(0, "TC2002", fmt.Sprintf("Mutation: '%s' は未宣言の変数です", m.Name))
		return TypeInfo{Kind: KindUnknown}
	}

	// イミュータブル変数への代入チェック
	// （VariableNodeのMutableフラグを使う、将来的にVarEnvに持つ）

	newType := typeFromString(m.Type)
	if !varType.CompatibleWith(newType) {
		c.addError(0, "TC2003", fmt.Sprintf(
			"型ミスマッチ: '%s' は %s 型ですが %s 型で上書きしようとしています",
			m.Name, varType.String(), newType.String(),
		))
	}

	// 整数オーバーフロー検出
	if varType.Kind == KindInt && m.Value != nil {
		if lit, ok := m.Value.(*ast.LiteralNode); ok && lit.Kind == "INT_LIT" {
			c.checkIntOverflow(lit)
		}
	}

	return varType
}

// ===== If =====

func (c *Checker) checkIf(n *ast.IfNode) TypeInfo {
	if cond, ok := n.Condition.(*ast.ConditionNode); ok {
		// 比較対象の型が一致しているか
		leftType := c.resolveIdentType(cond.Left)
		rightType := c.resolveIdentType(cond.Right)
		if leftType.Kind != KindUnknown && rightType.Kind != KindUnknown {
			if !leftType.CompatibleWith(rightType) {
				c.addError(0, "TC2004", fmt.Sprintf(
					"比較型ミスマッチ: %s(%s) と %s(%s) は比較できません",
					cond.Left, leftType.String(), cond.Right, rightType.String(),
				))
			}
		}

		// null許容型のnullチェックなしアクセス検出
		if leftType.Nullable {
			c.addError(0, "TC1001", fmt.Sprintf(
				"null安全: '%s' はnull許容型です。nullチェックなしで比較しています",
				cond.Left,
			))
		}
		if rightType.Nullable {
			c.addError(0, "TC1001", fmt.Sprintf(
				"null安全: '%s' はnull許容型です。nullチェックなしで比較しています",
				cond.Right,
			))
		}
	}

	for _, s := range n.True {
		c.checkNode(s)
	}
	for _, s := range n.False {
		c.checkNode(s)
	}
	return TypeInfo{Kind: KindUnknown}
}

// ===== Loop =====

func (c *Checker) checkLoop(n *ast.LoopNode) TypeInfo {
	if n.Init != nil {
		c.checkNode(n.Init)
	}
	for _, s := range n.Body {
		c.checkNode(s)
	}
	return TypeInfo{Kind: KindUnknown}
}

// ===== 関数呼び出し =====

func (c *Checker) checkCall(n *ast.CallNode) TypeInfo {
	// 関数が存在するか（登録済みのもの）
	if retType, ok := c.funcs[n.FuncName]; ok {
		return retType
	}
	// 未登録=外部関数として許容
	return TypeInfo{Kind: KindUnknown}
}

// ===== 式 =====

func (c *Checker) checkExpr(n *ast.ExprNode) TypeInfo {
	leftType := c.checkNode(n.Left)
	rightType := c.checkNode(n.Right)

	// 両辺の型チェック（unknownは変数参照なので許容）
	if leftType.Kind != KindUnknown && rightType.Kind != KindUnknown {
		if !leftType.CompatibleWith(rightType) {
			c.addError(0, "TC2005", fmt.Sprintf(
				"演算型ミスマッチ: %s と %s は演算できません（明示的castが必要です）",
				leftType.String(), rightType.String(),
			))
		}
	}

	// 明示型があればそれを返す
	if n.Type != "" {
		return typeFromString(n.Type)
	}
	if leftType.Kind != KindUnknown {
		return leftType
	}
	return rightType
}

// ===== リテラル =====

func (c *Checker) checkLiteral(n *ast.LiteralNode) TypeInfo {
	switch n.Kind {
	case "INT_LIT":
		c.checkIntOverflow(n)
		return TypeInfo{Kind: KindInt, Base: "int"}
	case "FLOAT_LIT":
		return TypeInfo{Kind: KindFloat, Base: "float"}
	case "BOOL_LIT":
		return TypeInfo{Kind: KindBool, Base: "bool"}
	case "STRING_LIT":
		return TypeInfo{Kind: KindString, Base: "String"}
	case "IDENT":
		return c.resolveIdentType(n.Value)
	}
	return TypeInfo{Kind: KindUnknown}
}

// ===== cast =====

func (c *Checker) checkCast(n *ast.CastNode) TypeInfo {
	srcType := c.checkNode(n.Value)
	dstType := typeFromString(n.Type)

	// 数値型間のcastのみ許容
	if srcType.Kind != KindUnknown && !srcType.IsNumeric() {
		c.addError(0, "TC2006", fmt.Sprintf(
			"cast: %s は数値型ではないためcastできません",
			srcType.String(),
		))
	}
	if !dstType.IsNumeric() {
		c.addError(0, "TC2007", fmt.Sprintf(
			"cast: %s へのcastはサポートされていません",
			dstType.String(),
		))
	}

	return dstType
}

// ===== deref =====

func (c *Checker) checkDeref(n *ast.DerefNode) TypeInfo {
	// risk{}の外でのderefは警告
	if !c.inRisk {
		c.addError(0, "TC3002", fmt.Sprintf(
			"ポインタ: deref{%s} はMem[risk{}]の外で使用されています。意図的な場合はMem[risk{...}]で囲んでください",
			n.Name,
		))
	}
	return TypeInfo{Kind: KindInt, Base: "int"} // derefの結果型は暫定int
}

// ===== index =====

func (c *Checker) checkIndex(n *ast.IndexNode) TypeInfo {
	arrType, ok := c.vars[n.Name]
	if !ok {
		c.addError(0, "TC2008", fmt.Sprintf("index: '%s' は未宣言の配列です", n.Name))
		return TypeInfo{Kind: KindUnknown}
	}

	// 配列型チェック
	if arrType.Kind != KindArrayInt && arrType.Kind != KindArrayFloat && arrType.Kind != KindArrayBool {
		c.addError(0, "TC2009", fmt.Sprintf(
			"index: '%s' は配列型ではありません（%s）", n.Name, arrType.String(),
		))
	}

	// インデックスの型チェック
	idxType := c.checkNode(n.Index)
	if idxType.Kind != KindUnknown && idxType.Kind != KindInt {
		c.addError(0, "TC2010", fmt.Sprintf(
			"index: 配列インデックスはint型でなければなりません（%s が渡されました）",
			idxType.String(),
		))
	}

	// 要素型を返す
	switch arrType.Kind {
	case KindArrayFloat:
		return TypeInfo{Kind: KindFloat, Base: "float"}
	case KindArrayBool:
		return TypeInfo{Kind: KindBool, Base: "bool"}
	default:
		return TypeInfo{Kind: KindInt, Base: "int"}
	}
}

// ===== Mem[risk{}] =====

func (c *Checker) checkRawMem(n *ast.RawMemNode) TypeInfo {
	c.inRisk = true
	for _, s := range n.Body {
		c.checkNode(s)
	}
	c.inRisk = false
	return TypeInfo{Kind: KindUnknown}
}

// ===== 整数オーバーフロー検出 =====

func (c *Checker) checkIntOverflow(n *ast.LiteralNode) {
	val, err := strconv.ParseInt(n.Value, 10, 64)
	if err != nil {
		return
	}
	// 32bit int の範囲チェック（QBEのwはi32）
	if val > math.MaxInt32 || val < math.MinInt32 {
		c.addError(n.Line, "TC4001", fmt.Sprintf(
			"整数オーバーフロー: %d はint(32bit)の範囲[%d, %d]を超えています",
			val, math.MinInt32, math.MaxInt32,
		))
	}
}

// ===== ヘルパー =====

func (c *Checker) resolveIdentType(name string) TypeInfo {
	if ti, ok := c.vars[name]; ok {
		return ti
	}
	// 数値リテラルの場合
	if _, err := strconv.ParseInt(name, 10, 64); err == nil {
		return TypeInfo{Kind: KindInt, Base: "int"}
	}
	if _, err := strconv.ParseFloat(name, 64); err == nil {
		return TypeInfo{Kind: KindFloat, Base: "float"}
	}
	return TypeInfo{Kind: KindUnknown}
}
