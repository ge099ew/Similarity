// Package stdlib: os標準ライブラリ
// Import[os{}]で使えるようになる関数群
// ファイルシステム操作。libc不要。
//
// 提供関数:
//   os_mkdir(path, mode) → ディレクトリ作成（0=成功, -1=失敗）
//   os_remove(path)      → ファイル削除（0=成功, -1=失敗）
//   os_rename(old, new)  → ファイル名変更（0=成功, -1=失敗）
//
// syscall番号（x86_64 Linux）:
//   mkdir=83, unlink=87, rename=82
package stdlib

const OsLibCAI = `
# os_mkdir: ディレクトリを作成する
# arg0 = path_ptr, arg1 = mode（例: 0755=493）
func $os_mkdir
  alloc  %path.ptr 8
  alloc  %mode.ptr 4
  storep %path.ptr %arg0
  store  %mode.ptr %arg1
  loadp2 %path %path.ptr
  load   %mode %mode.ptr
  syscall %ret 83 %path %mode 0
  ret    %ret
endfunc

# os_remove: ファイルを削除する（unlink）
# arg0 = path_ptr
func $os_remove
  alloc  %path.ptr 8
  storep %path.ptr %arg0
  loadp2 %path %path.ptr
  syscall %ret 87 %path 0 0
  ret    %ret
endfunc

# os_rename: ファイル名を変更する
# arg0 = old_path, arg1 = new_path
func $os_rename
  alloc  %old.ptr 8
  alloc  %new.ptr 8
  storep %old.ptr %arg0
  storep %new.ptr %arg1
  loadp2 %old %old.ptr
  loadp2 %new %new.ptr
  syscall %ret 82 %old %new 0
  ret    %ret
endfunc
`

const OsLibC = `
// stdlib: os (C fallback)
#include <sys/stat.h>
#include <stdio.h>
static int os_mkdir(const char *path, int mode){
    return mkdir(path, (mode_t)mode);
}
static int os_remove(const char *path){ return remove(path); }
static int os_rename(const char *oldp, const char *newp){
    return rename(oldp, newp);
}
`

const OsLib = ``
