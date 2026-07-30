// Package stdlib: Similarityの標準ライブラリ
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

func $minimum
  alloc  %a.ptr 4
  alloc  %b.ptr 4
  store  %a.ptr %arg0
  store  %b.ptr %arg1
  load   %a %a.ptr
  load   %b %b.ptr
  clt    %cond1 %a %b
  jnz    %cond1 min_a min_b
  label  min_a
  ret    %a
  label  min_b
  ret    %b
endfunc

func $pow_int
  alloc  %base.ptr 4
  alloc  %exp.ptr 4
  store  %base.ptr %arg0
  store  %exp.ptr %arg1
  alloc  %result.ptr 4
  store  %result.ptr 1
  alloc  %i.ptr 4
  store  %i.ptr 0
  label  pow_loop
  load   %i %i.ptr
  load   %e %exp.ptr
  clt    %cond %i %e
  jnz    %cond pow_body pow_end
  label  pow_body
  load   %r %result.ptr
  load   %b %base.ptr
  mul    %r1 %r %b
  store  %result.ptr %r1
  load   %i2 %i.ptr
  add    %i3 %i2 1
  store  %i.ptr %i3
  jmp    pow_loop
  label  pow_end
  load   %ret %result.ptr
  ret    %ret
endfunc

func $clamp
  alloc  %val.ptr 4
  alloc  %lo.ptr 4
  alloc  %hi.ptr 4
  store  %val.ptr %arg0
  store  %lo.ptr %arg1
  store  %hi.ptr %arg2
  load   %val %val.ptr
  load   %lo %lo.ptr
  load   %hi %hi.ptr
  clt    %under %val %lo
  jnz    %under clamp_lo clamp_check_hi
  label  clamp_lo
  ret    %lo
  label  clamp_check_hi
  cgt    %over %val %hi
  jnz    %over clamp_hi clamp_ok
  label  clamp_hi
  ret    %hi
  label  clamp_ok
  ret    %val
endfunc
`

const MathLibC = `
// stdlib: math (C fallback)
static int absolute_value(int x){ return x < 0 ? -x : x; }
static int maximum(int a, int b){ return a > b ? a : b; }
static int minimum(int a, int b){ return a < b ? a : b; }
static int pow_int(int base, int exp){
    int r=1; for(int i=0;i<exp;i++) r*=base; return r;
}
static int clamp(int val, int lo, int hi){
    return val<lo?lo:(val>hi?hi:val);
}
`

const MathLib = ``

// AvailableLibsCAI: 利用可能なライブラリ一覧（CAI）
var AvailableLibsCAI = map[string]string{
	"math":    MathLibCAI,
	"io":      IoLibCAI,
	"core":    CoreLibCAI,
	"string":  StringLibCAI,
	"memory":  MemoryLibCAI,
	"sys":     SysLibCAI,
	"time":    TimeLibCAI,
	"random":  RandomLibCAI,
	"process": ProcessLibCAI,
	"os":      OsLibCAI,
}

// AvailableLibsC: 利用可能なライブラリ一覧（Cフォールバック）
var AvailableLibsC = map[string]string{
	"math":    MathLibC,
	"io":      IoLibC,
	"core":    CoreLibC,
	"string":  StringLibC,
	"memory":  MemoryLibC,
	"sys":     SysLibC,
	"time":    TimeLibC,
	"random":  RandomLibC,
	"process": ProcessLibC,
	"os":      OsLibC,
}

// AvailableLibs: QBE IR用（互換性のために残す）
var AvailableLibs = map[string]string{
	"math": MathLib,
	"io":   IoLib,
}
