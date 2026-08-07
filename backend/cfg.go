// cfg.go: BackendFunction → CFG (Control Flow Graph) 変換。
//
// 入力: BackendFunc（AnalyzerやASTを直接参照しない）
// 出力: CFG（BasicBlock列 + Pred/Succ エッジ）
//
// 生成ルール:
//   - 全関数は Entry Block から開始する
//   - 通常文（Variable/Mutation/Incr/Call/ArrStore）は現在のBlockに追加
//   - If → IfBlock → TrueBlock / FalseBlock → MergeBlock
//   - Loop → LoopHeader → LoopBody → LoopHeader（繰り返し） / LoopExit（脱出）
//   - Return → ReturnBlock（終端）
//
// 未実装（今後のStage）:
//   - SSA / Phi Node
//   - Dominator Tree
//   - Liveness解析
//   - Register Allocation
package backend

import "fmt"

// ===== BlockKind =====

// BlockKind はBasicBlockの種別。
type BlockKind int

const (
	BlockEntry      BlockKind = iota // 関数エントリ
	BlockIf                          // If条件評価
	BlockIfTrue                      // Ifの真ブロック
	BlockIfFalse                     // Ifの偽ブロック
	BlockMerge                       // If合流点
	BlockLoopHeader                  // Loop条件評価（ループヘッダ）
	BlockLoopBody                    // Loopボディ
	BlockLoopExit                    // Loop脱出後
	BlockReturn                      // Return終端
	BlockGeneral                     // その他の一般ブロック
)

func (k BlockKind) String() string {
	switch k {
	case BlockEntry:
		return "entry"
	case BlockIf:
		return "if"
	case BlockIfTrue:
		return "if_true"
	case BlockIfFalse:
		return "if_false"
	case BlockMerge:
		return "merge"
	case BlockLoopHeader:
		return "loop_header"
	case BlockLoopBody:
		return "loop_body"
	case BlockLoopExit:
		return "loop_exit"
	case BlockReturn:
		return "return"
	case BlockGeneral:
		return "general"
	}
	return "unknown"
}

// ===== BasicBlock =====

// BasicBlock はCFGの1ノード。
type BasicBlock struct {
	ID    int
	Kind  BlockKind
	Stmts []BFStmt      // このブロックに属する文（IfやLoopは展開済み）
	Cond  *BFCond       // If/LoopHeaderブロックの条件（nilなら条件なし）
	Pred  []*BasicBlock // 前任ブロック
	Succ  []*BasicBlock // 後継ブロック
}

// ===== CFG =====

// CFG はBackendFuncから生成されるControl Flow Graph。
type CFG struct {
	Func   *BackendFunc   // 元のBackendFunc（変更しない）
	Blocks []*BasicBlock  // 全BasicBlock（ID順）
}

// ===== CFG Builder =====

// builder はCFG構築の内部状態。
type builder struct {
	cfg      *CFG
	nextID   int
	// ループスタック: break/continueの解決に使う
	loopHeaderStack []*BasicBlock
	loopExitStack   []*BasicBlock
}

func (b *builder) newBlock(kind BlockKind) *BasicBlock {
	blk := &BasicBlock{
		ID:   b.nextID,
		Kind: kind,
	}
	b.nextID++
	b.cfg.Blocks = append(b.cfg.Blocks, blk)
	return blk
}

// edge はfromからtoへの有向エッジを張る。
func edge(from, to *BasicBlock) {
	from.Succ = append(from.Succ, to)
	to.Pred = append(to.Pred, from)
}

// BuildCFG はBackendFuncからCFGを生成する。
// BackendFuncは変更しない。ASTやAnalyzerを参照しない。
func BuildCFG(fn *BackendFunc) *CFG {
	cfg := &CFG{Func: fn}
	b := &builder{cfg: cfg}

	// Entry Block（全関数の開始点）
	entry := b.newBlock(BlockEntry)

	// Stmtを再帰的に処理して BasicBlock列を構築する
	// 戻り値: stmtの処理後に「フォールスルーする現在のブロック」
	// nilが返ればそこで制御フローが終端している
	b.buildStmts(fn.Stmts, entry)

	return cfg
}

// BuildCFGProgram はBackendFunc一覧からCFG一覧を生成する。
func BuildCFGProgram(funcs []BackendFunc) []*CFG {
	cfgs := make([]*CFG, len(funcs))
	for i := range funcs {
		cfgs[i] = BuildCFG(&funcs[i])
	}
	return cfgs
}

// buildStmts はstmt列を処理して current ブロックに積んでいく。
// 制御フローが分岐する文（If/Loop/Return）はサブブロックを生成して接続する。
// 戻り値: フォールスルー先のブロック（終端した場合はnil）
func (b *builder) buildStmts(stmts []BFStmt, current *BasicBlock) *BasicBlock {
	for _, s := range stmts {
		if current == nil {
			// 終端後の文は到達不能（deadcode）: 無視する
			break
		}
		current = b.buildStmt(s, current)
	}
	return current
}

// buildStmt は1つのstmtを処理する。
func (b *builder) buildStmt(s BFStmt, current *BasicBlock) *BasicBlock {
	switch s.Kind {
	// --- 通常文: 現在のブロックに追加するだけ ---
	case BFStmtVariable, BFStmtMutation, BFStmtIncr,
		BFStmtCall, BFStmtArrStore:
		current.Stmts = append(current.Stmts, s)
		return current

	case BFStmtRawMem:
		// riskブロック内部はそのまま現在のブロックに展開
		return b.buildStmts(s.RawMemBody, current)

	// --- If ---
	case BFStmtIf:
		return b.buildIf(s, current)

	// --- Loop ---
	case BFStmtLoop:
		return b.buildLoop(s, current)

	// --- Return ---
	case BFStmtReturn, BFStmtReturnVoid:
		retBlk := b.newBlock(BlockReturn)
		retBlk.Stmts = append(retBlk.Stmts, s)
		edge(current, retBlk)
		return nil // 終端

	// --- Break ---
	case BFStmtBreak:
		if len(b.loopExitStack) > 0 {
			exitBlk := b.loopExitStack[len(b.loopExitStack)-1]
			edge(current, exitBlk)
		}
		return nil // 終端

	// --- Continue ---
	case BFStmtContinue:
		if len(b.loopHeaderStack) > 0 {
			headerBlk := b.loopHeaderStack[len(b.loopHeaderStack)-1]
			edge(current, headerBlk)
		}
		return nil // 終端
	}
	return current
}

// buildIf はIfStmtからIf/True/False/MergeブロックのCFGを構築する。
//
//	current → ifBlk → trueBlk → mergeBlk
//	                → falseBlk →
func (b *builder) buildIf(s BFStmt, current *BasicBlock) *BasicBlock {
	// Ifブロック（条件評価）
	ifBlk := b.newBlock(BlockIf)
	cond := s.IfCond
	ifBlk.Cond = &cond
	edge(current, ifBlk)

	// Mergeブロック（合流点）: 先に作っておいてTrue/Falseから繋ぐ
	mergeBlk := b.newBlock(BlockMerge)

	// Trueブロック
	trueBlk := b.newBlock(BlockIfTrue)
	edge(ifBlk, trueBlk)
	trueEnd := b.buildStmts(s.IfTrue, trueBlk)
	if trueEnd != nil {
		// Trueパスが終端しなかった場合はMergeへ繋ぐ
		edge(trueEnd, mergeBlk)
	}

	// Falseブロック
	falseBlk := b.newBlock(BlockIfFalse)
	edge(ifBlk, falseBlk)
	falseEnd := b.buildStmts(s.IfFalse, falseBlk)
	if falseEnd != nil {
		// Falseパスが終端しなかった場合はMergeへ繋ぐ
		edge(falseEnd, mergeBlk)
	}

	// FalseBodyが空の場合もFalseBlkからMergeへ繋ぐ（else節なし）
	// （空のFalseBodyはbuildStmtsがfalseBlkをそのまま返す）

	return mergeBlk
}

// buildLoop はLoopStmtからHeader/Body/ExitブロックのCFGを構築する。
//
//	current → headerBlk ─(true)→ bodyBlk → headerBlk
//	                    └(false)→ exitBlk
func (b *builder) buildLoop(s BFStmt, current *BasicBlock) *BasicBlock {
	// LoopHeaderブロック（条件評価）
	headerBlk := b.newBlock(BlockLoopHeader)
	cond := s.LoopCond
	headerBlk.Cond = &cond
	edge(current, headerBlk)

	// LoopExitブロック（条件偽で脱出）
	exitBlk := b.newBlock(BlockLoopExit)

	// break/continueの解決用スタックにpush
	b.loopHeaderStack = append(b.loopHeaderStack, headerBlk)
	b.loopExitStack = append(b.loopExitStack, exitBlk)

	// LoopBodyブロック
	bodyBlk := b.newBlock(BlockLoopBody)
	edge(headerBlk, bodyBlk)  // 条件真 → Body
	edge(headerBlk, exitBlk)  // 条件偽 → Exit

	bodyEnd := b.buildStmts(s.LoopBody, bodyBlk)
	if bodyEnd != nil {
		// Bodyが終端しなかった場合はHeaderへ戻るバックエッジ
		edge(bodyEnd, headerBlk)
	}

	// スタックからpop
	b.loopHeaderStack = b.loopHeaderStack[:len(b.loopHeaderStack)-1]
	b.loopExitStack = b.loopExitStack[:len(b.loopExitStack)-1]

	return exitBlk
}

// ===== CFGのテキスト表現（--dump-cfg用） =====

// Dump はCFGを人間が読める形式で返す。
func (c *CFG) Dump() string {
	pub := ""
	if c.Func.IsPublic {
		pub = " [public]"
	}
	out := fmt.Sprintf("===== Function: %s%s =====\n\n", c.Func.Name, pub)
	for _, blk := range c.Blocks {
		out += dumpBlock(blk)
	}
	out += "\n"
	return out
}

func dumpBlock(blk *BasicBlock) string {
	out := fmt.Sprintf("Block #%d\n", blk.ID)
	out += fmt.Sprintf("  kind=%s\n", blk.Kind)

	// 条件（If/LoopHeaderのみ）
	if blk.Cond != nil {
		c := blk.Cond
		out += fmt.Sprintf("  cond=%s(%s[sz=%d ptr=%v] : %s[sz=%d ptr=%v])\n",
			c.Op,
			c.Left, c.LeftSize, c.LeftPtr,
			c.Right, c.RightSize, c.RightPtr)
	}

	// 前任ブロック
	if len(blk.Pred) > 0 {
		out += "  pred=["
		for i, p := range blk.Pred {
			if i > 0 {
				out += ","
			}
			out += fmt.Sprintf("#%d", p.ID)
		}
		out += "]\n"
	}

	// 後継ブロック
	out += "  succ=["
	for i, s := range blk.Succ {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("#%d", s.ID)
	}
	out += "]\n"

	// 文
	if len(blk.Stmts) > 0 {
		out += "  stmts:\n"
		for _, s := range blk.Stmts {
			out += "    " + stmtSummary(s) + "\n"
		}
	}

	out += "--------------------------------\n"
	return out
}

func stmtSummary(s BFStmt) string {
	switch s.Kind {
	case BFStmtVariable:
		return fmt.Sprintf("Variable(%s sz=%d ptr=%v) = %s", s.VarName, s.VarSize, s.VarPtr, s.Expr)
	case BFStmtMutation:
		return fmt.Sprintf("Mutation(%s sz=%d ptr=%v) = %s", s.VarName, s.VarSize, s.VarPtr, s.Expr)
	case BFStmtIncr:
		return fmt.Sprintf("Incr(%s %s)", s.IncrName, s.IncrOp)
	case BFStmtCall:
		return fmt.Sprintf("Call(%s ret_sz=%d)", s.CallName, s.CallRetSize)
	case BFStmtArrStore:
		return fmt.Sprintf("ArrStore(%s elem_sz=%d)", s.ArrName, s.ElemSize)
	case BFStmtReturn:
		return fmt.Sprintf("Return(sz=%d ptr=%v) = %s", s.RetSize, s.RetPtr, s.RetExpr)
	case BFStmtReturnVoid:
		return "Return(void)"
	case BFStmtBreak:
		return "Break"
	case BFStmtContinue:
		return "Continue"
	}
	return fmt.Sprintf("Stmt(%d)", s.Kind)
}
