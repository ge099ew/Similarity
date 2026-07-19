package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"similarity/ast"
	"similarity/caigen"
	"similarity/cel"
	"similarity/cgen"
	"similarity/codegen"
	"similarity/echo"
	"similarity/lexer"
	"similarity/parser"
	"similarity/transpiler"
	"similarity/typecheck"
	"strings"
)

func compile(input, baseName, dir string, irOnly bool) {
	// ① .celファイルの読み込み（ir-onlyモードではスキップ）
	var celFile *cel.CelFile
	if !irOnly {
		var celErr error
		celFile, celErr = cel.Load(dir)
		if celErr != nil {
			fmt.Println("=== Cell Error ===")
			fmt.Println(celErr)
		}
		if celFile != nil {
			fmt.Print(celFile.Info())
			fmt.Println()
		}
	}

	// ② lexer → parser → AST
	l := lexer.New(input)
	tokens := l.Tokenize()
	if len(l.Errors) > 0 {
		fmt.Println("=== Lexer Errors ===")
		for _, e := range l.Errors {
			fmt.Println(e)
		}
	}

	p := parser.New(tokens)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		fmt.Println("=== Parser Errors ===")
		for _, e := range p.Errors {
			fmt.Println(e)
		}
		if len(p.Errors) > 5 {
			return
		}
	}

	// ② コンパイル時型チェック（null安全・型整合性・オーバーフロー）
	checker := typecheck.New()
	checkErrors := checker.Check(prog)
	if len(checkErrors) > 0 {
		fmt.Println("=== Type Check Errors ===")
		for _, e := range checkErrors {
			fmt.Println(e)
		}
		// エラーがあればコンパイルを中断
		fmt.Println("コンパイルを中断しました。型エラーを修正してください。")
		return
	}

	// ③ cel: Import依存チェック
	if celFile != nil {
		var imports []string
		for _, stmt := range prog.Statements {
			if imp, ok := stmt.(*ast.ImportNode); ok {
				imports = append(imports, imp.Module)
			}
		}
		missing := celFile.CheckImports(imports)
		if len(missing) > 0 {
			fmt.Println("=== Cell Dependency Error ===")
			for _, m := range missing {
				fmt.Printf("  Import[%s] が project.cel の dependencies に含まれていません\n", m)
			}
			fmt.Println("project.cel に依存関係を追加してください。")
			return
		}
	}

	// ④ Echo: riskブロックのスキャン・警告（ir-onlyモードではスキップ）
	if !irOnly {
		ec := echo.New(baseName)
		ec.Scan(prog)
		ec.ScanProject()
		if !ec.WarnInline() {
			return
		}
	}

	// ④ QBE IR生成（関数を sim_main にリネーム）
	ir := codegen.New().Generate(prog)
	// export function w $main → export function w $sim_main
	ir = strings.ReplaceAll(ir, "function w $main(", "export function w $sim_main(")
	ir = strings.ReplaceAll(ir, "export function w $sim_main(", "export function w $sim_main(")
	ir = strings.ReplaceAll(ir, "call $main(", "call $sim_main(")

	ssaFile := baseName + ".ssa"
	os.WriteFile(ssaFile, []byte(ir), 0644)
	fmt.Printf("QBE IR  → %s\n", ssaFile)

	if irOnly {
		fmt.Printf("QBE IR → %s\n", ssaFile)
		return
	}

	// ④ タイマー付きCラッパー生成
	wrapperCode := `#include <stdio.h>
#include <time.h>

extern int sim_main();

int main() {
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    long result = sim_main();
    clock_gettime(CLOCK_MONOTONIC, &end);
    double ms = (end.tv_sec - start.tv_sec) * 1000.0
              + (end.tv_nsec - start.tv_nsec) / 1e6;
    printf("Similarity result: %ld  time: %.2fms\n", result, ms);
    return 0;
}
`
	wrapperFile := baseName + "_wrapper.c"
	os.WriteFile(wrapperFile, []byte(wrapperCode), 0644)

	// ⑤ qbeが使えるか確認
	_, qbeErr := exec.LookPath("qbe")
	useQBE := qbeErr == nil

	binFile := baseName + ".out"

	if useQBE {
		// QBEパイプライン: .ssa → .s → binary
		asmFile := baseName + ".s"

		fmt.Println("Backend: QBE ✅")
		qbeCmd := exec.Command("qbe", "-o", asmFile, ssaFile)
		qbeCmd.Stdout = os.Stdout
		qbeCmd.Stderr = os.Stderr
		if err := qbeCmd.Run(); err != nil {
			fmt.Println("QBEエラー:", err)
			fmt.Println("C バックエンドに切り替えます...")
			useQBE = false
		} else {
			fmt.Printf("Assembly → %s\n", asmFile)
			// アセンブリ + Cラッパー → バイナリ
			gccCmd := exec.Command("gcc", "-O2", "-o", binFile, asmFile, wrapperFile)
			gccCmd.Stdout = os.Stdout
			gccCmd.Stderr = os.Stderr
			if err := gccCmd.Run(); err != nil {
				fmt.Println("リンクエラー:", err)
				return
			}
		}
	}

	if !useQBE {
		// Cバックエンド（QBEなしの環境用フォールバック）
		fmt.Println("Backend: C (fallback)")
		c := cgen.New().Generate(prog)
		cFile := baseName + ".c"
		os.WriteFile(cFile, []byte(c), 0644)
		fmt.Printf("C code  → %s\n", cFile)

		gccCmd := exec.Command("gcc", "-O2", "-o", binFile, cFile)
		gccCmd.Stdout = os.Stdout
		gccCmd.Stderr = os.Stderr
		if err := gccCmd.Run(); err != nil {
			fmt.Println("コンパイルエラー:", err)
			return
		}
	}

	fmt.Printf("Binary  → %s ✅\n", binFile)

	// Echo: コンパイル後レポート
	ecFinal := echo.New(baseName)
	ecFinal.Scan(prog)
	ecFinal.ScanProject()
	ecFinal.Report()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: sim [--ir-only] [--cai] <file.iia|file.sml>")
		fmt.Println("  --ir-only : QBE IRファイルのみ生成")
		fmt.Println("  --cai     : CAIバックエンドを使用")
		os.Exit(1)
	}

	irOnly := false
	useCAI := false
	filename := ""

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--ir-only":
			irOnly = true
		case "--cai":
			useCAI = true
		default:
			filename = arg
		}
	}

	if filename == "" {
		fmt.Println("Error: ファイルを指定してください")
		os.Exit(1)
	}

	b, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	src := string(b)
	dir := filepath.Dir(filename)

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
		filename = iiaFile
	}

	if useCAI {
		compileCAI(src, filename, dir)
		return
	}

	compile(src, filename, dir, irOnly)
}

func compileCAI(input, baseName, dir string) {
	// .cel読み込み
	celFile, celErr := cel.Load(dir)
	if celErr != nil {
		fmt.Println("=== Cell Error ===")
		fmt.Println(celErr)
	}
	if celFile != nil {
		fmt.Print(celFile.Info())
		fmt.Println()
	}

	// lexer → parser → AST
	l := lexer.New(input)
	tokens := l.Tokenize()
	p := parser.New(tokens)
	prog := p.ParseProgram()

	if len(p.Errors) > 0 {
		fmt.Println("=== Parser Errors ===")
		for _, e := range p.Errors {
			fmt.Println(e)
		}
		return
	}

	// typecheck
	tc := typecheck.New()
	tcErrors := tc.Check(prog)
	if len(tcErrors) > 0 {
		fmt.Println("=== Type Check Errors ===")
		for _, e := range tcErrors {
			fmt.Println(e)
		}
		fmt.Println("コンパイルを中断しました。型エラーを修正してください。")
		return
	}

	// CAI IR生成
	cg := caigen.New()
	caiSrc := cg.Generate(prog)

	caiFile := baseName + ".cai"
	if err := os.WriteFile(caiFile, []byte(caiSrc), 0644); err != nil {
		fmt.Println("CAI書き込みエラー:", err)
		return
	}
	fmt.Printf("CAI IR → %s\n", caiFile)

	// Echo
	ec := echo.New(baseName)
	ec.Scan(prog)
	ec.ScanProject()
	if !ec.WarnInline() {
		return
	}

	// CAI変換器でバイナリ生成
	// sim実行ファイルと同じディレクトリのcai_converterを探す
	exePath, _ := os.Executable()
	converterPath := filepath.Join(filepath.Dir(exePath), "cai_conv")
	if _, err := os.Stat(converterPath); err != nil {
		converterPath = filepath.Join(".", "cai_conv")
	}

	binFile := baseName + ".out"
	cmd := exec.Command(converterPath, caiFile, binFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("CAI変換エラー:", err)
		return
	}

	ec.Report()
}
