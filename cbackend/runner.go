// Package cbackend: Cバックエンドの起動・管理。
package cbackend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"similarity/ast"
)

// Run はAnnotated ASTをBIR形式にシリアライズしてCバックエンドを実行する。
// birFile: 生成するBIRファイルのパス
// outFile: 出力ELFバイナリのパス
func Run(prog *ast.Program, birFile, outFile string) error {
	// Step 1: Annotated AST → BIR テキスト生成
	bir := Serialize(prog)

	// Step 2: BIRファイルに書き出す
	if err := os.WriteFile(birFile, []byte(bir), 0644); err != nil {
		return fmt.Errorf("BIRファイル書き出し失敗: %w", err)
	}

	// Step 3: Cバックエンドバイナリを探して実行
	backendPath := findBackend()
	cmd := exec.Command(backendPath, birFile, outFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Cバックエンド実行失敗: %w", err)
	}

	return nil
}

// WriteBIR はBIRファイルのみを書き出す（バックエンド実行なし）。
// --ir-only 相当のデバッグ用。
func WriteBIR(prog *ast.Program, birFile string) error {
	bir := Serialize(prog)
	return os.WriteFile(birFile, []byte(bir), 0644)
}

// findBackend は sim_backend バイナリのパスを探す。
func findBackend() string {
	exePath, err := os.Executable()
	if err == nil {
		candidates := []string{
			filepath.Join(filepath.Dir(exePath), "sim_backend"),
			filepath.Join(".", "sim_backend"),
			"./cbackend/sim_backend",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	return "./sim_backend"
}

// BIRPath は入力ファイルパスに対応するBIRファイルパスを返す。
func BIRPath(inputFile string) string {
	base := strings.TrimSuffix(inputFile, filepath.Ext(inputFile))
	return base + ".bir"
}

// OutPath は入力ファイルパスに対応する出力バイナリパスを返す。
func OutPath(inputFile string) string {
	base := strings.TrimSuffix(inputFile, filepath.Ext(inputFile))
	return base + ".out"
}
