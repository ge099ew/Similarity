// Package stdlib: io ライブラリ
// Import[io{}] で使えるようになる入出力関数群
package stdlib

// IoLib: io ライブラリの QBE IR 定義
const IoLib = `
# stdlib: io

# print_int(x) → 整数を標準出力（改行なし）
function $print_int(w %x) {
@start
    %buf =l alloc16 24
    %neg =w csltw %x, 0
    jnz %neg, @pi_neg, @pi_pos
@pi_neg
    %minus =w copy 45
    storeb %minus, %buf
    %x =w sub 0, %x
    %buf1 =l add %buf, 1
    %end1 =l call $itoa_buf(w %x, l %buf1)
    %len1 =w call $strlen_sim(l %buf1)
    %total =w add %len1, 1
    %total_l =l extsw %total
    %_ =w call $write_buf(l %buf, l %total_l)
    ret
@pi_pos
    %end2 =l call $itoa_buf(w %x, l %buf)
    %len2 =w call $strlen_sim(l %buf)
    %len2_l =l extsw %len2
    %_ =w call $write_buf(l %buf, l %len2_l)
    ret
}

# itoa_buf: 非負整数をバッファに書き込み、末端ポインタを返す
function l $itoa_buf(w %n, l %buf) {
@start
    %zero =w copy 0
    %eq0 =w ceqw %n, 0
    jnz %eq0, @ib_zero, @ib_main
@ib_zero
    %c0 =w copy 48
    storeb %c0, %buf
    %buf1 =l add %buf, 1
    %nul =w copy 0
    storeb %nul, %buf1
    ret %buf1
@ib_main
    # 末尾から桁を詰める（最大10桁）
    %tmp =l alloc4 12
    %i =l alloc4 4
    %iz =w copy 0
    storew %iz, %i
    %nn =l alloc4 4
    storew %n, %nn
@ib_loop
    %nv =w loadw %nn
    %iv =w loadw %i
    %done =w ceqw %nv, 0
    jnz %done, @ib_rev, @ib_body
@ib_body
    %d =w rems %nv, 10
    %dc =w add %d, 48
    %ip =l extsw %iv
    %tp =l add %tmp, %ip
    storeb %dc, %tp
    %nv2 =w divs %nv, 10
    storew %nv2, %nn
    %iv2 =w add %iv, 1
    storew %iv2, %i
    jmp @ib_loop
@ib_rev
    %iv3 =w loadw %i
    %j =l alloc4 4
    %jz =w copy 0
    storew %jz, %j
@ib_revloop
    %jv =w loadw %j
    %done2 =w ceqw %jv, %iv3
    jnz %done2, @ib_end, @ib_revbody
@ib_revbody
    %iv3m1 =w sub %iv3, 1
    %src_idx =w sub %iv3m1, %jv
    %si =l extsw %src_idx
    %sp =l add %tmp, %si
    %ch =w loadb %sp
    %jl =l extsw %jv
    %dp =l add %buf, %jl
    storeb %ch, %dp
    %jv2 =w add %jv, 1
    storew %jv2, %j
    jmp @ib_revloop
@ib_end
    %iv3l =l extsw %iv3
    %endp =l add %buf, %iv3l
    %nul2 =w copy 0
    storeb %nul2, %endp
    ret %endp
}

# strlen_sim: NUL終端文字列の長さを返す
function w $strlen_sim(l %s) {
@start
    %i =l alloc4 4
    %iz =w copy 0
    storew %iz, %i
@sl_loop
    %iv =w loadw %i
    %il =l extsw %iv
    %p =l add %s, %il
    %c =w loadb %p
    %done =w ceqw %c, 0
    jnz %done, @sl_end, @sl_body
@sl_body
    %iv2 =w add %iv, 1
    storew %iv2, %i
    jmp @sl_loop
@sl_end
    %res =w loadw %i
    ret %res
}

# write_buf: bufをlenバイト書き出す（syscall write fd=1）
function w $write_buf(l %buf, l %len) {
@start
    %fd =l copy 1
    %r =l call $write(l %fd, l %buf, l %len)
    %ri =w copy 0
    ret %ri
}

# newline() → 改行を標準出力
function $newline() {
@start
    %buf =l alloc4 4
    %lf =w copy 10
    storeb %lf, %buf
    %one =l copy 1
    %fd =l copy 1
    %_ =l call $write(l %fd, l %buf, l %one)
    ret
}

# print_string(s) → 文字列を標準出力（改行なし）
function $print_string(l %s) {
@start
    %len =w call $strlen_sim(l %s)
    %lenl =l extsw %len
    %_ =w call $write_buf(l %s, l %lenl)
    ret
}
`

// IoLibC: Cフォールバック用io実装
const IoLibC = `
// stdlib: io (C fallback)
#include <stdio.h>
static void print_int(int x)    { printf("%d", x); }
static void print_float(double x){ printf("%f", x); }
static void print_string(const char* s){ printf("%s", s); }
static int  read_int(void)      { int v; scanf("%d", &v); return v; }
static void newline(void)       { putchar('\n'); }
`

// IoLibCAI: CAI形式のio実装（syscall直接版）
const IoLibCAI = `
func $print_int
  alloc  %buf 24
  alloc  %x.ptr 4
  store  %x.ptr %arg0
  load   %x %x.ptr
  call   %_ $itoa_buf %x %buf
  call   %len $strlen_sim %buf
  call   %_ $write_buf %buf %len
  ret    0
endfunc

func $newline
  alloc  %buf 4
  store  %buf 10
  call   %_ $write_buf %buf 1
  ret    0
endfunc

func $print_string
  alloc  %s.ptr 8
  store  %s.ptr %arg0
  load   %s %s.ptr
  call   %len $strlen_sim %s
  call   %_ $write_buf %s %len
  ret    0
endfunc
`
