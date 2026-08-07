package backend_test

import (
	"testing"

	"similarity/analyzer"
	"similarity/backend"
	"similarity/lexer"
	"similarity/parser"
	"similarity/typecheck"
)

// ===== ヘルパー =====

func cfgOf(t *testing.T, src string) []*backend.CFG {
	t.Helper()
	l := lexer.New(src)
	tokens := l.Tokenize()
	p := parser.New(tokens)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse error: %v", p.Errors)
	}
	checker := typecheck.New()
	if errs := checker.Check(prog); len(errs) > 0 {
		t.Fatalf("typecheck error: %v", errs)
	}
	analyzer.New().Annotate(prog)
	funcs := backend.BuildBackendProgram(prog)
	return backend.BuildCFGProgram(funcs)
}

func findCFG(cfgs []*backend.CFG, name string) *backend.CFG {
	for _, c := range cfgs {
		if c.Func.Name == name {
			return c
		}
	}
	return nil
}

func findBlock(cfg *backend.CFG, kind backend.BlockKind) *backend.BasicBlock {
	for _, b := range cfg.Blocks {
		if b.Kind == kind {
			return b
		}
	}
	return nil
}

func countBlocks(cfg *backend.CFG, kind backend.BlockKind) int {
	n := 0
	for _, b := range cfg.Blocks {
		if b.Kind == kind {
			n++
		}
	}
	return n
}

// ===== Entry Block =====

func TestCFG_EntryBlock_AlwaysExists(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:test)}]
Function_public[main{receive{},Variable[let{int(x:1)}],return(x)}]`)
	cfg := findCFG(cfgs, "main")
	if cfg == nil {
		t.Fatal("cfg for 'main' not found")
	}
	if len(cfg.Blocks) == 0 {
		t.Fatal("no blocks in CFG")
	}
	if cfg.Blocks[0].Kind != backend.BlockEntry {
		t.Errorf("Blocks[0].Kind=%v, want BlockEntry", cfg.Blocks[0].Kind)
	}
}

func TestCFG_BlockIDs_Sequential(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:test)}]
Function_public[main{receive{},Variable[let{int(x:1)}],return(x)}]`)
	cfg := findCFG(cfgs, "main")
	if cfg == nil {
		t.Fatal("cfg for 'main' not found")
	}
	for i, blk := range cfg.Blocks {
		if blk.ID != i {
			t.Errorf("Blocks[%d].ID=%d, want %d", i, blk.ID, i)
		}
	}
}

// ===== 通常文 =====

func TestCFG_PlainStmts_InEntryBlock(t *testing.T) {
	// Variable/Mutation/Incr は Entry Block に追加される
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:test)}]
Function_public[main{
  receive{},
  Variable[let{int(x:0)}],
  Mutation[variable{int(x:1)}],
  ++{x},
  return(x)
}]`)
	cfg := findCFG(cfgs, "main")
	if cfg == nil {
		t.Fatal("cfg not found")
	}
	entry := findBlock(cfg, backend.BlockEntry)
	if entry == nil {
		t.Fatal("entry block not found")
	}
	// Variable / Mutation / Incr の3文が Entry に入っている
	if len(entry.Stmts) != 3 {
		t.Errorf("entry.Stmts len=%d, want 3", len(entry.Stmts))
	}
	if entry.Stmts[0].Kind != backend.BFStmtVariable {
		t.Errorf("Stmts[0].Kind=%v, want BFStmtVariable", entry.Stmts[0].Kind)
	}
	if entry.Stmts[1].Kind != backend.BFStmtMutation {
		t.Errorf("Stmts[1].Kind=%v, want BFStmtMutation", entry.Stmts[1].Kind)
	}
	if entry.Stmts[2].Kind != backend.BFStmtIncr {
		t.Errorf("Stmts[2].Kind=%v, want BFStmtIncr", entry.Stmts[2].Kind)
	}
}

// ===== Return =====

func TestCFG_Return_TerminatesBlock(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:test)}]
Function_public[main{receive{},Variable[let{int(x:1)}],return(x)}]`)
	cfg := findCFG(cfgs, "main")
	if cfg == nil {
		t.Fatal("cfg not found")
	}
	// ReturnBlockが存在する
	retBlk := findBlock(cfg, backend.BlockReturn)
	if retBlk == nil {
		t.Fatal("BlockReturn not found")
	}
	// ReturnBlockの後継は0（終端）
	if len(retBlk.Succ) != 0 {
		t.Errorf("ReturnBlock.Succ len=%d, want 0", len(retBlk.Succ))
	}
	// ReturnBlockにReturnStmtが入っている
	if len(retBlk.Stmts) == 0 || retBlk.Stmts[0].Kind != backend.BFStmtReturn {
		t.Errorf("ReturnBlock.Stmts[0].Kind=%v, want BFStmtReturn", retBlk.Stmts[0].Kind)
	}
}

// ===== If =====

func TestCFG_If_Blocks(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:fibonacci)}]
Function[fibonacci{
  receive{int(n)},
  If[check{lesseq(n:1)},
    True{return(n)},
    False{
      Variable[let{int(a:call{fibonacci(-{int(n,1)})})}],
      Variable[let{int(b:call{fibonacci(-{int(n,2)})})}],
      return(+{int(a,b)})
    }
  ]
}]
Function_public[main{
  receive{},
  Variable[let{int(result:call{fibonacci(40)})}],
  return(result)
}]`)
	cfg := findCFG(cfgs, "fibonacci")
	if cfg == nil {
		t.Fatal("cfg for 'fibonacci' not found")
	}

	// IfBlock が1つ存在する
	if countBlocks(cfg, backend.BlockIf) != 1 {
		t.Errorf("BlockIf count=%d, want 1", countBlocks(cfg, backend.BlockIf))
	}
	// IfTrueBlock が1つ存在する
	if countBlocks(cfg, backend.BlockIfTrue) != 1 {
		t.Errorf("BlockIfTrue count=%d, want 1", countBlocks(cfg, backend.BlockIfTrue))
	}
	// IfFalseBlock が1つ存在する
	if countBlocks(cfg, backend.BlockIfFalse) != 1 {
		t.Errorf("BlockIfFalse count=%d, want 1", countBlocks(cfg, backend.BlockIfFalse))
	}

	// IfBlock の Cond が正しい
	ifBlk := findBlock(cfg, backend.BlockIf)
	if ifBlk.Cond == nil {
		t.Fatal("IfBlock.Cond is nil")
	}
	if ifBlk.Cond.Op != "lesseq" {
		t.Errorf("IfBlock.Cond.Op=%q, want 'lesseq'", ifBlk.Cond.Op)
	}
	if ifBlk.Cond.Left != "n" {
		t.Errorf("IfBlock.Cond.Left=%q, want 'n'", ifBlk.Cond.Left)
	}

	// IfBlock → TrueBlock, FalseBlock の2後継
	if len(ifBlk.Succ) != 2 {
		t.Errorf("IfBlock.Succ len=%d, want 2", len(ifBlk.Succ))
	}
}

func TestCFG_If_EdgeStructure(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:fibonacci)}]
Function[fibonacci{
  receive{int(n)},
  If[check{lesseq(n:1)},
    True{return(n)},
    False{
      Variable[let{int(a:call{fibonacci(-{int(n,1)})})}],
      Variable[let{int(b:call{fibonacci(-{int(n,2)})})}],
      return(+{int(a,b)})
    }
  ]
}]
Function_public[main{receive{},Variable[let{int(r:call{fibonacci(5)})}],return(r)}]`)
	cfg := findCFG(cfgs, "fibonacci")
	if cfg == nil {
		t.Fatal("cfg not found")
	}

	ifBlk := findBlock(cfg, backend.BlockIf)
	if ifBlk == nil {
		t.Fatal("BlockIf not found")
	}
	trueBlk := findBlock(cfg, backend.BlockIfTrue)
	if trueBlk == nil {
		t.Fatal("BlockIfTrue not found")
	}
	falseBlk := findBlock(cfg, backend.BlockIfFalse)
	if falseBlk == nil {
		t.Fatal("BlockIfFalse not found")
	}

	// IfBlock の後継に TrueBlock と FalseBlock が含まれる
	succIDs := map[int]bool{}
	for _, s := range ifBlk.Succ {
		succIDs[s.ID] = true
	}
	if !succIDs[trueBlk.ID] {
		t.Errorf("IfBlock.Succ does not contain TrueBlock #%d", trueBlk.ID)
	}
	if !succIDs[falseBlk.ID] {
		t.Errorf("IfBlock.Succ does not contain FalseBlock #%d", falseBlk.ID)
	}

	// TrueBlock/FalseBlock の Pred に IfBlock が含まれる
	predHas := func(blk *backend.BasicBlock, id int) bool {
		for _, p := range blk.Pred {
			if p.ID == id {
				return true
			}
		}
		return false
	}
	if !predHas(trueBlk, ifBlk.ID) {
		t.Errorf("TrueBlock.Pred does not contain IfBlock #%d", ifBlk.ID)
	}
	if !predHas(falseBlk, ifBlk.ID) {
		t.Errorf("FalseBlock.Pred does not contain IfBlock #%d", ifBlk.ID)
	}
}

// ===== Loop =====

func TestCFG_Loop_Blocks(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:sum)}]
Function_public[main{
  receive{},
  Variable[let{int(i:0)}],
  Variable[let{int(sum:0)}],
  Loop[check{lesseq(i:100000000)},for{
    Mutation[variable{int(sum:+{int(sum,i)})}],
    ++{i}
  }],
  return(sum)
}]`)
	cfg := findCFG(cfgs, "main")
	if cfg == nil {
		t.Fatal("cfg not found")
	}

	// LoopHeader / LoopBody / LoopExit が1つずつ存在する
	if countBlocks(cfg, backend.BlockLoopHeader) != 1 {
		t.Errorf("BlockLoopHeader count=%d, want 1", countBlocks(cfg, backend.BlockLoopHeader))
	}
	if countBlocks(cfg, backend.BlockLoopBody) != 1 {
		t.Errorf("BlockLoopBody count=%d, want 1", countBlocks(cfg, backend.BlockLoopBody))
	}
	if countBlocks(cfg, backend.BlockLoopExit) != 1 {
		t.Errorf("BlockLoopExit count=%d, want 1", countBlocks(cfg, backend.BlockLoopExit))
	}

	// LoopHeader の Cond が正しい
	headerBlk := findBlock(cfg, backend.BlockLoopHeader)
	if headerBlk.Cond == nil {
		t.Fatal("LoopHeader.Cond is nil")
	}
	if headerBlk.Cond.Op != "lesseq" {
		t.Errorf("LoopHeader.Cond.Op=%q, want 'lesseq'", headerBlk.Cond.Op)
	}
}

func TestCFG_Loop_BackEdge(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:sum)}]
Function_public[main{
  receive{},
  Variable[let{int(i:0)}],
  Variable[let{int(sum:0)}],
  Loop[check{lesseq(i:100000000)},for{
    Mutation[variable{int(sum:+{int(sum,i)})}],
    ++{i}
  }],
  return(sum)
}]`)
	cfg := findCFG(cfgs, "main")
	if cfg == nil {
		t.Fatal("cfg not found")
	}

	headerBlk := findBlock(cfg, backend.BlockLoopHeader)
	bodyBlk := findBlock(cfg, backend.BlockLoopBody)
	exitBlk := findBlock(cfg, backend.BlockLoopExit)

	// LoopHeader → LoopBody（条件真）
	hasSucc := func(from, to *backend.BasicBlock) bool {
		for _, s := range from.Succ {
			if s.ID == to.ID {
				return true
			}
		}
		return false
	}
	if !hasSucc(headerBlk, bodyBlk) {
		t.Error("LoopHeader → LoopBody edge missing")
	}
	// LoopHeader → LoopExit（条件偽）
	if !hasSucc(headerBlk, exitBlk) {
		t.Error("LoopHeader → LoopExit edge missing")
	}
	// LoopBody → LoopHeader（バックエッジ）
	if !hasSucc(bodyBlk, headerBlk) {
		t.Error("LoopBody → LoopHeader back-edge missing")
	}
}

func TestCFG_Loop_BodyStmts(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:sum)}]
Function_public[main{
  receive{},
  Variable[let{int(i:0)}],
  Variable[let{int(sum:0)}],
  Loop[check{lesseq(i:100000000)},for{
    Mutation[variable{int(sum:+{int(sum,i)})}],
    ++{i}
  }],
  return(sum)
}]`)
	cfg := findCFG(cfgs, "main")
	if cfg == nil {
		t.Fatal("cfg not found")
	}
	bodyBlk := findBlock(cfg, backend.BlockLoopBody)
	if bodyBlk == nil {
		t.Fatal("LoopBody not found")
	}
	// Mutation と Incr がBodyに入っている
	if len(bodyBlk.Stmts) != 2 {
		t.Errorf("LoopBody.Stmts len=%d, want 2", len(bodyBlk.Stmts))
	}
	if bodyBlk.Stmts[0].Kind != backend.BFStmtMutation {
		t.Errorf("LoopBody.Stmts[0].Kind=%v, want BFStmtMutation", bodyBlk.Stmts[0].Kind)
	}
	if bodyBlk.Stmts[1].Kind != backend.BFStmtIncr {
		t.Errorf("LoopBody.Stmts[1].Kind=%v, want BFStmtIncr", bodyBlk.Stmts[1].Kind)
	}
}

// ===== NestedLoop =====

func TestCFG_NestedLoop_TwoHeaders(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:nested_loop)}]
Function_public[main{
  receive{},
  Variable[let{int(i:0)}],
  Variable[let{int(j:0)}],
  Variable[let{int(count:0)}],
  Loop[check{less(i:1000)},for{
    Mutation[variable{int(j:0)}],
    Loop[check{less(j:1000)},for{
      Mutation[variable{int(count:+{int(count,1)})}],
      ++{j}
    }],
    ++{i}
  }],
  return(count)
}]`)
	cfg := findCFG(cfgs, "main")
	if cfg == nil {
		t.Fatal("cfg not found")
	}
	// 外側・内側の2つのLoopHeader/Body/Exit
	if countBlocks(cfg, backend.BlockLoopHeader) != 2 {
		t.Errorf("BlockLoopHeader count=%d, want 2", countBlocks(cfg, backend.BlockLoopHeader))
	}
	if countBlocks(cfg, backend.BlockLoopBody) != 2 {
		t.Errorf("BlockLoopBody count=%d, want 2", countBlocks(cfg, backend.BlockLoopBody))
	}
	if countBlocks(cfg, backend.BlockLoopExit) != 2 {
		t.Errorf("BlockLoopExit count=%d, want 2", countBlocks(cfg, backend.BlockLoopExit))
	}
}

// ===== CFG.Dump =====

func TestCFG_Dump_ContainsKeywords(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:sum)}]
Function_public[main{
  receive{},
  Variable[let{int(i:0)}],
  Variable[let{int(sum:0)}],
  Loop[check{lesseq(i:100000000)},for{
    Mutation[variable{int(sum:+{int(sum,i)})}],
    ++{i}
  }],
  return(sum)
}]`)
	cfg := findCFG(cfgs, "main")
	if cfg == nil {
		t.Fatal("cfg not found")
	}
	dump := cfg.Dump()
	t.Log(dump)

	for _, kw := range []string{
		"Function: main",
		"kind=entry",
		"kind=loop_header",
		"kind=loop_body",
		"kind=loop_exit",
		"kind=return",
		"cond=lesseq",
	} {
		found := false
		for _, line := range splitLines(dump) {
			if contains(line, kw) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Dump missing keyword %q\n---\n%s", kw, dump)
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

// ===== BackendFunc は変更されない =====

func TestCFG_DoesNotModifyBackendFunc(t *testing.T) {
	src := `
Explanation[Application{Benchmark(type:sum)}]
Function_public[main{
  receive{},
  Variable[let{int(i:0)}],
  Variable[let{int(sum:0)}],
  Loop[check{lesseq(i:100000000)},for{
    Mutation[variable{int(sum:+{int(sum,i)})}],
    ++{i}
  }],
  return(sum)
}]`
	l := lexer.New(src)
	tokens := l.Tokenize()
	p := parser.New(tokens)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse error: %v", p.Errors)
	}
	checker := typecheck.New()
	if errs := checker.Check(prog); len(errs) > 0 {
		t.Fatalf("typecheck error: %v", errs)
	}
	analyzer.New().Annotate(prog)

	funcs := backend.BuildBackendProgram(prog)
	// CFG構築前のStmt数を記録
	stmtCountBefore := len(funcs[0].Stmts)

	backend.BuildCFGProgram(funcs)

	// CFG構築後もBackendFuncのStmt数が変わっていない
	if len(funcs[0].Stmts) != stmtCountBefore {
		t.Errorf("BackendFunc.Stmts len changed: %d → %d",
			stmtCountBefore, len(funcs[0].Stmts))
	}
}

// ===== ASTはCFGから参照されない =====
// CFG.Func は *BackendFunc であり *ast.FuncNode ではないことをコンパイル時に確認
func TestCFG_NoASTReference(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:test)}]
Function_public[main{receive{},Variable[let{int(x:1)}],return(x)}]`)
	cfg := findCFG(cfgs, "main")
	if cfg == nil {
		t.Fatal("cfg not found")
	}
	// CFG.Func は *BackendFunc（ASTを直接持たない）
	var _ *backend.BackendFunc = cfg.Func
	_ = cfg
}

// ===== Pred/Succ の対称性 =====

func TestCFG_PredSuccSymmetry(t *testing.T) {
	cfgs := cfgOf(t, `
Explanation[Application{Benchmark(type:sum)}]
Function_public[main{
  receive{},
  Variable[let{int(i:0)}],
  Variable[let{int(sum:0)}],
  Loop[check{lesseq(i:100000000)},for{
    Mutation[variable{int(sum:+{int(sum,i)})}],
    ++{i}
  }],
  return(sum)
}]`)
	cfg := findCFG(cfgs, "main")
	if cfg == nil {
		t.Fatal("cfg not found")
	}
	// 全ブロックについて: AがBのSuccならBのPredにAが含まれる
	for _, a := range cfg.Blocks {
		for _, b := range a.Succ {
			found := false
			for _, pred := range b.Pred {
				if pred.ID == a.ID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Block #%d is Succ of #%d but #%d.Pred does not contain #%d",
					b.ID, a.ID, b.ID, a.ID)
			}
		}
	}
}


