// Package stdlib: memory標準ライブラリ
// Import[memory{}]で使えるようになる関数群
// libc不要。syscall直接実装。
//
// 提供関数:
//   mem_copy(dst, src, n)  → nバイトコピー
//   mem_set(dst, val, n)   → nバイトをvalで埋める
//   mem_compare(a, b, n)   → 0=等しい, 1=異なる
//   mem_zero(dst, n)       → nバイトをゼロクリア
package stdlib

const MemoryLibCAI = `
# mem_copy: srcからdstへnバイトコピー
# arg0=dst, arg1=src, arg2=n → dst
func $mem_copy
  alloc  %dst.ptr 8
  alloc  %src.ptr 8
  alloc  %n.ptr 4
  storep %dst.ptr %arg0
  storep %src.ptr %arg1
  store  %n.ptr %arg2
  alloc  %dst0.ptr 8
  storep %dst0.ptr %arg0
  alloc  %i.ptr 4
  store  %i.ptr 0
  label  memcpy_loop
    load   %i %i.ptr
    load   %n %n.ptr
    clt    %cont %i %n
    jnz    %cont memcpy_body memcpy_end
  label  memcpy_body
    loadp2 %ps %src.ptr
    loadb  %c %ps
    loadp2 %pd %dst.ptr
    storeb %pd %c
    addp   %ps2 %ps 1
    addp   %pd2 %pd 1
    storep %src.ptr %ps2
    storep %dst.ptr %pd2
    load   %i2 %i.ptr
    add    %i3 %i2 1
    store  %i.ptr %i3
    jmp    memcpy_loop
  label  memcpy_end
  loadp2 %ret %dst0.ptr
  ret    %ret
endfunc

# mem_set: dstのnバイトをvalで埋める
# arg0=dst, arg1=val(byte), arg2=n → dst
func $mem_set
  alloc  %dst.ptr 8
  alloc  %val.ptr 4
  alloc  %n.ptr 4
  storep %dst.ptr %arg0
  store  %val.ptr %arg1
  store  %n.ptr %arg2
  alloc  %dst0.ptr 8
  storep %dst0.ptr %arg0
  alloc  %i.ptr 4
  store  %i.ptr 0
  load   %val %val.ptr
  label  memset_loop
    load   %i %i.ptr
    load   %n %n.ptr
    clt    %cont %i %n
    jnz    %cont memset_body memset_end
  label  memset_body
    loadp2 %pd %dst.ptr
    storeb %pd %val
    addp   %pd2 %pd 1
    storep %dst.ptr %pd2
    load   %i2 %i.ptr
    add    %i3 %i2 1
    store  %i.ptr %i3
    jmp    memset_loop
  label  memset_end
  loadp2 %ret %dst0.ptr
  ret    %ret
endfunc

# mem_zero: dstのnバイトをゼロクリア
# arg0=dst, arg1=n → dst
func $mem_zero
  alloc  %dst.ptr 8
  alloc  %n.ptr 4
  storep %dst.ptr %arg0
  store  %n.ptr %arg1
  call   %ret $mem_set %arg0 0 %arg1
  ret    %ret
endfunc

# mem_compare: aとbのnバイトを比較
# arg0=a, arg1=b, arg2=n → 0=等しい, 1=異なる
func $mem_compare
  alloc  %a.ptr 8
  alloc  %b.ptr 8
  alloc  %n.ptr 4
  storep %a.ptr %arg0
  storep %b.ptr %arg1
  store  %n.ptr %arg2
  alloc  %i.ptr 4
  store  %i.ptr 0
  label  memcmp_loop
    load   %i %i.ptr
    load   %n %n.ptr
    clt    %cont %i %n
    jnz    %cont memcmp_body memcmp_equal
  label  memcmp_body
    loadp2 %pa %a.ptr
    loadp2 %pb %b.ptr
    loadb  %ca %pa
    loadb  %cb %pb
    cne    %diff %ca %cb
    jnz    %diff memcmp_ne memcmp_next
  label  memcmp_next
    addp   %pa2 %pa 1
    addp   %pb2 %pb 1
    storep %a.ptr %pa2
    storep %b.ptr %pb2
    load   %i2 %i.ptr
    add    %i3 %i2 1
    store  %i.ptr %i3
    jmp    memcmp_loop
  label  memcmp_equal
  ret    0
  label  memcmp_ne
  ret    1
endfunc
`

const MemoryLibC = `
// stdlib: memory (C fallback)
static void *mem_copy(void *dst, const void *src, int n){
    char *d=dst; const char *s=src;
    for(int i=0;i<n;i++) d[i]=s[i];
    return dst;
}
static void *mem_set(void *dst, int val, int n){
    char *d=dst; for(int i=0;i<n;i++) d[i]=(char)val; return dst;
}
static void *mem_zero(void *dst, int n){ return mem_set(dst,0,n); }
static int mem_compare(const void *a, const void *b, int n){
    const char *p=a, *q=b;
    for(int i=0;i<n;i++) if(p[i]!=q[i]) return 1;
    return 0;
}
`

const MemoryLib = ``
