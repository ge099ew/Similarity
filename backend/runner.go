// Package backend: Cバックエンドの起動・管理。
package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"similarity/ast"
)

// RunOptions はCバックエンド実行時のオプション。
type RunOptions struct {
	DumpBackend bool // --dump: BackendFunctionのダンプ
	DumpCFG     bool // --dump-cfg: CFGのダンプ（C Backend側で実装）
}

// Run はAnnotated ASTをBIR形式にシリアライズしてCバックエンドを実行する。
// birFile: 生成するBIRファイルのパス
// outFile: 出力ELFバイナリのパス
// opts:    Cバックエンドへ渡すオプション
func Run(prog *ast.Program, birFile, outFile string, opts RunOptions) error {
	// Step 1: Annotated AST → BIR テキスト生成
	bir := Serialize(prog)

	// Step 2: BIRファイルに書き出す
	if err := os.WriteFile(birFile, []byte(bir), 0644); err != nil {
		return fmt.Errorf("BIRファイル書き出し失敗: %w", err)
	}

	// Step 3: Cバックエンドバイナリを探してフラグ付きで実行
	backendPath := findBackend()
	args := []string{birFile, outFile}
	if opts.DumpBackend {
		args = append(args, "--dump")
	}
	if opts.DumpCFG {
		args = append(args, "--dump-cfg")
	}
	cmd := exec.Command(backendPath, args...)
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
// 実際のバイナリは cbackend/sim_backend に存在する。
func findBackend() string {
	// 固定候補: cbackend/sim_backend を最優先で確認する
	candidates := []string{
		"./cbackend/sim_backend",
		"cbackend/sim_backend",
	}
	// os.Executable() が取れる場合は sim と同じディレクトリの cbackend/ も確認
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "cbackend", "sim_backend"),
			filepath.Join(exeDir, "sim_backend"),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// 最終フォールバック
	return "./cbackend/sim_backend"
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
