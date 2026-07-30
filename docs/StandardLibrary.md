# Similarity 標準ライブラリ

---

## 使い方

```iia
Import[math{}]
Import[io{}]
```

---

## math

```iia
call{absolute_value(x)}      # 絶対値
call{maximum(a, b)}          # 最大値
call{minimum(a, b)}          # 最小値
call{pow_int(base, exp)}     # 整数累乗
call{clamp(val, lo, hi)}     # 範囲クランプ
```

---

## io

```iia
call{io_write(fd, ptr, len)}   # ファイル書き込み
call{io_read(fd, ptr, len)}    # ファイル読み込み
call{io_open(path, flags)}     # ファイルオープン（fd返す）
call{io_close(fd)}             # ファイルクローズ
call{io_strlen(ptr)}           # NUL終端文字列の長さ
call{io_print(ptr)}            # stdout出力

# io_open flags:
#   0 = O_RDONLY
#   1 = O_WRONLY
#   65 = O_WRONLY|O_CREAT|O_TRUNC
```

---

## core

```iia
call{panic(msg_ptr)}             # エラー出力してexit(1)
call{assert(cond, msg_ptr)}      # 条件が偽ならpanic
```

---

## string

```iia
call{str_len(ptr)}                  # 文字列長
call{str_compare(a, b)}             # 比較（0=等しい）
call{str_copy(dst, src)}            # コピー
call{str_contains_char(ptr, c)}     # 文字が含まれるか（1/0）
```

---

## memory

```iia
call{mem_copy(dst, src, n)}    # nバイトコピー
call{mem_set(dst, val, n)}     # nバイトをvalで埋める
call{mem_zero(dst, n)}         # nバイトゼロクリア
call{mem_compare(a, b, n)}     # nバイト比較（0=等しい）
```

---

## sys

```iia
call{sys_exit(code)}      # プロセス終了
call{sys_getpid()}        # プロセスID取得
call{sys_sleep(sec)}      # 秒単位スリープ
call{sys_timestamp()}     # 現在時刻（nanosec下位32bit）
```

---

## time

```iia
call{time_now_ms()}      # 現在時刻（ミリ秒）
call{time_sleep(sec)}    # 秒単位スリープ
```

---

## random

```iia
# seed変数（alloc済み）を渡して使う
call{random_next(seed_ptr)}           # 次の乱数
call{random_range(seed_ptr, lo, hi)}  # 範囲指定乱数
```

---

## process

```iia
call{process_exit(code)}   # プロセス終了
call{process_getpid()}     # プロセスID
```

---

## os

```iia
call{os_mkdir(path, mode)}     # ディレクトリ作成（mode例: 493=0755）
call{os_remove(path)}          # ファイル削除
call{os_rename(old, new)}      # ファイル名変更
```
