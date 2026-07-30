// Package stdlib: sys標準ライブラリ
// Import[sys{}]で使えるようになる関数群
// プロセス制御・システム情報。libc不要。
//
// 提供関数:
//   sys_exit(code)     → プロセス終了
//   sys_getpid()       → プロセスID
//   sys_sleep(seconds) → スリープ（nanosleep syscall）
//   sys_timestamp()    → CLOCK_MONOTONICのナノ秒（下位32bit）
package stdlib

const SysLibCAI = `
# sys_exit: プロセスを終了する
# arg0 = exit code
func $sys_exit
  alloc  %code.ptr 4
  store  %code.ptr %arg0
  load   %code %code.ptr
  syscall %_ 60 %code 0 0
  ret    0
endfunc

# sys_getpid: 現在のプロセスIDを返す
# syscall: rax=39(getpid)
func $sys_getpid
  syscall %pid 39 0 0 0
  ret    %pid
endfunc

# sys_sleep: 秒単位でスリープ
# arg0 = seconds
# syscall: nanosleep(35) — req={sec,nsec}をスタックに構築
func $sys_sleep
  alloc  %sec.ptr 4
  store  %sec.ptr %arg0
  load   %sec %sec.ptr
  alloc  %ts_sec.ptr 8
  alloc  %ts_nsec.ptr 8
  storep %ts_sec.ptr %sec
  storep %ts_nsec.ptr 0
  syscall %_ 35 %ts_sec.ptr 0 0
  ret    0
endfunc

# sys_timestamp: CLOCK_MONOTONIC のtv_nsec（下位32bit）を返す
# syscall: clock_gettime(228), clockid=1(CLOCK_MONOTONIC)
func $sys_timestamp
  alloc  %ts_sec.ptr 8
  alloc  %ts_nsec.ptr 8
  syscall %_ 228 1 %ts_sec.ptr 0
  loadp2 %nsec %ts_nsec.ptr
  ret    %nsec
endfunc
`

const SysLibC = `
// stdlib: sys (C fallback)
#include <unistd.h>
#include <time.h>
static void sys_exit(int code){ _exit(code); }
static int  sys_getpid(void){ return (int)getpid(); }
static void sys_sleep(int sec){ sleep((unsigned)sec); }
static long sys_timestamp(void){
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (long)ts.tv_nsec;
}
`

const SysLib = ``
