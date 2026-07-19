// Package stdlib: Similarityの標準ライブラリ
// Import[math{}]で使えるようになる関数群
package stdlib

// MathLib: mathライブラリのQBE IR定義
// Import[math{}]が検出されたときcodegenに挿入される
const MathLib = `
# stdlib: math
function w $absolute_value(w %x) {
@start
    %cond1 =w csltw %x, 0
    jnz %cond1, @neg, @pos
@neg
    %r1 =w sub 0, %x
    ret %r1
@pos
    ret %x
}

function w $maximum(w %a, w %b) {
@start
    %cond1 =w csgtw %a, %b
    jnz %cond1, @retA, @retB
@retA
    ret %a
@retB
    ret %b
}
`

// MathLibC: Cフォールバック用math実装
const MathLibC = `
// stdlib: math (C fallback)
static int absolute_value(int x) { return x < 0 ? -x : x; }
static int maximum(int a, int b) { return a > b ? a : b; }
`

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

// AvailableLibs: 利用可能なライブラリ一覧（QBE IR）
var AvailableLibs = map[string]string{
	"math": MathLib,
}

// AvailableLibsC: 利用可能なライブラリ一覧（Cフォールバック）
var AvailableLibsC = map[string]string{
	"math": MathLibC,
}

// AvailableLibsCAI: 利用可能なライブラリ一覧（CAI）
var AvailableLibsCAI = map[string]string{
	"math": MathLibCAI,
}
