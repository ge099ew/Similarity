// Package stdlib: core標準ライブラリ
// Import[core{}]で使えるようになる関数群
package stdlib

const CoreLibCAI = `
# panic: メッセージをstderrに出力してexit(1)
# arg0 = msg_ptr（NUL終端文字列）
func $panic
  alloc  %msg.ptr 8
  storep %msg.ptr %arg0
  loadp2 %msg %msg.ptr
  call   %len $io_strlen %msg
  loadp2 %msg2 %msg.ptr
  syscall %_ 1 2 %msg2 %len
  syscall %_ 60 1 0 0
  ret    0
endfunc

# assert: condが0ならpanic
# arg0 = cond, arg1 = msg_ptr
func $assert
  alloc  %cond.ptr 4
  alloc  %msg.ptr 8
  store  %cond.ptr %arg0
  storep %msg.ptr %arg1
  load   %cond %cond.ptr
  ceq    %fail %cond 0
  jnz    %fail assert_fail assert_ok
  label  assert_fail
  loadp2 %msg %msg.ptr
  call   %_ $panic %msg
  label  assert_ok
  ret    0
endfunc
`

const CoreLibC = `
// stdlib: core (C fallback)
#include <stdio.h>
#include <stdlib.h>
static void panic_sim(const char *msg){
    fprintf(stderr, "%s\n", msg);
    exit(1);
}
static void assert_sim(int cond, const char *msg){
    if(!cond) panic_sim(msg);
}
`

const CoreLib = ``
