// Package stdlib: time標準ライブラリ
// Import[time{}]で使えるようになる関数群
//
// 提供関数:
//   time_now_ms()   → CLOCK_MONOTONICのミリ秒（下位32bit）
//   time_sleep(sec) → 秒単位スリープ
package stdlib

const TimeLibCAI = `
# time_now_ms: 現在時刻をミリ秒で返す（CLOCK_MONOTONIC、下位32bit）
# syscall: clock_gettime(228), clockid=1
func $time_now_ms
  alloc  %tv_sec.ptr 8
  alloc  %tv_nsec.ptr 8
  syscall %_ 228 1 %tv_sec.ptr 0
  loadp2 %sec %tv_sec.ptr
  loadp2 %nsec %tv_nsec.ptr
  mul64  %sec_ms %sec 1000
  div64  %nsec_ms %nsec 1000000
  add64  %ms %sec_ms %nsec_ms
  ret    %ms
endfunc

# time_sleep: 秒単位スリープ（nanosleep）
# arg0 = seconds
func $time_sleep
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
`

const TimeLibC = `
// stdlib: time (C fallback)
#include <time.h>
#include <unistd.h>
static long time_now_ms(void){
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec*1000L + ts.tv_nsec/1000000L;
}
static void time_sleep(int sec){ sleep((unsigned)sec); }
`

const TimeLib = ``
