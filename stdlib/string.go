// Package stdlib: string標準ライブラリ
// Import[string{}]で使えるようになる関数群
// NUL終端文字列操作。libc不要。
//
// 提供関数:
//   str_len(ptr)              → 文字列長
//   str_compare(a, b)         → 0=等しい, 1=異なる
//   str_copy(dst, src)        → srcをdstにコピー（dstのアドレスを返す）
//   str_concat(dst, a, b)     → a+bをdstに連結（dstのアドレスを返す）
//   str_contains_char(ptr, c) → 文字cが含まれれば1、なければ0
package stdlib

const StringLibCAI = `
# str_len: NUL終端文字列の長さ
# arg0 = ptr → length
func $str_len
  alloc  %ptr.ptr 8
  storep %ptr.ptr %arg0
  alloc  %len.ptr 4
  store  %len.ptr 0
  label  str_len_loop
    loadp2 %p %ptr.ptr
    loadb  %c %p
    ceq    %done %c 0
    jnz    %done str_len_end str_len_cont
  label  str_len_cont
    load   %l %len.ptr
    add    %l1 %l 1
    store  %len.ptr %l1
    loadp2 %p2 %ptr.ptr
    addp   %p3 %p2 1
    storep %ptr.ptr %p3
    jmp    str_len_loop
  label  str_len_end
  load   %ret %len.ptr
  ret    %ret
endfunc

# str_compare: 2つのNUL終端文字列を比較
# arg0 = a, arg1 = b → 0=等しい, 1=異なる
func $str_compare
  alloc  %a.ptr 8
  alloc  %b.ptr 8
  storep %a.ptr %arg0
  storep %b.ptr %arg1
  label  str_cmp_loop
    loadp2 %pa %a.ptr
    loadp2 %pb %b.ptr
    loadb  %ca %pa
    loadb  %cb %pb
    cne    %diff %ca %cb
    jnz    %diff str_cmp_ne str_cmp_eq_so_far
  label  str_cmp_eq_so_far
    ceq    %both0 %ca 0
    jnz    %both0 str_cmp_equal str_cmp_next
  label  str_cmp_next
    addp   %pa2 %pa 1
    addp   %pb2 %pb 1
    storep %a.ptr %pa2
    storep %b.ptr %pb2
    jmp    str_cmp_loop
  label  str_cmp_equal
  ret    0
  label  str_cmp_ne
  ret    1
endfunc

# str_copy: srcをdstにコピー（NUL終端含む）
# arg0 = dst, arg1 = src → dst
func $str_copy
  alloc  %dst.ptr 8
  alloc  %src.ptr 8
  storep %dst.ptr %arg0
  storep %src.ptr %arg1
  alloc  %dst0.ptr 8
  storep %dst0.ptr %arg0
  label  str_copy_loop
    loadp2 %ps %src.ptr
    loadb  %c %ps
    loadp2 %pd %dst.ptr
    storeb %pd %c
    ceq    %done %c 0
    jnz    %done str_copy_end str_copy_cont
  label  str_copy_cont
    addp   %ps2 %ps 1
    addp   %pd2 %pd 1
    storep %src.ptr %ps2
    storep %dst.ptr %pd2
    jmp    str_copy_loop
  label  str_copy_end
  loadp2 %ret %dst0.ptr
  ret    %ret
endfunc

# str_contains_char: 文字cがptr内に含まれるか
# arg0 = ptr, arg1 = c → 1=含む, 0=含まない
func $str_contains_char
  alloc  %ptr.ptr 8
  alloc  %c.ptr 4
  storep %ptr.ptr %arg0
  store  %c.ptr %arg1
  load   %target %c.ptr
  label  str_cc_loop
    loadp2 %p %ptr.ptr
    loadb  %ch %p
    ceq    %null %ch 0
    jnz    %null str_cc_notfound str_cc_check
  label  str_cc_check
    ceq    %found %ch %target
    jnz    %found str_cc_found str_cc_next
  label  str_cc_next
    loadp2 %p2 %ptr.ptr
    addp   %p3 %p2 1
    storep %ptr.ptr %p3
    jmp    str_cc_loop
  label  str_cc_found
  ret    1
  label  str_cc_notfound
  ret    0
endfunc
`

const StringLibC = `
// stdlib: string (C fallback)
static int str_len(const char *s){
    int n=0; while(s[n]) n++; return n;
}
static int str_compare(const char *a, const char *b){
    while(*a && *a==*b){ a++; b++; }
    return *a!=*b ? 1 : 0;
}
static char *str_copy(char *dst, const char *src){
    char *d=dst; while((*d++=*src++)); return dst;
}
static int str_contains_char(const char *s, int c){
    while(*s){ if(*s==(char)c) return 1; s++; } return 0;
}
`

const StringLib = ``
