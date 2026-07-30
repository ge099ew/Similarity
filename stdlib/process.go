// Package stdlib: process標準ライブラリ
// Import[process{}]で使えるようになる関数群
//
// 提供関数:
//   process_exit(code) → プロセス終了
//   process_getpid()   → プロセスID取得
package stdlib

const ProcessLibCAI = `
# process_exit: プロセスを終了する
# arg0 = exit code
func $process_exit
  alloc  %code.ptr 4
  store  %code.ptr %arg0
  load   %code %code.ptr
  syscall %_ 60 %code 0 0
  ret    0
endfunc

# process_getpid: 現在のプロセスIDを返す
func $process_getpid
  syscall %pid 39 0 0 0
  ret    %pid
endfunc
`

const ProcessLibC = `
// stdlib: process (C fallback)
#include <unistd.h>
static void process_exit(int code){ _exit(code); }
static int  process_getpid(void){ return (int)getpid(); }
`

const ProcessLib = ``
