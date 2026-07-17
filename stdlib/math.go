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

// AvailableLibs: 利用可能なライブラリ一覧（QBE IR）
var AvailableLibs = map[string]string{
	"math": MathLib,
}

// AvailableLibsC: 利用可能なライブラリ一覧（Cフォールバック）
var AvailableLibsC = map[string]string{
	"math": MathLibC,
}
