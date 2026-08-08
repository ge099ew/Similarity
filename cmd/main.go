package main

import (
	"fmt"
	"os"
	"path/filepath"
	"similarity/analyzer"
	"similarity/backend"
	"similarity/compiler"
	"similarity/debug"
	"similarity/transpiler"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	opts := parseArgs(os.Args[1:])
	if opts.InputFile == "" {
		fmt.Println("Error: ファイルを指定してください")
		os.Exit(1)
	}

	src, filename, err := loadSource(opts.InputFile)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	opts.InputFile = filename
	opts.OutputDir = filepath.Dir(filename)

	// .sml → .iia トランスパイル
	if strings.HasSuffix(filename, ".sml") {
		iiaSource := transpiler.Transpile(src)
		iiaFile := strings.TrimSuffix(filename, ".sml") + ".iia"
		if err := os.WriteFile(iiaFile, []byte(iiaSource), 0644); err != nil {
			fmt.Println("トランスパイルエラー:", err)
			os.Exit(1)
		}
		fmt.Printf("Transpile → %s\n", iiaFile)
		src = iiaSource
		opts.InputFile = iiaFile
	}

	if opts.UseCAI {
		runCAI(src, opts)
	} else {
		fmt.Println("Error: QBEバックエンドは廃止されました。--cai を使用してください。")
		os.Exit(1)
	}
}

func runCAI(src string, opts compiler.Options) {
	ctx := compiler.New(opts)
	ctx.Source = src

	// Cell読み込み
	celFile := compiler.LoadCell(ctx)

	// フロントエンド（lexer → parser → typecheck）
	result := compiler.RunFrontend(ctx)
	if result == nil {
		os.Exit(1)
	}

	// Import依存チェック
	if !compiler.CheckImports(ctx, celFile, result.Program) {
		os.Exit(1)
	}

	// Analyzer（ASTにAnnotationを付与）
	a := analyzer.New()
	a.Annotate(result.Program)

	// --dump-analyzer
	if opts.DumpAnalyzer {
		debug.DumpAnalyzer(result.Program)
	}

	// BackendFunction生成（Go側の責務はここまで）
	bfuncs := backend.BuildBackendProgram(result.Program)

	// --dump-backend
	if opts.DumpBackend {
		debug.DumpBackend(bfuncs)
	}

	// BIRシリアライズ + Cバックエンド呼び出し（--dump-cfg 等のフラグを渡す）
	birFile := backend.BIRPath(ctx.InputFile)
	outFile := backend.OutPath(ctx.InputFile)
	runOpts := backend.RunOptions{
		DumpCFG: opts.DumpCFG,
	}
	if err := backend.Run(result.Program, birFile, outFile, runOpts); err != nil {
		fmt.Fprintf(os.Stderr, "Cバックエンド失敗: %v\n", err)
	}

	if opts.DumpRegAlloc {
		debug.DumpRegAlloc()
	}
	if opts.DumpMachine {
		debug.DumpMachine()
	}

	// CAIパイプライン（旧バックエンド: 将来廃止予定）
	if !opts.DumpCFG {
		compiler.RunCAIPipeline(ctx, result.Program)
	}

	// エラーがあれば終了コード1
	if ctx.Diagnostics.HasErrors() {
		os.Exit(1)
	}
}

func parseArgs(args []string) compiler.Options {
	opts := compiler.Options{}
	for _, arg := range args {
		switch arg {
		case "--ir-only":
			opts.IROnly = true
		case "--cai":
			opts.UseCAI = true
		case "--verbose":
			opts.Verbose = true
		case "--dump-tokens":
			opts.DumpTokens = true
		case "--dump-ast":
			opts.DumpAST = true
		case "--dump-types":
			opts.DumpTypes = true
		case "--dump-analyzer":
			opts.DumpAnalyzer = true
		case "--dump-backend":
			opts.DumpBackend = true
		case "--dump-cfg":
			opts.DumpCFG = true
		case "--dump-regalloc":
			opts.DumpRegAlloc = true
		case "--dump-machine":
			opts.DumpMachine = true
		default:
			opts.InputFile = arg
		}
	}
	// デフォルトでCAIを使う
	opts.UseCAI = true
	return opts
}

func loadSource(filename string) (string, string, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return "", filename, err
	}
	return string(b), filename, nil
}

func printUsage() {
	fmt.Println("Usage: sim [options] <file.iia|file.sml>")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --ir-only        CAI IRファイルのみ生成")
	fmt.Println("  --verbose        詳細ログ出力")
	fmt.Println()
	fmt.Println("Debug options:")
	fmt.Println("  --dump-tokens    Lexer出力を表示")
	fmt.Println("  --dump-ast       Parser出力（AST）を表示")
	fmt.Println("  --dump-types     TypeChecker通過後の型情報を表示")
	fmt.Println("  --dump-analyzer  Analyzer通過後のAnnotation付きASTを表示")
	fmt.Println("  --dump-backend   BackendFunction生成直後の内容を表示")
	fmt.Println("  --dump-cfg       CFG生成直後の内容を表示")
	fmt.Println("  --dump-regalloc  レジスタ割り当て結果を表示（Backend実装後に有効化）")
	fmt.Println("  --dump-machine   最終機械語を表示（Backend実装後に有効化）")
}
