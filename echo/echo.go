// Package echo: Similarityの開発者サポートシステム
// riskブロックの検出・警告・レポートファイル生成を担当
package echo

import (
	"fmt"
	"os"
	"path/filepath"
	"similarity/ast"
	"strings"
	"time"
)

// RiskReport: 一つのriskブロックの情報
type RiskReport struct {
	File      string
	LineStart int
	LineEnd   int
	Ops       []string
}

// Echo: 開発者サポートシステム本体
type Echo struct {
	filename string
	reports  []*RiskReport
}

func New(filename string) *Echo {
	return &Echo{filename: filename}
}

// Scan: ASTを走査してriskブロックを収集
func (e *Echo) Scan(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		e.scanNode(stmt)
	}
}

func (e *Echo) scanNode(node ast.Node) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.FuncNode:
		for _, s := range n.Body {
			e.scanNode(s)
		}
	case *ast.RawMemNode:
		e.reports = append(e.reports, &RiskReport{
			File:      e.filename,
			LineStart: n.LineStart,
			LineEnd:   n.LineEnd,
			Ops:       n.Ops,
		})
		for _, s := range n.Body {
			e.scanNode(s)
		}
	case *ast.IfNode:
		for _, s := range n.True {
			e.scanNode(s)
		}
		for _, s := range n.False {
			e.scanNode(s)
		}
	case *ast.LoopNode:
		for _, s := range n.Body {
			e.scanNode(s)
		}
	}
}

// HasRisk: riskブロックが存在するか
func (e *Echo) HasRisk() bool {
	return len(e.reports) > 0
}

// WarnInline: コンパイル中のシンプル警告
func (e *Echo) WarnInline() {
	if !e.HasRisk() {
		return
	}
	ehoFile := ehoFilename(e.filename)
	fmt.Printf("\n⚠️  risk block detected → %s を確認してください。\n\n", ehoFile)
}

// Report: .ehoファイルに詳細レポートを書き出す
func (e *Echo) Report() {
	if !e.HasRisk() {
		return
	}

	ehoFile := ehoFilename(e.filename)

	var sb strings.Builder
	sb.WriteString("Similarity Echo Report\n")
	sb.WriteString(fmt.Sprintf("Generated : %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Source    : %s\n", e.filename))
	sb.WriteString(fmt.Sprintf("Risk Blocks: %d\n", len(e.reports)))
	sb.WriteString(strings.Repeat("-", 40) + "\n\n")

	for i, r := range e.reports {
		sb.WriteString(fmt.Sprintf("[%d] %s : line %d-%d\n", i+1, r.File, r.LineStart, r.LineEnd))
		sb.WriteString(fmt.Sprintf("    → %s use\n", strings.Join(r.Ops, ", ")))
		sb.WriteString("    メモリ安全性は保証されません。\n\n")
	}

	sb.WriteString(strings.Repeat("-", 40) + "\n")
	sb.WriteString("エディタでファイルを開き、上記の行番号に移動して内容を確認してください。\n")

	os.WriteFile(ehoFile, []byte(sb.String()), 0644)
	fmt.Printf("Echo Report → %s\n", ehoFile)
}

// ehoFilename: .iiaファイル名から.ehoファイル名を生成
func ehoFilename(filename string) string {
	ext := filepath.Ext(filename)
	return filename[:len(filename)-len(ext)] + ".eho"
}
