package main

import (
	"fmt"
	"os"
	"os/exec"
	"similarity/cgen"
	"similarity/codegen"
	"similarity/lexer"
	"similarity/parser"
	"strings"
)

func compile(input, baseName string) {
	// ① lexer → parser → AST
	l := lexer.New(input)
	tokens := l.Tokenize()
	if len(l.Errors) > 0 {
		fmt.Println("=== Lexer Errors ===")
		for _, e := range l.Errors { fmt.Println(e) }
	}

	p := parser.New(tokens)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		fmt.Println("=== Parser Errors ===")
		for _, e := range p.Errors { fmt.Println(e) }
		if len(p.Errors) > 5 { return }
	}

	// ② QBE IR生成（関数を sim_main にリネーム）
	ir := codegen.New().Generate(prog)
	// export function w $main → export function w $sim_main
	ir = strings.ReplaceAll(ir, "function w $main(", "function w $sim_main(")
	ir = strings.ReplaceAll(ir, "call $main(", "call $sim_main(")

	ssaFile := baseName + ".ssa"
	os.WriteFile(ssaFile, []byte(ir), 0644)
	fmt.Printf("QBE IR  → %s\n", ssaFile)

	// ③ タイマー付きCラッパー生成
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

	// ④ qbeが使えるか確認
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
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: sim <file.iia>")
		fmt.Println("  QBEがあれば自動的にQBEを使います")
		fmt.Println("  なければCにフォールバックします")
		os.Exit(1)
	}

	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	compile(string(b), os.Args[1])
}
