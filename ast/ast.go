// Package ast: .iia の構文木
package ast

// Node ASTの全ノードが実装するインターフェース
type Node interface {
	TokenLiteral() string
}

// Program ルートノード
type Program struct {
	Explanation *ExplanationNode
	Statements  []Node
}

// ExplanationNode ファイル先頭の宣言
// Explanation[Application{Game(type:RPG)}]
type ExplanationNode struct {
	Category string            // Application / Bridge / System / Module
	Args     map[string]string // type:RPG, name:Minecraft ...
}

// VariableNode 変数宣言
// Variable[let{int(x:10)}]
type VariableNode struct {
	Mutable bool   // true=let, false=unclet
	Type    string // int, float, String, Box_int ...
	Name    string
	Value   Node
}

// IfNode 条件分岐
// If[check{le(hp,0)}, True[...], False[...]]
type IfNode struct {
	Condition Node
	True      []Node
	False     []Node
}

// ReturnNode return
type ReturnNode struct {
	Value Node
}

func (r *ReturnNode) TokenLiteral() string { return "return" }

// LoopNode ループ
// Loop[for{int(i:0), lt(i,10), step{1}}, Body[...]]
type LoopNode struct {
	Kind      string // for / Count
	Init      Node
	Condition Node
	Step      int
	Body      []Node
}

// MutationNode
type MutationNode struct {
	Type  string
	Name  string
	Value Node
}

func (m *MutationNode) TokenLiteral() string { return "Mutation" }

// FuncNode 関数定義
// Func[名前{receive{...}, 処理, return{...}}]
type FuncNode struct {
	Name    string
	Public  bool // Func_pub かどうか
	Params  []VariableNode
	Body    []Node
	Returns Node
}

// CallNode 関数呼び出し
// call{関数名(引数)}
type CallNode struct {
	FuncName string
	Args     []Node
}

// ErrorNode エラーハンドリング
// Error[try{...}, Ok[...], Err[type{...}, msg{...}]]
type ErrorNode struct {
	Try     []Node
	Ok      []Node
	Err     []Node
	ErrType string
	Msg     string
	Pass    bool // pass{} = 呼び出し元へ投げる
}

// FatalNode 回復不能エラー
// Fatal[type{...}, msg{...}]
type FatalNode struct {
	ErrType string
	Msg     string
}

// ExternNode C外部関数宣言
// Extern[C{lib{"SDL2"}, func{...}}]
type ExternNode struct {
	Libs  []string
	Funcs []FuncNode
}

// ImportNode モジュール読み込み
// Import[discord{token}]
type ImportNode struct {
	Module  string
	Symbols []string // 空なら全部
}

func (p *Program) TokenLiteral() string         { return "" }
func (e *ExplanationNode) TokenLiteral() string { return "Explanation" }
func (v *VariableNode) TokenLiteral() string    { return "Variable" }
func (i *IfNode) TokenLiteral() string          { return "If" }
func (l *LoopNode) TokenLiteral() string        { return "Loop" }
func (f *FuncNode) TokenLiteral() string        { return "Func" }
func (c *CallNode) TokenLiteral() string        { return "call" }
func (e *ErrorNode) TokenLiteral() string       { return "Error" }
func (f *FatalNode) TokenLiteral() string       { return "Fatal" }
func (e *ExternNode) TokenLiteral() string      { return "Extern" }
func (i *ImportNode) TokenLiteral() string      { return "Import" }

// LiteralNode リテラル値（int, float, bool, String）
type LiteralNode struct {
	Kind  string // INT_LIT / FLOAT_LIT / BOOL_LIT / STRING_LIT / IDENT
	Value string
	Line  int
}

// ConditionNode 比較条件（le, lt, eq など）
type ConditionNode struct {
	Op    string // le / lt / eq / ge / gt / ne
	Left  string
	Right string
}

func (l *LiteralNode) TokenLiteral() string   { return l.Value }
func (c *ConditionNode) TokenLiteral() string { return c.Op }

// ExprNode 演算式（+{int(a,b)}, *{int(a,b)} など）
type ExprNode struct {
	Op    string // + - * /
	Left  Node
	Right Node
	Type  string // int / float など
}

func (e *ExprNode) TokenLiteral() string { return e.Op }

// 非同期
// Async[{処理}]
type AsyncNode struct {
	Body []Node
}

func (a *AsyncNode) TokenLiteral() string { return "Async" }

// share(x) → Async間で共有する変数を明示
type ShareNode struct {
	Name string
}

func (s *ShareNode) TokenLiteral() string { return "share" }

// Await[task]
type AwaitNode struct {
	Target string
}

func (a *AwaitNode) TokenLiteral() string { return "Await" }

// GPU[{処理}]
type GPUNode struct {
	Body []Node
}

func (g *GPUNode) TokenLiteral() string { return "GPU" }

// メモリ・ポインタ
// Mem[risk{...}] / Mem[Raw{...}]
type RawMemNode struct {
	Body      []Node
	LineStart int
	LineEnd   int
	Ops       []string // 使用されてるunsafe操作（deref, addr等）
}

func (r *RawMemNode) TokenLiteral() string { return "Mem" }

// ポインタのアドレス取得
// addr{x} → xのアドレスを取得
type AddressNode struct {
	Name string
}

func (a *AddressNode) TokenLiteral() string { return "addr" }

// ポインタの参照外し
// deref{ptr} → ptrが指す値を取得
type DerefNode struct {
	Name string
}

func (d *DerefNode) TokenLiteral() string { return "deref" }

// 型キャスト
// cast{int(x)} → xをintにキャスト
type CastNode struct {
	Type  string
	Value Node
}

func (c *CastNode) TokenLiteral() string { return "cast" }

// 配列アクセス
// index{arr(i)} → arr[i]
type IndexNode struct {
	Name  string
	Index Node
}

func (i *IndexNode) TokenLiteral() string { return "index" }

// ループ制御
// break{}
type BreakNode struct{}

func (b *BreakNode) TokenLiteral() string { return "break" }

// continue{}
type ContinueNode struct{}

func (c *ContinueNode) TokenLiteral() string { return "continue" }
