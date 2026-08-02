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

# 2. fibonacci
./sim --cai examples/fibonacci.iia
./examples/fibonacci.out
# → Similarity result: 55  time: 0ms
```

---

## コンパイラパイプライン

```
.iia → lexer → parser → AST → typecheck → echo → caigen → CAI IR → cai_conv → バイナリ
```

**CAI（Common Assembly Instructions）** はSimilarity独自のIRです。  
`cai_conv`はx86_64機械語を直接生成し、as（アセンブラ）もld（リンカ）も使いません。  
静的PIE（ET_DYN）のELFバイナリをsyscallベースで直接出力します。GCC不要。

---

## 言語仕様

### 基本パターン

```
カテゴリ[操作{引数}]
```

### 変数

```iia
Variable[let{int(x:10)}]          # ミュータブル
Variable[unclet{float(PI:3.14)}]  # イミュータブル
Mutation[variable{int(x:30)}]     # 再代入
```

### 演算子

```iia
+{int(a,b)}   # 加算
-{int(a,b)}   # 減算
*{int(a,b)}   # 乗算
/{int(a,b)}   # 除算
++{i}         # インクリメント
--{i}         # デクリメント
```

### 比較

```iia
equal(a:b)     # a == b
notequal(a:b)  # a != b
less(a:b)      # a < b
lesseq(a:b)    # a <= b
greater(a:b)   # a > b
greatereq(a:b) # a >= b
```

### 制御フロー

```iia
If[check{less(hp:0)},
  True{...},
  False{...}
]

Loop[
  check{less(i:10)},
  for{
    ...,
    ++{i}
  }
]

break{}
continue{}
```

### 関数

```iia
Function[add{
  receive{int(a), int(b)},
  return(+{int(a,b)})
}]

Function_public[main{
  receive{},
  Variable[let{int(result:call{add(1, 2)})}],
  return(result)
}]
```

### 配列

```iia
Variable[let{Array_int(arr:10)}]          # 10要素のint配列
Mutation[array{int(arr:0:42)}]            # arr[0] = 42
Variable[let{int(val:index{arr(0)})}]     # val = arr[0]
```

### ポインタ

```iia
Variable[let{int(ptr:addr{x})}]
Mem[risk{
  Variable[let{int(val:deref{ptr})}]
}]
```

### 非同期

```iia
Async[{
  share(x),
  Mutation[variable{int(x:30)}]
}]
```

### モジュール

```iia
Import[math{}]
Import[io{}]
```

---

## 標準ライブラリ

| ライブラリ | 主要関数 |
|---|---|
| `math` | absolute_value, maximum |
| `io` | io_print |

詳細は `docs/StandardLibrary.md` を参照。

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

## ベンチマーク（vs C/C++/Rust、-O0）

# 警告:古いベンチマークです。正しくありません。現在、CAI等の不備で正しいベンチマークを測れていません。
| テスト | Similarity (CAI) | C++ (-O0) | C (-O0) | Rust (-O0) |
|---|---|---|---|---|
| fibonacci(40) | 3909ms | 884ms | 902ms | 899ms |
| sum(0〜1億) | 468ms | 262ms | 266ms | 463ms |
| bubble_sort(5000²) | 59ms | 61ms | 61ms | 88ms |
| nested_loop(1Kx1K) | 2ms | 2.4ms | 2.4ms | 3.5ms |
| matrix(200³) | 29ms | 21ms | 19ms | 28ms |
| ackermann(3,7) | 9ms | 5.7ms | 5.8ms | 5.2ms |

```bash
bash benchmark/run_benchmark.sh
```

---

## ディレクトリ構成

```
Similarity/
├── cmd/main.go              — エントリーポイント
├── lexer/
├── parser/
├── ast/
├── typecheck/
├── caigen/                  — AST → CAI IR生成
├── cai_converter/
│   └── cai_converter.c      — CAI → x86_64機械語直接生成
├── echo/                    — riskブロックスキャン
├── cel/                     — project.cel管理
├── stdlib/                  — 標準ライブラリ
├── examples/                — サンプルコード
├── benchmark/               — ベンチマーク（C/C++/Rust比較付き）
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
| 配列（Array_int等） | ✅ |
| ポインタ/deref/addr | ✅ |
| 構造体 | ✅ |
| 非同期（Async/share） | ✅ |
| Echo（project.eho） | ✅ |
| Cell（project.cel） | ✅ |
| 標準ライブラリ | ✅ |
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
