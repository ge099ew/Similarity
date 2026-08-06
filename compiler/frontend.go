// Package compiler: フロントエンドパス
// lexer → parser → AST → typecheck
package compiler

import (
	"fmt"
	"similarity/ast"
	"similarity/debug"
	"similarity/lexer"
	"similarity/parser"
	"similarity/typecheck"
)

// FrontendResult はフロントエンドパスの出力
type FrontendResult struct {
	Program *ast.Program
	Tokens  []lexer.Token // デバッグ用: --dump-tokens で使用
}

// RunFrontend はlexer→parser→typecheckを実行し、
// エラーがあればContextのDiagnosticsに積む。
// 成功時は FrontendResult を返す。失敗時は nil。
func RunFrontend(ctx *CompilerContext) *FrontendResult {
	ctx.Logf("frontend: lexer start")

	// ===== Lexer =====
	l := lexer.New(ctx.Source)
	tokens := l.Tokenize()

	// --dump-tokens
	if ctx.Options.DumpTokens {
		debug.DumpTokens(tokens)
	}

	if len(l.Errors) > 0 {
		fmt.Println("=== Lexer Errors ===")
		for _, e := range l.Errors {
			ctx.Diagnostics.AddError("LEX0001", e, ctx.InputFile, 0, 0)
			fmt.Println(e)
		}
	}

	// ===== Parser =====
	ctx.Logf("frontend: parser start")
	p := parser.New(tokens)
	prog := p.ParseProgram()

	// --dump-ast
	if ctx.Options.DumpAST {
		debug.DumpAST(prog)
	}

	if len(p.Errors) > 0 {
		fmt.Println("=== Parser Errors ===")
		for _, e := range p.Errors {
			ctx.Diagnostics.AddError("PRS0001", e, ctx.InputFile, 0, 0)
			fmt.Println(e)
		}
		if len(p.Errors) > 5 {
			fmt.Println("コンパイルを中断しました。パースエラーを修正してください。")
			return nil
		}
	}

	// ===== TypeCheck =====
	ctx.Logf("frontend: typecheck start")
	checker := typecheck.New()
	checkErrors := checker.Check(prog)

	// --dump-types
	if ctx.Options.DumpTypes {
		debug.DumpTypes(prog)
	}

	if len(checkErrors) > 0 {
		fmt.Println("=== Type Check Errors ===")
		for _, e := range checkErrors {
			ctx.Diagnostics.AddError(e.Code, e.Message, ctx.InputFile, e.Line, e.Col)
			fmt.Println(e)
		}
		fmt.Println("コンパイルを中断しました。型エラーを修正してください。")
		return nil
	}

	ctx.Logf("frontend: done")
	return &FrontendResult{Program: prog, Tokens: tokens}
}
