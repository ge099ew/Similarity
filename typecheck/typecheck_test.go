package typecheck

import (
	"similarity/lexer"
	"similarity/parser"
	"testing"
)

func parse(src string) *parser.Parser {
	l := lexer.New(src)
	tokens := l.Tokenize()
	return parser.New(tokens)
}

func check(src string) []*CheckError {
	p := parse(src)
	prog := p.ParseProgram()
	return New().Check(prog)
}

func hasErrorCode(errors []*CheckError, code string) bool {
	for _, e := range errors {
		if e.Code == code {
			return true
		}
	}
	return false
}

// ===== 型チェック =====

func TestTypeMismatch_IntFloat(t *testing.T) {
	src := `
Function[main{
  receive{},
  Variable[let{int(x:1.5)}],
  return{x}
}]
`
	errs := check(src)
	if !hasErrorCode(errs, "TC2001") {
		t.Errorf("int型にfloat値を代入したときTC2001が出るべき: %v", errs)
	}
}

func TestTypeMatch_IntInt(t *testing.T) {
	src := `
Function[main{
  receive{},
  Variable[let{int(x:10)}],
  return{x}
}]
`
	errs := check(src)
	if len(errs) > 0 {
		t.Errorf("int型にint値は正常なのにエラーが出た: %v", errs)
	}
}

// ===== 整数オーバーフロー =====

func TestIntOverflow(t *testing.T) {
	src := `
Function[main{
  receive{},
  Variable[let{int(x:9999999999)}],
  return{x}
}]
`
	errs := check(src)
	if !hasErrorCode(errs, "TC4001") {
		t.Errorf("オーバーフロー値でTC4001が出るべき: %v", errs)
	}
}

func TestNoOverflow(t *testing.T) {
	src := `
Function[main{
  receive{},
  Variable[let{int(x:2147483647)}],
  return{x}
}]
`
	errs := check(src)
	if hasErrorCode(errs, "TC4001") {
		t.Errorf("MaxInt32はオーバーフローしないはず: %v", errs)
	}
}

// ===== Mutation型チェック =====

func TestMutationUndeclared(t *testing.T) {
	src := `
Function[main{
  receive{},
  Mutation[variable{int(x:30)}],
  return{0}
}]
`
	errs := check(src)
	if !hasErrorCode(errs, "TC2002") {
		t.Errorf("未宣言変数へのMutationでTC2002が出るべき: %v", errs)
	}
}

// ===== deref安全チェック =====

func TestDerefOutsideRisk(t *testing.T) {
	src := `
Function[main{
  receive{},
  Variable[let{int(x:10)}],
  Variable[let{int(p:deref{x})}],
  return{p}
}]
`
	errs := check(src)
	if !hasErrorCode(errs, "TC3002") {
		t.Errorf("risk{}外のderefでTC3002が出るべき: %v", errs)
	}
}

// ===== cast チェック =====

func TestCastIntToFloat(t *testing.T) {
	src := `
Function[main{
  receive{},
  Variable[let{int(x:10)}],
  Variable[let{float(y:cast{float(x)})}],
  return{0}
}]
`
	errs := check(src)
	if len(errs) > 0 {
		t.Errorf("int→float castは正常なのにエラーが出た: %v", errs)
	}
}
