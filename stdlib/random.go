// Package stdlib: random標準ライブラリ
// Import[random{}]で使えるようになる関数群
// 線形合同法（LCG）による擬似乱数。
//
// 提供関数:
//   random_seed(s)      → シード設定
//   random_next()       → 次の乱数（0〜2147483647）
//   random_range(lo,hi) → lo〜hi の乱数
package stdlib

const RandomLibCAI = `
# LCG状態（グローバル変数としてスタックに確保）
# seed: 1つのfuncスコープ内で使うため、呼び出し側でseed変数を管理する
# random_seed: シードを設定してseedポインタを返す
# arg0 = seed_ptr（alloc済みの4バイト領域）, arg1 = seed値
func $random_seed
  alloc  %ptr.ptr 8
  storep %ptr.ptr %arg0
  alloc  %val.ptr 4
  store  %val.ptr %arg1
  loadp2 %ptr %ptr.ptr
  load   %val %val.ptr
  storeb %ptr %val
  store  %ptr %val
  ret    0
endfunc

# random_next: 次の乱数を返す（LCG: a=1664525, c=1013904223）
# arg0 = seed_ptr（状態を保持する4バイト変数のアドレス）
func $random_next
  alloc  %ptr.ptr 8
  storep %ptr.ptr %arg0
  loadp2 %ptr %ptr.ptr
  load   %s %ptr
  mul    %s1 %s 1664525
  add    %s2 %s1 1013904223
  store  %ptr %s2
  clt    %neg %s2 0
  jnz    %neg rng_neg rng_pos
  label  rng_neg
  sub    %r 0 %s2
  ret    %r
  label  rng_pos
  ret    %s2
endfunc

# random_range: lo〜hi の乱数
# arg0=seed_ptr, arg1=lo, arg2=hi
func $random_range
  alloc  %ptr.ptr 8
  alloc  %lo.ptr 4
  alloc  %hi.ptr 4
  storep %ptr.ptr %arg0
  store  %lo.ptr %arg1
  store  %hi.ptr %arg2
  loadp2 %ptr %ptr.ptr
  call   %r $random_next %ptr
  load   %lo %lo.ptr
  load   %hi %hi.ptr
  sub    %range %hi %lo
  add    %range1 %range 1
  div    %mod %r %range1
  mul    %back %mod %range1
  sub    %rem %r %back
  add    %ret %rem %lo
  ret    %ret
endfunc
`

const RandomLibC = `
// stdlib: random (C fallback)
static int _rng_seed = 42;
static void random_seed(int s){ _rng_seed = s; }
static int random_next(void){
    _rng_seed = _rng_seed * 1664525 + 1013904223;
    return _rng_seed < 0 ? -_rng_seed : _rng_seed;
}
static int random_range(int lo, int hi){
    return lo + random_next() % (hi - lo + 1);
}
`

const RandomLib = ``
