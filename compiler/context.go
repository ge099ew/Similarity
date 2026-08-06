// Package compiler: Similarityコンパイラの中核構造体
package compiler

import (
	"fmt"
	"strings"
)

// ===== Target =====

// Target はコンパイル対象アーキテクチャを表す
type Target int

const (
	TargetX86_64 Target = iota // Linux x86_64（現在の実装対象）
	TargetARM64                // 将来実装
)

func (t Target) String() string {
	switch t {
	case TargetX86_64:
		return "x86_64-linux"
	case TargetARM64:
		return "aarch64-linux"
	default:
		return "unknown"
	}
}

// ===== ABI =====

// ABI は呼び出し規約を表す
type ABI int

const (
	ABISysV ABI = iota // System V AMD64 ABI（Linux標準）
	ABIWIN64           // Windows x64（将来実装）
)

func (a ABI) String() string {
	switch a {
	case ABISysV:
		return "sysv"
	case ABIWIN64:
		return "win64"
	default:
		return "unknown"
	}
}

// TypeSize は型のバイトサイズを返す（ABI依存）
func (a ABI) TypeSize(typeName string) int {
	switch typeName {
	case "int", "bool":
		return 4
	case "float":
		return 4
	case "int64", "ptr":
		return 8
	default:
		return 4 // デフォルト: 4バイト
	}
}

// ===== Options =====

// Options はコンパイルオプションをまとめる
type Options struct {
	IROnly    bool   // --ir-only: CAI IRファイルのみ生成
	UseCAI    bool   // --cai: CAIバックエンドを使用
	Verbose   bool   // --verbose: 詳細ログ出力
	InputFile string // 入力ファイルパス
	OutputDir string // 出力先ディレクトリ

	// デバッグオプション（各コンパイルステージの出力を確認する）
	DumpTokens   bool // --dump-tokens:    Lexer出力を表示
	DumpAST      bool // --dump-ast:       Parser出力（AST）を表示
	DumpTypes    bool // --dump-types:     TypeChecker通過後の型情報を表示
	DumpAnalyzer bool // --dump-analyzer:  Analyzer通過後のAnnotation付きASTを表示
	DumpCFG      bool // --dump-cfg:       BackendのCFGを表示（Backend実装後に有効化）
	DumpRegAlloc bool // --dump-regalloc:  レジスタ割り当て結果を表示（Backend実装後に有効化）
	DumpMachine  bool // --dump-machine:   最終機械語/逆アセンブルを表示（Backend実装後に有効化）
}

// ===== Severity =====

// Severity は診断メッセージの重要度
type Severity int

const (
	SeverityInfo    Severity = iota
	SeverityWarning
	SeverityError
	SeverityFatal
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// ===== Diagnostic =====

// Diagnostic は1件の診断メッセージ
type Diagnostic struct {
	Severity Severity
	Code     string // "TC1001" 等
	Message  string
	File     string
	Line     int
	Col      int
	Hint     string // 修正のヒント（省略可）
}

func (d Diagnostic) String() string {
	loc := ""
	if d.File != "" {
		loc = fmt.Sprintf("%s:", d.File)
	}
	if d.Line > 0 {
		loc += fmt.Sprintf("%d:", d.Line)
	}
	if d.Col > 0 {
		loc += fmt.Sprintf("%d:", d.Col)
	}
	if loc != "" {
		loc += " "
	}

	s := fmt.Sprintf("%s%s [%s]: %s", loc, d.Severity, d.Code, d.Message)
	if d.Hint != "" {
		s += fmt.Sprintf("\n  hint: %s", d.Hint)
	}
	return s
}

// ===== Diagnostics =====

// Diagnostics は診断メッセージのコレクション
type Diagnostics struct {
	items []Diagnostic
}

func (d *Diagnostics) Add(diag Diagnostic) {
	d.items = append(d.items, diag)
}

func (d *Diagnostics) AddError(code, msg, file string, line, col int) {
	d.Add(Diagnostic{
		Severity: SeverityError,
		Code:     code,
		Message:  msg,
		File:     file,
		Line:     line,
		Col:      col,
	})
}

func (d *Diagnostics) AddWarning(code, msg string) {
	d.Add(Diagnostic{
		Severity: SeverityWarning,
		Code:     code,
		Message:  msg,
	})
}

func (d *Diagnostics) HasErrors() bool {
	for _, item := range d.items {
		if item.Severity >= SeverityError {
			return true
		}
	}
	return false
}

func (d *Diagnostics) Errors() []Diagnostic {
	var result []Diagnostic
	for _, item := range d.items {
		if item.Severity >= SeverityError {
			result = append(result, item)
		}
	}
	return result
}

func (d *Diagnostics) Print() {
	for _, item := range d.items {
		fmt.Println(item)
	}
}

// ===== CompilerContext =====

// CompilerContext はコンパイル全体の状態を保持する中核構造体。
// 全パスがこれを通じて情報を共有する。
type CompilerContext struct {
	Target      Target
	ABI         ABI
	Options     Options
	Diagnostics Diagnostics

	// 入力ファイル情報
	InputFile string
	BaseName  string // 拡張子なしのファイルパス
	SourceDir string

	// ソースコード（パース前）
	Source string
}

// New は CompilerContext を生成する
func New(opts Options) *CompilerContext {
	return &CompilerContext{
		Target:    TargetX86_64,
		ABI:       ABISysV,
		Options:   opts,
		InputFile: opts.InputFile,
		BaseName:  baseName(opts.InputFile),
		SourceDir: opts.OutputDir,
	}
}

// TypeSize は現在のABIに基づく型サイズを返す
func (ctx *CompilerContext) TypeSize(typeName string) int {
	return ctx.ABI.TypeSize(typeName)
}

// Logf はVerboseモードのときのみ出力する
func (ctx *CompilerContext) Logf(format string, args ...interface{}) {
	if ctx.Options.Verbose {
		fmt.Printf("[verbose] "+format+"\n", args...)
	}
}

// baseName はファイルパスから拡張子を除いたパスを返す
func baseName(path string) string {
	// 拡張子を除く
	for _, ext := range []string{".iia", ".sml", ".cai"} {
		if strings.HasSuffix(path, ext) {
			return path[:len(path)-len(ext)]
		}
	}
	return path
}
