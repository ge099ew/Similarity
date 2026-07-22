package codegen

import (
	"similarity/ast"
	"similarity/lexer"
	"similarity/parser"
	"strings"
	"testing"
)

func generateIR(input string) string {
	l := lexer.New(input)
	tokens := l.Tokenize()
	prog := parser.New(tokens).ParseProgram()
	return New().Generate(prog)
}

func TestExplanationComment(t *testing.T) {
	ir := generateIR(`Explanation[Application{Game(type:RPG)}]`)
	if !strings.Contains(ir, "# Similarity - Application") {
		t.Errorf("Explanationのコメントがない:\n%s", ir)
	}
}

func TestVariableIR(t *testing.T) {
	ir := generateIR(`
Explanation[Application{App(type:test)}]
Function[main{
  receive{},
  Variable[let{int(x:10)}],
  return()
}]
`)
	t.Logf("生成されたIR:\n%s", ir)
	// メモリベース: alloc4 + storew
	if !strings.Contains(ir, "alloc4 4") {
		t.Errorf("alloc4 4がない（メモリベース変数）:\n%s", ir)
	}
	if !strings.Contains(ir, "storew 10") {
		t.Errorf("storew 10がない:\n%s", ir)
	}
}

func TestFuncIR(t *testing.T) {
	ir := generateIR(`
Function[main{
  receive{},
  Variable[let{int(hp:100)}],
  return(hp)
}]
`)
	t.Logf("生成されたIR:\n%s", ir)
	if !strings.Contains(ir, "function w $main") {
		t.Errorf("関数のIRが正しくない:\n%s", ir)
	}
	if !strings.Contains(ir, "storew 100") {
		t.Errorf("hp=100のstorewがない:\n%s", ir)
	}
	if !strings.Contains(ir, "loadw") {
		t.Errorf("return用のloadwがない:\n%s", ir)
	}
}

func TestHFTMode(t *testing.T) {
	ir := generateIR(`Explanation[System{HFT}]`)
	t.Logf("生成されたIR:\n%s", ir)
	if !strings.Contains(ir, "HFTモード") {
		t.Errorf("HFTモードのコメントがない:\n%s", ir)
	}
}

func TestFatalIR(t *testing.T) {
	prog := &ast.Program{
		Statements: []ast.Node{
			&ast.FuncNode{
				Name:   "main",
				Public: true,
				Body: []ast.Node{
					&ast.FatalNode{ErrType: "OutOfMemory", Msg: "回復不能"},
				},
			},
		},
	}
	ir := New().Generate(prog)
	t.Logf("生成されたIR:\n%s", ir)
	if !strings.Contains(ir, "Fatal") {
		t.Errorf("FatalのIRがない:\n%s", ir)
	}
}
