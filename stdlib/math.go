// Package stdlib: Similarityの標準ライブラリ
// Import[math{}]で使えるようになる関数群
package stdlib

// MathLibCAI: CAI形式のmath実装
const MathLibCAI = `
func $absolute_value
  alloc  %x.ptr 4
  store  %x.ptr %arg0
  load   %x %x.ptr
  clt    %cond1 %x 0
  jnz    %cond1 abs_neg abs_pos
  label  abs_neg
  sub    %r1 0 %x
  ret    %r1
  label  abs_pos
  ret    %x
endfunc
func $maximum
  alloc  %a.ptr 4
  alloc  %b.ptr 4
  store  %a.ptr %arg0
  store  %b.ptr %arg1
  load   %a %a.ptr
  load   %b %b.ptr
  cgt    %cond1 %a %b
  jnz    %cond1 max_a max_b
  label  max_a
  ret    %a
  label  max_b
  ret    %b
endfunc
`

// MathLibC: Cフォールバック用math実装
const MathLibC = `
// stdlib: math (C fallback)
static int absolute_value(int x) { return x < 0 ? -x : x; }
static int maximum(int a, int b) { return a > b ? a : b; }
`

// MathLib: QBE IR用（未使用、互換性のために残す）
const MathLib = ``

// AvailableLibsCAI: 利用可能なライブラリ一覧（CAI）
var AvailableLibsCAI = map[string]string{
	"math": MathLibCAI,
	"io":   IoLibCAI,
}

// AvailableLibsC: 利用可能なライブラリ一覧（Cフォールバック）
var AvailableLibsC = map[string]string{
	"math": MathLibC,
	"io":   IoLibC,
}

// AvailableLibs: 利用可能なライブラリ一覧（QBE IR、互換性のために残す）
var AvailableLibs = map[string]string{
	"math": MathLib,
	"io":   IoLib,
}
