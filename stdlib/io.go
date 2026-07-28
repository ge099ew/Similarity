// Package stdlib: io標準ライブラリ
// Import[io{}]で使えるようになる関数群
// syscallを直接呼び出す。libc不要。
//
// 提供関数:
//   io_write(fd, ptr, len) → 書き込んだバイト数（-1=エラー）
//   io_read(fd, ptr, len)  → 読み込んだバイト数（0=EOF, -1=エラー）
//   io_open(path_ptr, flags) → fd（-1=エラー）
//   io_close(fd)           → 0=成功, -1=エラー
//   io_strlen(ptr)         → NUL終端文字列の長さ
//   io_print(ptr)          → NUL終端文字列をstdout(fd=1)に出力
//
// syscall番号（x86_64 Linux）:
//   read=0, write=1, open=2, close=3
//
// io_openのflagsの値:
//   0   = O_RDONLY
//   1   = O_WRONLY
//   2   = O_RDWR
//   65  = O_WRONLY|O_CREAT|O_TRUNC
//   577 = O_WRONLY|O_CREAT|O_APPEND
package stdlib

// IoLibCAI: CAI形式のio実装（syscall直接呼び出し）
const IoLibCAI = `
# io_write: fd, buf_ptr, len → 書き込んだバイト数（-1=エラー）
# syscall: rax=1(write), rdi=fd, rsi=buf, rdx=len
func $io_write
  alloc  %fd.ptr 4
  alloc  %ptr.ptr 8
  alloc  %len.ptr 4
  store  %fd.ptr %arg0
  storep %ptr.ptr %arg1
  store  %len.ptr %arg2
  load   %fd %fd.ptr
  loadp2 %ptr %ptr.ptr
  load   %len %len.ptr
  syscall %ret 1 %fd %ptr %len
  ret    %ret
endfunc

# io_read: fd, buf_ptr, len → 読み込んだバイト数（0=EOF, -1=エラー）
# syscall: rax=0(read), rdi=fd, rsi=buf, rdx=len
func $io_read
  alloc  %fd.ptr 4
  alloc  %ptr.ptr 8
  alloc  %len.ptr 4
  store  %fd.ptr %arg0
  storep %ptr.ptr %arg1
  store  %len.ptr %arg2
  load   %fd %fd.ptr
  loadp2 %ptr %ptr.ptr
  load   %len %len.ptr
  syscall %ret 0 %fd %ptr %len
  ret    %ret
endfunc

# io_open: path_ptr, flags → fd（-1=エラー）
# syscall: rax=2(open), rdi=path, rsi=flags, rdx=0644(mode=420)
func $io_open
  alloc  %path.ptr 8
  alloc  %flags.ptr 4
  storep %path.ptr %arg0
  store  %flags.ptr %arg1
  loadp2 %path %path.ptr
  load   %flags %flags.ptr
  syscall %ret 2 %path %flags 420
  ret    %ret
endfunc

# io_close: fd → 0=成功, -1=エラー
# syscall: rax=3(close), rdi=fd
func $io_close
  alloc  %fd.ptr 4
  store  %fd.ptr %arg0
  load   %fd %fd.ptr
  syscall %ret 3 %fd 0 0
  ret    %ret
endfunc

# io_strlen: NUL終端文字列の長さを返す
# arg0 = ptr（文字列先頭アドレス）
func $io_strlen
  alloc  %ptr.ptr 8
  storep %ptr.ptr %arg0
  alloc  %len.ptr 4
  store  %len.ptr 0
  label  strlen_loop
    loadp2 %p %ptr.ptr
    loadb  %c %p
    ceq    %done %c 0
    jnz    %done strlen_end strlen_cont
  label  strlen_cont
    load   %l %len.ptr
    add    %l1 %l 1
    store  %len.ptr %l1
    loadp2 %p2 %ptr.ptr
    addp   %p3 %p2 1
    storep %ptr.ptr %p3
    jmp    strlen_loop
  label  strlen_end
  load   %ret %len.ptr
  ret    %ret
endfunc

# io_print: NUL終端文字列をstdout(fd=1)に出力
# arg0 = ptr
func $io_print
  alloc  %orig.ptr 8
  storep %orig.ptr %arg0
  loadp2 %orig %orig.ptr
  call   %len $io_strlen %orig
  loadp2 %orig2 %orig.ptr
  syscall %ret 1 1 %orig2 %len
  ret    %ret
endfunc
`

// IoLibC: Cフォールバック用io実装
const IoLibC = `
// stdlib: io (C fallback)
#include <unistd.h>
#include <fcntl.h>
static int io_write(int fd, const void *buf, int len) {
    return (int)write(fd, buf, (size_t)len);
}
static int io_read(int fd, void *buf, int len) {
    return (int)read(fd, buf, (size_t)len);
}
static int io_open(const char *path, int flags) {
    return open(path, flags, 0644);
}
static int io_close(int fd) {
    return close(fd);
}
static int io_strlen(const char *s) {
    int n=0; while(s[n]) n++; return n;
}
static int io_print(const char *s) {
    return (int)write(1, s, (size_t)io_strlen(s));
}
`

// IoLib: QBE IR用（未使用、互換性のために残す）
const IoLib = ``
