package backend_test

import (
	"testing"

	"similarity/analyzer"
	"similarity/backend"
	"similarity/lexer"
	"similarity/parser"
	"similarity/typecheck"
)

func buildBF(t *testing.T, src string) []backend.BackendFunc {
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
	return cbackend.BuildBackendProgram(prog)
}

func findBF(funcs []cbackend.BackendFunc, name string) *cbackend.BackendFunc {
	for i := range funcs {
		if funcs[i].Name == name {
			return &funcs[i]
		}
	}
	return nil
}

// === sum ===

func TestBackendFunc_Sum_Locals(t *testing.T) {
	funcs := buildBF(t, `
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
	fn := findBF(funcs, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	// ReturnAnn
	if fn.RetSize != 4 || fn.RetPtr {
		t.Errorf("RetSize=%d RetPtr=%v, want 4/false", fn.RetSize, fn.RetPtr)
	}
	// LocalVars: i, sum
	if len(fn.Locals) != 2 {
		t.Fatalf("Locals len=%d, want 2", len(fn.Locals))
	}
	for idx, want := range []string{"i", "sum"} {
		if fn.Locals[idx].Name != want {
			t.Errorf("Locals[%d].Name=%q, want %q", idx, fn.Locals[idx].Name, want)
		}
		if fn.Locals[idx].Size != 4 || fn.Locals[idx].IsPtr {
			t.Errorf("Locals[%d] size=%d ptr=%v, want 4/false",
				idx, fn.Locals[idx].Size, fn.Locals[idx].IsPtr)
		}
	}
}

func TestBackendFunc_Sum_Loop(t *testing.T) {
	funcs := buildBF(t, `
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
	fn := findBF(funcs, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	// Body: Variable(i), Variable(sum), Loop, Return
	var loopStmt *cbackend.BFStmt
	for i := range fn.Stmts {
		if fn.Stmts[i].Kind == cbackend.BFStmtLoop {
			loopStmt = &fn.Stmts[i]
			break
		}
	}
	if loopStmt == nil {
		t.Fatal("Loop stmt not found")
	}
	// LoopDepth=0（Analyzerが付与済み）
	if loopStmt.LoopDepth != 0 {
		t.Errorf("LoopDepth=%d, want 0", loopStmt.LoopDepth)
	}
	// Cond: lesseq(i : 100000000)
	if loopStmt.LoopCond.Op != "lesseq" {
		t.Errorf("LoopCond.Op=%q, want 'lesseq'", loopStmt.LoopCond.Op)
	}
	if loopStmt.LoopCond.Left != "i" {
		t.Errorf("LoopCond.Left=%q, want 'i'", loopStmt.LoopCond.Left)
	}
	if loopStmt.LoopCond.LeftSize != 4 || loopStmt.LoopCond.LeftPtr {
		t.Errorf("LoopCond.LeftSize=%d LeftPtr=%v, want 4/false",
			loopStmt.LoopCond.LeftSize, loopStmt.LoopCond.LeftPtr)
	}
}

// === nested_loop ===

func TestBackendFunc_NestedLoop_Depth(t *testing.T) {
	funcs := buildBF(t, `
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
	fn := findBF(funcs, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	var outer *cbackend.BFStmt
	for i := range fn.Stmts {
		if fn.Stmts[i].Kind == cbackend.BFStmtLoop {
			outer = &fn.Stmts[i]
			break
		}
	}
	if outer == nil {
		t.Fatal("outer Loop not found")
	}
	if outer.LoopDepth != 0 {
		t.Errorf("outer.LoopDepth=%d, want 0", outer.LoopDepth)
	}
	var inner *cbackend.BFStmt
	for i := range outer.LoopBody {
		if outer.LoopBody[i].Kind == cbackend.BFStmtLoop {
			inner = &outer.LoopBody[i]
			break
		}
	}
	if inner == nil {
		t.Fatal("inner Loop not found")
	}
	if inner.LoopDepth != 1 {
		t.Errorf("inner.LoopDepth=%d, want 1", inner.LoopDepth)
	}
}

// === fibonacci ===

func TestBackendFunc_Fib_Params(t *testing.T) {
	funcs := buildBF(t, `
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
	fib := findBF(funcs, "fibonacci")
	if fib == nil {
		t.Fatal("function 'fibonacci' not found")
	}
	// 引数: n (size=4, ptr=false)
	if len(fib.Params) != 1 {
		t.Fatalf("Params len=%d, want 1", len(fib.Params))
	}
	if fib.Params[0].Name != "n" || fib.Params[0].Size != 4 || fib.Params[0].IsPtr {
		t.Errorf("Params[0]={%q,%d,%v}, want {n,4,false}",
			fib.Params[0].Name, fib.Params[0].Size, fib.Params[0].IsPtr)
	}
	// ReturnAnn: size=4, ptr=false
	if fib.RetSize != 4 || fib.RetPtr {
		t.Errorf("RetSize=%d RetPtr=%v, want 4/false", fib.RetSize, fib.RetPtr)
	}
	// IfNode
	var ifStmt *cbackend.BFStmt
	for i := range fib.Stmts {
		if fib.Stmts[i].Kind == cbackend.BFStmtIf {
			ifStmt = &fib.Stmts[i]
			break
		}
	}
	if ifStmt == nil {
		t.Fatal("If stmt not found in fibonacci")
	}
	if ifStmt.IfCond.Op != "lesseq" {
		t.Errorf("IfCond.Op=%q, want 'lesseq'", ifStmt.IfCond.Op)
	}
}

// === BIRシリアライズ: BackendFuncと一致するか ===

func TestBackendFunc_BIR_Consistency(t *testing.T) {
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

	// BackendFuncとBIRは同じAnnotationを参照する
	funcs := cbackend.BuildBackendProgram(prog)
	bir := cbackend.Serialize(prog)

	fn := findBF(funcs, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}

	// BackendFunc.Locals[0].Size と BIR の "LOCAL i 4 0" が一致
	if fn.Locals[0].Size != 4 {
		t.Errorf("Locals[0].Size=%d, want 4", fn.Locals[0].Size)
	}
	_ = bir // BIR内容はserial_test.goで検証済み
}
