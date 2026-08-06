package cbackend_test

import (
	"strings"
	"testing"

	"similarity/analyzer"
	"similarity/cbackend"
	"similarity/lexer"
	"similarity/parser"
	"similarity/typecheck"
)

func birOf(t *testing.T, src string) string {
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
	return cbackend.Serialize(prog)
}

func mustContain(t *testing.T, bir string, checks []string) {
	t.Helper()
	for _, c := range checks {
		if !strings.Contains(bir, c) {
			t.Errorf("missing %q in BIR:\n%s", c, bir)
		}
	}
}

func TestSerialize_Sum(t *testing.T) {
	bir := birOf(t, `
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
	t.Log(bir)
	mustContain(t, bir, []string{
		"BIR 1",
		"FUNC main 1 4 0",
		"LOCAL i 4 0",
		"LOCAL sum 4 0",
		"BODY",
		"LOOP 0",
		"COND lesseq",
		"LOOPBODY",
		"STORE sum",
		"INCR i",
		"ENDLOOP",
		"RET 4 0",
		"ENDFUNC",
	})
}

func TestSerialize_Fib(t *testing.T) {
	bir := birOf(t, `
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
	t.Log(bir)
	mustContain(t, bir, []string{
		"FUNC fibonacci 0 4 0",
		"PARAM n 4 0",
		"FUNC main 1 4 0",
		"LOCAL result 4 0",
		"CALL fibonacci 4 0",
		"IF",
		"COND lesseq",
		"IFTRUE",
		"IFFALSE",
		"ENDIF",
	})
}

func TestSerialize_NestedLoop(t *testing.T) {
	bir := birOf(t, `
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
	t.Log(bir)
	if !strings.Contains(bir, "LOOP 0") {
		t.Errorf("outer LOOP 0 not found\n%s", bir)
	}
	if !strings.Contains(bir, "LOOP 1") {
		t.Errorf("inner LOOP 1 not found\n%s", bir)
	}
}

func TestSerialize_Header(t *testing.T) {
	bir := birOf(t, `
Explanation[Application{Benchmark(type:sum)}]
Function_public[main{receive{},Variable[let{int(x:1)}],return(x)}]`)
	lines := strings.Split(bir, "\n")
	if lines[0] != "BIR 1" {
		t.Errorf("first line = %q, want 'BIR 1'", lines[0])
	}
}
