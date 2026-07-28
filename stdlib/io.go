// Package stdlib: io標準ライブラリ
// Import[io{}]で使えるようになる関数群
// syscallを直接呼び出す。libc不要。
//
// 提供関数:
//   io_write(fd, ptr, len) → 書き込んだバイト数（-1=エラー）
//   io_read(fd, ptr, len)  → 読み込んだバイト数（0=EOF, -1=エラー）
//   io_open(path_ptr, flags) → fd（-1=エラー）
//   io_close(fd)           → 0=成功, -1=エラー
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
//
// 注意: io_print は将来loadb命令実装後に追加予定。
// 現在は io_write(1, buf_ptr, len) を直接使う。
package stdlib

// IoLibCAI: CAI形式のio実装（syscall直接呼び出し）
const IoLibCAI = `
# io_write: fd, buf_ptr, len → 書き込んだバイト数（-1=エラー）
# syscall: rax=1(write), rdi=fd, rsi=buf, rdx=len
func $io_write
  alloc  %fd.ptr 4
  alloc  %ptr.ptr 4
  alloc  %len.ptr 4
  store  %fd.ptr %arg0
  store  %ptr.ptr %arg1
  store  %len.ptr %arg2
  load   %fd %fd.ptr
  load   %ptr %ptr.ptr
  load   %len %len.ptr
  syscall %ret 1 %fd %ptr %len
  ret    %ret
endfunc

# io_read: fd, buf_ptr, len → 読み込んだバイト数（0=EOF, -1=エラー）
# syscall: rax=0(read), rdi=fd, rsi=buf, rdx=len
func $io_read
  alloc  %fd.ptr 4
  alloc  %ptr.ptr 4
  alloc  %len.ptr 4
  store  %fd.ptr %arg0
  store  %ptr.ptr %arg1
  store  %len.ptr %arg2
  load   %fd %fd.ptr
  load   %ptr %ptr.ptr
  load   %len %len.ptr
  syscall %ret 0 %fd %ptr %len
  ret    %ret
endfunc

# io_open: path_ptr, flags → fd（-1=エラー）
# syscall: rax=2(open), rdi=path, rsi=flags, rdx=0644(mode=420)
func $io_open
  alloc  %path.ptr 4
  alloc  %flags.ptr 4
  store  %path.ptr %arg0
  store  %flags.ptr %arg1
  load   %path %path.ptr
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
`

// IoLib: QBE IR用（未使用、互換性のために残す）
const IoLib = ``
