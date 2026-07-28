package main

import (
	"fmt"
	"os"
	"path/filepath"
	"similarity/compiler"
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

	// CAIパイプライン（CAI IR生成 → cai_conv → バイナリ）
	compiler.RunCAIPipeline(ctx, result.Program)

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
	fmt.Println("Usage: sim [--ir-only] [--verbose] <file.iia|file.sml>")
	fmt.Println("  --ir-only : CAI IRファイルのみ生成")
	fmt.Println("  --verbose : 詳細ログ出力")
}
