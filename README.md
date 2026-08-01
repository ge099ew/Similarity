# 警告:書類等の更新が間に合っていません。古い可能性があるため、あまり信用し過ぎないでください。
<<<<<<< HEAD

=======
>>>>>>> 42c4cf9 (Similarity プロトタイプv0.1)
# Similarity

**"No GC. No guessing. No C/C++"**

C/C++を玉座から引きずり降ろすために設計されたシステムプログラミング言語。

作者: 奇曲 宮夢 (Kikyoku Miyu)
バージョン: v0.1.0 (Prototype)

---

## Why Similarity?

C/C++はシステムプログラミングの標準だが、問題が多い。unsafeな操作が暗黙的に許可され、コンパイラは推測で動き、ツールチェーンはGCCやLLVMに依存し続ける。

Similarityはその全てを拒否する。

- **GCなし** — メモリ管理を言語が隠さない
- **推測しない** — 全ての操作を明示する
- **unsafe操作は`Mem[risk{}]`で明示** — Echoが自動でレポートを生成する
- **Async間の共有変数は`share()`で明示** — データ競合をコンパイル時に検出
- **C/C++依存ゼロ** — CAI変換器がas・ldを完全排除。GCC不要

---

## クイックスタート

```bash
# 1. ビルド
go build -o sim ./cmd/
gcc -O2 -o cai_conv cai_converter/cai_converter.c

# 2. Hello World
./sim --cai examples/hello.iia

# 3. fibonacci
./sim --cai examples/fibonacci.iia
./examples/fibonacci.iia.out
# → Similarity result: 55  time: 0ms
```

---

## コンパイラパイプライン

```
.iia → lexer → parser → AST → typecheck → echo → caigen → CAI IR → cai_conv → バイナリ
.sml → transpiler → .iia → 上記パイプライン
```

**CAI（Common Assembly Instructions）** はSimilarity独自のIRです。
`cai_conv`はx86_64機械語を直接生成し、as（アセンブラ）もld（リンカ）も使いません。
静的PIE（ET_DYN）のELFバイナリをsyscallベースで直接出力します。GCC不要。

---

## 言語の特徴

### 基本パターン
```
カテゴリ[操作{引数}]
```

### 変数・制御フロー
```iia
Variable[let{int(x:10)}]
If[check{less(x:0)}, True[...], False[...]]
Loop[for{int(i:0), less(i:10), step{1}}, Body[...]]
```

### 関数
```iia
Function[add{
  receive{int(a), int(b)},
  return(+{int(a:b)})
}]
```

### 安全性
```iia
Mem[risk{
  Variable[let{int(val:deref{ptr})}]  # unsafeを明示
}]

Async[{
  share(x),                            # 共有変数を明示
  Mutation[variable{int(x:30)}]
}]
```

### モジュール
```iia
Import[io{}]
Import[math{}]
Import[string{}]
```

---

## 標準ライブラリ

| ライブラリ | 主要関数 |
|---|---|
| `math` | absolute_value, maximum, minimum, pow_int, clamp |
| `io` | io_write, io_read, io_open, io_close, io_print |
| `core` | panic, assert |
| `string` | str_len, str_compare, str_copy |
| `memory` | mem_copy, mem_set, mem_zero, mem_compare |
| `sys` | sys_exit, sys_getpid, sys_sleep |
| `time` | time_now_ms, time_sleep |
| `random` | random_next, random_range |
| `process` | process_exit, process_getpid |
| `os` | os_mkdir, os_remove, os_rename |

---

## 安全性システム（コンパイル時）

エラーは **`行:列: TypeCheck Error [コード]: メッセージ`** 形式で出力されます。

| コード | 内容 |
|---|---|
| TC1001 | null許容型のnullチェックなしアクセス |
| TC2001〜TC2010 | 型ミスマッチ・未宣言変数・配列型違反等 |
| TC3002 | risk{}外でのderef使用 |
| TC4001 | 整数オーバーフロー（32bit範囲超え） |
| TC5001 | share: 未宣言変数 |
| TC5002 | Async内でshare宣言なしにMutation |

---

## ベンチマーク（vs C++ -O0）

| テスト | Similarity (CAI) | C++ (-O0) |
|---|---|---|
| fibonacci(40) | ~709ms | ~985ms |
| sum(0〜1億) | ~180ms | ~268ms |
| ackermann(3,7) | ~12ms | ~5ms |

```bash
bash benchmark/run_benchmark_all.sh
```

---

## ディレクトリ構成

```
Similarity/
├── cmd/main.go              — エントリーポイント
├── compiler/                — CompilerContext（Target/ABI/Options/Diagnostics）
├── lexer/
├── parser/
├── ast/
├── typecheck/
├── caigen/                  — AST → CAI IR生成
├── cai_converter/
│   └── cai_converter.c      — CAI → x86_64機械語直接生成
├── echo/                    — riskブロックスキャン
├── cel/                     — project.cel管理
├── stdlib/
│   ├── math.go
│   ├── io.go
│   ├── core.go
│   ├── string.go
│   ├── memory.go
│   ├── sys.go
│   ├── time.go
│   ├── random.go
│   ├── process.go
│   └── os.go
├── examples/                — サンプルコード
├── benchmark/               — ベンチマーク
└── docs/                    — ドキュメント
```

---

## 実装状況

| 機能 | 状態 |
|---|---|
| lexer/parser | ✅ |
| typecheck | ✅ |
| CAI IR生成 | ✅ |
| x86_64機械語直接生成 | ✅ |
| ELF直接生成 | ✅ |
| 静的PIE（ET_DYN + ASLR） | ✅ |
| NX（スタック実行禁止） | ✅ |
| i32/i64/f32演算 | ✅ |
| syscall直接呼び出し | ✅ |
| 標準ライブラリ（10種） | ✅ |
| Echo（project.eho） | ✅ |
| Cell（project.cel） | ✅ |
| APE形式（マルチOS） | 🔶 未着手 |
| 各言語互換性レイヤー | 🔶 未着手 |
| GPU本実装 | 🔶 未着手 |
| 自己ホスト | 📅 長期目標 |

---

## 設計原則

1. **コンパイラは推測しない** — 全て明示
2. **unsafe操作はMem[risk{}]で明示**（Echoが自動レポート）
3. **Async間の共有変数はshare()で明示**
4. **速度は妥協しない** — GCなし、ゼロコスト抽象化
5. **C/C++依存ゼロ** — CAI変換器がas・ldを完全排除済み

---

## ライセンス

MIT License — Copyright (c) 2026 Kikyoku Miyu (奇曲 宮夢)
