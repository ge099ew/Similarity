package analyzer_test

import (
	"testing"

	"similarity/analyzer"
	"similarity/ast"
	"similarity/lexer"
	"similarity/parser"
	"similarity/typecheck"
)

// parse はiiaソース文字列をパースしてTypeCheck済みast.Programを返すヘルパー。
func parse(t *testing.T, src string) *ast.Program {
	t.Helper()
	l := lexer.New(src)
	tokens := l.Tokenize()
	p := parser.New(tokens)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse errors: %v", p.Errors)
	}
	checker := typecheck.New()
	errs := checker.Check(prog)
	if len(errs) > 0 {
		t.Fatalf("typecheck errors: %v", errs)
	}
	return prog
}

// annotate はparse後にAnnotatorを適用して返すヘルパー。
func annotate(t *testing.T, src string) *ast.Program {
	t.Helper()
	prog := parse(t, src)
	a := analyzer.New()
	return a.Annotate(prog)
}

// findFunc はProgram内から指定名のFuncNodeを返す。
func findFunc(prog *ast.Program, name string) *ast.FuncNode {
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*ast.FuncNode); ok && fn.Name == name {
			return fn
		}
	}
	return nil
}

// findLoop はBody内の最初のLoopNodeを返す（再帰なし）。
func findLoop(body []ast.Node) *ast.LoopNode {
	for _, n := range body {
		if loop, ok := n.(*ast.LoopNode); ok {
			return loop
		}
	}
	return nil
}

// findVar はLocalVars内から指定名のVariableNodeを返す。
func findVar(locals []*ast.VariableNode, name string) *ast.VariableNode {
	for _, v := range locals {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// =============================================================
// SizeOfType のテスト
// =============================================================

func TestSizeOfType(t *testing.T) {
	cases := []struct {
		typeName string
		wantSize int
		wantPtr  bool
		wantFlt  bool
	}{
		{"int", 4, false, false},
		{"float", 4, false, true},
		{"bool", 4, false, false},
		{"String", 8, true, false},
		{"ptr", 8, true, false},
		{"int64", 8, true, false},
		{"Array_int", 4, false, false},   // 要素サイズのみ
		{"Array_float", 4, false, true},  // 要素サイズ
		{"unknown", 4, false, false},     // フォールバック
	}
	for _, c := range cases {
		got := analyzer.SizeOfType(c.typeName)
		if got.Size != c.wantSize || got.IsPtr != c.wantPtr || got.IsFloat != c.wantFlt {
			t.Errorf("SizeOfType(%q) = {%d,%v,%v}, want {%d,%v,%v}",
				c.typeName, got.Size, got.IsPtr, got.IsFloat,
				c.wantSize, c.wantPtr, c.wantFlt)
		}
	}
}

// =============================================================
// bench_sum.iia に対するテスト
// =============================================================

const srcSum = `
Explanation[Application{Benchmark(type:sum)}]
Function_public[main{
  receive{},
  Variable[let{int(i:0)}],
  Variable[let{int(sum:0)}],
  Loop[
    check{lesseq(i:100000000)},
    for{
      Mutation[variable{int(sum:+{int(sum,i)})}],
      ++{i}
    }
  ],
  return(sum)
}]
`

func TestSum_LocalVars(t *testing.T) {
	prog := annotate(t, srcSum)
	fn := findFunc(prog, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	// LocalVarsにi, sumが含まれること
	if len(fn.LocalVars) != 2 {
		t.Fatalf("LocalVars len = %d, want 2", len(fn.LocalVars))
	}
	i := findVar(fn.LocalVars, "i")
	if i == nil {
		t.Fatal("LocalVars: 'i' not found")
	}
	if i.Ann.Size != 4 || i.Ann.IsPtr {
		t.Errorf("i.Ann = {%d,%v}, want {4,false}", i.Ann.Size, i.Ann.IsPtr)
	}
	s := findVar(fn.LocalVars, "sum")
	if s == nil {
		t.Fatal("LocalVars: 'sum' not found")
	}
	if s.Ann.Size != 4 || s.Ann.IsPtr {
		t.Errorf("sum.Ann = {%d,%v}, want {4,false}", s.Ann.Size, s.Ann.IsPtr)
	}
}

func TestSum_ReturnAnn(t *testing.T) {
	prog := annotate(t, srcSum)
	fn := findFunc(prog, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	if fn.ReturnAnn.Size != 4 || fn.ReturnAnn.IsPtr {
		t.Errorf("ReturnAnn = {%d,%v}, want {4,false}", fn.ReturnAnn.Size, fn.ReturnAnn.IsPtr)
	}
}

func TestSum_LoopDepth(t *testing.T) {
	prog := annotate(t, srcSum)
	fn := findFunc(prog, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	loop := findLoop(fn.Body)
	if loop == nil {
		t.Fatal("LoopNode not found in main body")
	}
	if loop.LoopDepth != 0 {
		t.Errorf("LoopDepth = %d, want 0", loop.LoopDepth)
	}
}

func TestSum_ConditionAnn(t *testing.T) {
	prog := annotate(t, srcSum)
	fn := findFunc(prog, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	loop := findLoop(fn.Body)
	if loop == nil {
		t.Fatal("LoopNode not found")
	}
	cond, ok := loop.Condition.(*ast.ConditionNode)
	if !ok {
		t.Fatal("loop.Condition is not ConditionNode")
	}
	// Left="i" → int (Size=4, IsPtr=false)
	if cond.LeftAnn.Size != 4 || cond.LeftAnn.IsPtr {
		t.Errorf("LeftAnn = {%d,%v}, want {4,false}", cond.LeftAnn.Size, cond.LeftAnn.IsPtr)
	}
	// Right="100000000" → 数値リテラル (Size=4, IsPtr=false)
	if cond.RightAnn.Size != 4 || cond.RightAnn.IsPtr {
		t.Errorf("RightAnn = {%d,%v}, want {4,false}", cond.RightAnn.Size, cond.RightAnn.IsPtr)
	}
}

func TestSum_MutationAnn(t *testing.T) {
	prog := annotate(t, srcSum)
	fn := findFunc(prog, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	loop := findLoop(fn.Body)
	if loop == nil {
		t.Fatal("LoopNode not found")
	}
	// Body[0] = MutationNode{Name:"sum"}
	mut, ok := loop.Body[0].(*ast.MutationNode)
	if !ok {
		t.Fatal("loop.Body[0] is not MutationNode")
	}
	if mut.Ann.Size != 4 || mut.Ann.IsPtr {
		t.Errorf("MutationNode.Ann = {%d,%v}, want {4,false}", mut.Ann.Size, mut.Ann.IsPtr)
	}
}

func TestSum_IncrAnn(t *testing.T) {
	prog := annotate(t, srcSum)
	fn := findFunc(prog, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	loop := findLoop(fn.Body)
	if loop == nil {
		t.Fatal("LoopNode not found")
	}
	// Body[1] = IncrNode{Name:"i"}
	incr, ok := loop.Body[1].(*ast.IncrNode)
	if !ok {
		t.Fatal("loop.Body[1] is not IncrNode")
	}
	if incr.Ann.Size != 4 || incr.Ann.IsPtr {
		t.Errorf("IncrNode.Ann = {%d,%v}, want {4,false}", incr.Ann.Size, incr.Ann.IsPtr)
	}
}

// =============================================================
// bench_nested_loop.iia に対するテスト
// =============================================================

const srcNestedLoop = `
Explanation[Application{Benchmark(type:nested_loop)}]
Function_public[main{
  receive{},
  Variable[let{int(i:0)}],
  Variable[let{int(j:0)}],
  Variable[let{int(count:0)}],
  Loop[
    check{less(i:1000)},
    for{
      Mutation[variable{int(j:0)}],
      Loop[
        check{less(j:1000)},
        for{
          Mutation[variable{int(count:+{int(count,1)})}],
          ++{j}
        }
      ],
      ++{i}
    }
  ],
  return(count)
}]
`

func TestNestedLoop_LoopDepth(t *testing.T) {
	prog := annotate(t, srcNestedLoop)
	fn := findFunc(prog, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	outer := findLoop(fn.Body)
	if outer == nil {
		t.Fatal("outer LoopNode not found")
	}
	if outer.LoopDepth != 0 {
		t.Errorf("outer.LoopDepth = %d, want 0", outer.LoopDepth)
	}
	inner := findLoop(outer.Body)
	if inner == nil {
		t.Fatal("inner LoopNode not found")
	}
	if inner.LoopDepth != 1 {
		t.Errorf("inner.LoopDepth = %d, want 1", inner.LoopDepth)
	}
}

func TestNestedLoop_LocalVars(t *testing.T) {
	prog := annotate(t, srcNestedLoop)
	fn := findFunc(prog, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	// i, j, count の3変数がLocalVarsに含まれること
	if len(fn.LocalVars) != 3 {
		t.Fatalf("LocalVars len = %d, want 3", len(fn.LocalVars))
	}
	for _, name := range []string{"i", "j", "count"} {
		v := findVar(fn.LocalVars, name)
		if v == nil {
			t.Errorf("LocalVars: %q not found", name)
			continue
		}
		if v.Ann.Size != 4 || v.Ann.IsPtr {
			t.Errorf("%s.Ann = {%d,%v}, want {4,false}", name, v.Ann.Size, v.Ann.IsPtr)
		}
	}
}

// =============================================================
// bench_matrix.iia に対するテスト（3重ループ）
// =============================================================

const srcMatrix = `
Explanation[Application{Benchmark(type:matrix)}]
Function_public[main{
  receive{},
  Variable[let{int(N:200)}],
  Variable[let{int(sum:0)}],
  Variable[let{int(i:0)}],
  Variable[let{int(j:0)}],
  Variable[let{int(k:0)}],
  Loop[
    check{less(i:N)},
    for{
      Mutation[variable{int(j:0)}],
      Loop[
        check{less(j:N)},
        for{
          Mutation[variable{int(k:0)}],
          Loop[
            check{less(k:N)},
            for{
              Mutation[variable{int(sum:+{int(sum,*{int(i,k)})})}],
              ++{k}
            }
          ],
          ++{j}
        }
      ],
      ++{i}
    }
  ],
  return(sum)
}]
`

func TestMatrix_LoopDepth(t *testing.T) {
	prog := annotate(t, srcMatrix)
	fn := findFunc(prog, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	outer := findLoop(fn.Body)
	if outer == nil {
		t.Fatal("outer LoopNode not found")
	}
	if outer.LoopDepth != 0 {
		t.Errorf("outer.LoopDepth = %d, want 0", outer.LoopDepth)
	}
	mid := findLoop(outer.Body)
	if mid == nil {
		t.Fatal("mid LoopNode not found")
	}
	if mid.LoopDepth != 1 {
		t.Errorf("mid.LoopDepth = %d, want 1", mid.LoopDepth)
	}
	inner := findLoop(mid.Body)
	if inner == nil {
		t.Fatal("inner LoopNode not found")
	}
	if inner.LoopDepth != 2 {
		t.Errorf("inner.LoopDepth = %d, want 2", inner.LoopDepth)
	}
}

// =============================================================
// bench_fib.iia に対するテスト（再帰・CallNode）
// =============================================================

const srcFib = `
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
}]
`

func TestFib_FuncReturnAnn(t *testing.T) {
	prog := annotate(t, srcFib)

	// fibonacci: ReturnAnn = {4, false}
	fib := findFunc(prog, "fibonacci")
	if fib == nil {
		t.Fatal("function 'fibonacci' not found")
	}
	if fib.ReturnAnn.Size != 4 || fib.ReturnAnn.IsPtr {
		t.Errorf("fibonacci.ReturnAnn = {%d,%v}, want {4,false}",
			fib.ReturnAnn.Size, fib.ReturnAnn.IsPtr)
	}

	// main: ReturnAnn = {4, false}
	main := findFunc(prog, "main")
	if main == nil {
		t.Fatal("function 'main' not found")
	}
	if main.ReturnAnn.Size != 4 || main.ReturnAnn.IsPtr {
		t.Errorf("main.ReturnAnn = {%d,%v}, want {4,false}",
			main.ReturnAnn.Size, main.ReturnAnn.IsPtr)
	}
}

func TestFib_ParamAnn(t *testing.T) {
	prog := annotate(t, srcFib)
	fib := findFunc(prog, "fibonacci")
	if fib == nil {
		t.Fatal("function 'fibonacci' not found")
	}
	if len(fib.Params) != 1 {
		t.Fatalf("fibonacci.Params len = %d, want 1", len(fib.Params))
	}
	if fib.Params[0].Ann.Size != 4 || fib.Params[0].Ann.IsPtr {
		t.Errorf("Params[0].Ann = {%d,%v}, want {4,false}",
			fib.Params[0].Ann.Size, fib.Params[0].Ann.IsPtr)
	}
}

func TestFib_CallNodeReturnAnn(t *testing.T) {
	prog := annotate(t, srcFib)
	main := findFunc(prog, "main")
	if main == nil {
		t.Fatal("function 'main' not found")
	}
	// main.LocalVars[0] = result, Value = CallNode{fibonacci}
	result := findVar(main.LocalVars, "result")
	if result == nil {
		t.Fatal("LocalVars: 'result' not found")
	}
	call, ok := result.Value.(*ast.CallNode)
	if !ok {
		t.Fatal("result.Value is not CallNode")
	}
	if call.ReturnAnn.Size != 4 || call.ReturnAnn.IsPtr {
		t.Errorf("CallNode.ReturnAnn = {%d,%v}, want {4,false}",
			call.ReturnAnn.Size, call.ReturnAnn.IsPtr)
	}
}

func TestFib_LocalVars(t *testing.T) {
	prog := annotate(t, srcFib)

	// fibonacci: LocalVars = [a, b]（IfノードのFalse内）
	fib := findFunc(prog, "fibonacci")
	if fib == nil {
		t.Fatal("function 'fibonacci' not found")
	}
	if len(fib.LocalVars) != 2 {
		t.Fatalf("fibonacci.LocalVars len = %d, want 2", len(fib.LocalVars))
	}
	for _, name := range []string{"a", "b"} {
		v := findVar(fib.LocalVars, name)
		if v == nil {
			t.Errorf("LocalVars: %q not found", name)
			continue
		}
		if v.Ann.Size != 4 || v.Ann.IsPtr {
			t.Errorf("%s.Ann = {%d,%v}, want {4,false}", name, v.Ann.Size, v.Ann.IsPtr)
		}
	}
}

// =============================================================
// bench_bubble_sort.iia に対するテスト（引数あり関数）
// =============================================================

const srcBubble = `
Explanation[Application{Benchmark(type:bubble_sort)}]
Function[bubble_sort_n{
  receive{int(n)},
  Variable[let{int(i:0)}],
  Variable[let{int(passes:0)}],
  Variable[let{int(total:*{int(n,n)})}],
  Loop[
    check{less(i:total)},
    for{
      Mutation[variable{int(passes:+{int(passes,1)})}],
      ++{i}
    }
  ],
  return(passes)
}]
Function_public[main{
  receive{},
  Variable[let{int(result:call{bubble_sort_n(5000)})}],
  return(result)
}]
`

func TestBubble_ParamAnn(t *testing.T) {
	prog := annotate(t, srcBubble)
	fn := findFunc(prog, "bubble_sort_n")
	if fn == nil {
		t.Fatal("function 'bubble_sort_n' not found")
	}
	if len(fn.Params) != 1 {
		t.Fatalf("Params len = %d, want 1", len(fn.Params))
	}
	// n: int → {4, false}
	if fn.Params[0].Ann.Size != 4 || fn.Params[0].Ann.IsPtr {
		t.Errorf("Params[0].Ann = {%d,%v}, want {4,false}",
			fn.Params[0].Ann.Size, fn.Params[0].Ann.IsPtr)
	}
}

func TestBubble_LocalVars(t *testing.T) {
	prog := annotate(t, srcBubble)
	fn := findFunc(prog, "bubble_sort_n")
	if fn == nil {
		t.Fatal("function 'bubble_sort_n' not found")
	}
	// LocalVars: i, passes, total（引数nは含まない）
	if len(fn.LocalVars) != 3 {
		t.Fatalf("LocalVars len = %d, want 3", len(fn.LocalVars))
	}
	for _, name := range []string{"i", "passes", "total"} {
		v := findVar(fn.LocalVars, name)
		if v == nil {
			t.Errorf("LocalVars: %q not found", name)
		}
	}
}

func TestBubble_CallReturnAnn(t *testing.T) {
	prog := annotate(t, srcBubble)
	main := findFunc(prog, "main")
	if main == nil {
		t.Fatal("function 'main' not found")
	}
	result := findVar(main.LocalVars, "result")
	if result == nil {
		t.Fatal("LocalVars: 'result' not found")
	}
	call, ok := result.Value.(*ast.CallNode)
	if !ok {
		t.Fatal("result.Value is not CallNode")
	}
	// bubble_sort_n の戻り値は int → {4, false}
	if call.ReturnAnn.Size != 4 || call.ReturnAnn.IsPtr {
		t.Errorf("CallNode.ReturnAnn = {%d,%v}, want {4,false}",
			call.ReturnAnn.Size, call.ReturnAnn.IsPtr)
	}
}

// =============================================================
// ExprNode のAnnotationテスト
// =============================================================

const srcExpr = `
Explanation[Application{Benchmark(type:expr)}]
Function_public[main{
  receive{},
  Variable[let{int(a:1)}],
  Variable[let{int(b:2)}],
  Variable[let{int(c:+{int(a,b)})}],
  return(c)
}]
`

func TestExpr_Ann(t *testing.T) {
	prog := annotate(t, srcExpr)
	fn := findFunc(prog, "main")
	if fn == nil {
		t.Fatal("function 'main' not found")
	}
	c := findVar(fn.LocalVars, "c")
	if c == nil {
		t.Fatal("LocalVars: 'c' not found")
	}
	expr, ok := c.Value.(*ast.ExprNode)
	if !ok {
		t.Fatal("c.Value is not ExprNode")
	}
	// +{int(a,b)} → Ann = {4, false}
	if expr.Ann.Size != 4 || expr.Ann.IsPtr {
		t.Errorf("ExprNode.Ann = {%d,%v}, want {4,false}", expr.Ann.Size, expr.Ann.IsPtr)
	}
}
