// Package compiler: コンパイルパイプライン
// CAIパイプライン: AST → CAI IR → cai_conv → バイナリ
package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"similarity/ast"
	"similarity/caigen"
	"similarity/cel"
	"similarity/echo"
	"strings"
)

// ===== Cell =====

// LoadCell は project.cel を読み込む。
// エラーがあってもコンパイルは継続（警告扱い）。
func LoadCell(ctx *CompilerContext) *cel.CelFile {
	celFile, celErr := cel.Load(ctx.SourceDir)
	if celErr != nil {
		fmt.Println("=== Cell Error ===")
		fmt.Println(celErr)
		ctx.Diagnostics.AddWarning("CEL0001", celErr.Error())
	}
	if celFile != nil {
		fmt.Print(celFile.Info())
		fmt.Println()
	}
	return celFile
}

// CheckImports は Import[xxx{}] が project.cel に記載されているか検証する
func CheckImports(ctx *CompilerContext, celFile *cel.CelFile, prog *ast.Program) bool {
	if celFile == nil {
		return true
	}
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
			msg := fmt.Sprintf("Import[%s] が project.cel の dependencies に含まれていません", m)
			fmt.Printf("  %s\n", msg)
			ctx.Diagnostics.AddError("CEL0002", msg, ctx.InputFile, 0, 0)
		}
		fmt.Println("project.cel に依存関係を追加してください。")
		return false
	}
	return true
}

// ===== Echo =====

// RunEcho は riskブロックのスキャンと警告表示を行う。
// ユーザーがコンパイル継続を拒否した場合は false を返す。
func RunEcho(ctx *CompilerContext, prog *ast.Program) bool {
	ec := echo.New(ctx.BaseName)
	ec.Scan(prog)
	ec.ScanProject()
	return ec.WarnInline()
}

// ReportEcho はコンパイル後のEchoレポートを生成する
func ReportEcho(ctx *CompilerContext, prog *ast.Program) {
	ec := echo.New(ctx.BaseName)
	ec.Scan(prog)
	ec.ScanProject()
	ec.Report()
}

// ===== CAI IR生成 =====

// GenerateCAIIR は AST → CAI IR テキストを生成し、.caiファイルに書き出す。
// 生成したCAI IRのパスを返す。
func GenerateCAIIR(ctx *CompilerContext, prog *ast.Program) (string, error) {
	ctx.Logf("caigen: generating CAI IR")
	cg := caigen.New()
	caiSrc := cg.Generate(prog)

	caiFile := ctx.BaseName + ".cai"
	if err := os.WriteFile(caiFile, []byte(caiSrc), 0644); err != nil {
		return "", fmt.Errorf("CAI書き込みエラー: %w", err)
	}
	fmt.Printf("CAI IR → %s\n", caiFile)
	return caiFile, nil
}

// ===== CAI変換器（cai_conv）=====

// findConverter は cai_conv バイナリのパスを探す。
// sim実行ファイルと同じディレクトリ → カレントディレクトリの順で探す。
func findConverter() string {
	exePath, _ := os.Executable()
	candidates := []string{
		filepath.Join(filepath.Dir(exePath), "cai_conv"),
		filepath.Join(".", "cai_conv"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "./cai_conv"
}

// RunConverter は cai_conv を実行してバイナリを生成する
func RunConverter(ctx *CompilerContext, caiFile string) error {
	binFile := ctx.BaseName + ".out"
	converterPath := findConverter()

	ctx.Logf("converter: %s %s %s", converterPath, caiFile, binFile)
	cmd := exec.Command(converterPath, caiFile, binFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("CAI変換エラー: %w", err)
	}
	return nil
}

// ===== パイプライン =====

// RunCAIPipeline はCAIバックエンドのフルパイプラインを実行する。
//
//	AST → CAI IR → cai_conv → バイナリ
func RunCAIPipeline(ctx *CompilerContext, prog *ast.Program) {
	// Echo（riskブロック警告）
	if !RunEcho(ctx, prog) {
		return
	}

	// CAI IR生成
	caiFile, err := GenerateCAIIR(ctx, prog)
	if err != nil {
		fmt.Println(err)
		ctx.Diagnostics.AddError("CAI0001", err.Error(), ctx.InputFile, 0, 0)
		return
	}

	// IRのみモード
	if ctx.Options.IROnly {
		fmt.Printf("CAI IR → %s\n", caiFile)
		return
	}

	// cai_convでバイナリ生成
	if err := RunConverter(ctx, caiFile); err != nil {
		fmt.Println(err)
		ctx.Diagnostics.AddError("CAI0002", err.Error(), ctx.InputFile, 0, 0)
		return
	}

	// Echoレポート
	ReportEcho(ctx, prog)
}

// ===== トランスパイル =====

// Transpile は .sml → .iia のトランスパイルを行い、
// .iia のソースとファイルパスを返す。
func Transpile(ctx *CompilerContext, src, filename string) (string, string, error) {
	// transpilerパッケージを直接importすると循環するため
	// ここではmain側から呼ぶ設計にする。
	// このメソッドはパス計算のみ担当。
	iiaFile := strings.TrimSuffix(filename, ".sml") + ".iia"
	return src, iiaFile, nil
}
