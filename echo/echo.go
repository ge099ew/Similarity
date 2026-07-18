// Package echo: Similarityの開発者サポートシステム
// riskブロックの検出・警告・レポートファイル生成を担当
package echo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"similarity/ast"
	"similarity/lexer"
	"similarity/parser"
	"strings"
	"time"
)

type RiskReport struct {
	File      string
	LineStart int
	LineEnd   int
	Ops       []string
}

type Echo struct {
	filename    string
	projectDir  string
	reports     []*RiskReport
	safeFiles   []string
	allIiaFiles []string
}

func New(filename string) *Echo {
	return &Echo{
		filename:   filename,
		projectDir: filepath.Dir(filename),
	}
}

// Scan: 単一ファイルのASTをスキャン
func (e *Echo) Scan(prog *ast.Program) {
	e.scanAST(e.filename, prog)
}

// ScanProject: プロジェクト全体の.iiaファイルをスキャン
func (e *Echo) ScanProject() error {
	files, err := filepath.Glob(filepath.Join(e.projectDir, "*.iia"))
	if err != nil {
		return err
	}
	e.allIiaFiles = files

	for _, f := range files {
		// コンパイル対象ファイルはScan()で処理済みなのでスキップ
		if filepath.Base(f) == filepath.Base(e.filename) {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		l := lexer.New(string(b))
		tokens := l.Tokenize()
		prog := parser.New(tokens).ParseProgram()
		e.scanAST(f, prog)
	}

	// safeファイルを収集
	riskFiles := map[string]bool{}
	for _, r := range e.reports {
		riskFiles[r.File] = true
	}
	for _, f := range e.allIiaFiles {
		if !riskFiles[f] {
			e.safeFiles = append(e.safeFiles, filepath.Base(f))
		}
	}
	return nil
}

func (e *Echo) scanAST(filename string, prog *ast.Program) {
	for _, stmt := range prog.Statements {
		e.scanNode(filename, stmt)
	}
}

func (e *Echo) scanNode(filename string, node ast.Node) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.FuncNode:
		for _, s := range n.Body {
			e.scanNode(filename, s)
		}
	case *ast.RawMemNode:
		e.reports = append(e.reports, &RiskReport{
			File:      filename,
			LineStart: n.LineStart,
			LineEnd:   n.LineEnd,
			Ops:       n.Ops,
		})
		for _, s := range n.Body {
			e.scanNode(filename, s)
		}
	case *ast.IfNode:
		for _, s := range n.True {
			e.scanNode(filename, s)
		}
		for _, s := range n.False {
			e.scanNode(filename, s)
		}
	case *ast.LoopNode:
		for _, s := range n.Body {
			e.scanNode(filename, s)
		}
	}
}

func (e *Echo) HasRisk() bool {
	return len(e.reports) > 0
}

// HasCurrentFileRisk: コンパイル対象ファイル自身にriskブロックがあるか
func (e *Echo) HasCurrentFileRisk() bool {
	for _, r := range e.reports {
		if filepath.Base(r.File) == filepath.Base(e.filename) {
			return true
		}
	}
	return false
}

// WarnInline: .eho生成→CLI表示→Y/n確認
func (e *Echo) WarnInline() bool {
	if !e.HasRisk() {
		e.Report()
		return true
	}

	// コンパイル対象ファイル自身にriskがない場合はY/n不要
	if !e.HasCurrentFileRisk() {
		e.Report()
		return true
	}

	// 先に.ehoを生成
	e.Report()

	ehoFile := ehoFilename(e.projectDir)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║        ⚠️   RISK BLOCK DETECTED  ⚠️        ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	// riskブロック一覧（コンパイル対象ファイルのみ表示）
	idx := 1
	for _, r := range e.reports {
		if filepath.Base(r.File) == filepath.Base(e.filename) {
			fmt.Printf("  [%d] %s : line %d-%d\n", idx, filepath.Base(r.File), r.LineStart, r.LineEnd)
			fmt.Printf("      → %s use\n", strings.Join(r.Ops, ", "))
			fmt.Println()
			idx++
		}
	}

	// 他ファイルのriskがあれば概要だけ表示
	otherRisk := []string{}
	for _, r := range e.reports {
		if filepath.Base(r.File) != filepath.Base(e.filename) {
			otherRisk = append(otherRisk, filepath.Base(r.File))
		}
	}
	if len(otherRisk) > 0 {
		seen := map[string]bool{}
		uniq := []string{}
		for _, f := range otherRisk {
			if !seen[f] {
				seen[f] = true
				uniq = append(uniq, f)
			}
		}
		fmt.Printf("  ⚠️  他ファイルにもriskブロックあり: %s\n\n", strings.Join(uniq, ", "))
	}

	if len(e.safeFiles) > 0 {
		fmt.Printf("  safe: %s （riskブロックなし）\n", strings.Join(e.safeFiles, ", "))
		fmt.Println()
	}

	fmt.Printf("詳細は %s を確認してください。\n", ehoFile)
	fmt.Print("コンパイルを続行しますか？ [Y/n]: ")

	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "n" || input == "no" {
		fmt.Println("コンパイルを中断しました。")
		return false
	}
	fmt.Println()
	return true
}

// Report: project.ehoにプロジェクト全体のレポートを書き出す（riskなしでも生成）
func (e *Echo) Report() {
	ehoFile := ehoFilename(e.projectDir)

	var sb strings.Builder
	sb.WriteString("Similarity Echo Report\n")
	sb.WriteString(fmt.Sprintf("Generated  : %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Risk Blocks: %d\n", len(e.reports)))
	if len(e.safeFiles) > 0 {
		sb.WriteString(fmt.Sprintf("Safe Files : %s\n", strings.Join(e.safeFiles, ", ")))
	}
	sb.WriteString(strings.Repeat("-", 40) + "\n")

	if len(e.reports) > 0 {
		sb.WriteString("Use risk block\n")
		for i, r := range e.reports {
			sb.WriteString(fmt.Sprintf("[%d] %s : line %d-%d\n", i+1, filepath.Base(r.File), r.LineStart, r.LineEnd))
			sb.WriteString(fmt.Sprintf("    → %s use\n", strings.Join(r.Ops, ", ")))
			sb.WriteString("    メモリ安全性は保証されません。\n")
			if i < len(e.reports)-1 {
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString(strings.Repeat("-", 40) + "\n")
	sb.WriteString("エディタでファイルを開き、上記の行番号に移動して内容を確認してください。\n")

	if len(e.safeFiles) > 0 {
		sb.WriteString("\n[Safe Files]\n")
		for _, f := range e.safeFiles {
			sb.WriteString(fmt.Sprintf("  ✅ %s\n", f))
		}
	}

	os.WriteFile(ehoFile, []byte(sb.String()), 0644)
	fmt.Printf("Echo Report → %s\n", ehoFile)
}

// ehoFilename: プロジェクトディレクトリにproject.ehoを置く
func ehoFilename(projectDir string) string {
	return filepath.Join(projectDir, "project.eho")
}

// ReadLines: .iiaファイルの指定行を読む（エディタ連携用）
func ReadLines(filename string, start, end int) []string {
	f, err := os.Open(filename)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	line := 1
	for scanner.Scan() {
		if line >= start && line <= end {
			lines = append(lines, fmt.Sprintf("%d: %s", line, scanner.Text()))
		}
		line++
	}
	return lines
}
