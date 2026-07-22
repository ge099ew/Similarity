# Similarity言語 引き継ぎドキュメント

> 新しいチャットでこのドキュメントを渡すことで開発を継続できます。

---

## 1. プロジェクトの目的

C/C++を玉座から引きずり降ろすために設計されたシステムプログラミング言語。

**キャッチコピー:** "No GC. No guessing. No C/C++"

**核心哲学:**
- コンパイラは推測しない（全て明示）
- unsafe操作は明示必須
- 速度は妥協しない（GCなし）
- C/C++依存ゼロ（CAI完成後）

**作者:** 奇曲 宮夢 (Kikyoku Miyu)

---

## 2. アーキテクチャ

```
.iia → lexer → parser → AST → typecheck → echo → codegen → QBE IR → バイナリ
.sml → transpiler → .iia → 上記パイプライン

--caiフラグ使用時:
.iia → lexer → parser → AST → typecheck → caigen → CAI IR(.cai) → cai_conv → バイナリ
```

**ディレクトリ構成:**
```
Similarity/
├── cmd/main.go              — エントリーポイント（--ir-only / --cai フラグ対応）
├── lexer/lexer.go           — トークナイザ
├── parser/parser.go         — 構文解析
├── ast/ast.go               — AST定義
├── codegen/codegen.go       — QBE IR生成
├── caigen/caigen.go         — CAI IR生成
├── cgen/cgen.go             — Cバックエンド（QBEなし環境用フォールバック）
├── transpiler/transpiler.go — .sml→.iiaトランスパイラ
├── typecheck/               — コンパイル時安全性チェック
├── echo/echo.go             — 開発者サポートシステム（project.eho生成）
├── cel/cel.go               — パッケージ管理（.cel読み込み）
├── stdlib/math.go           — 標準ライブラリ（Go実装）
├── error/error.go           — エラーメッセージ生成
├── cai_converter/
│   └── cai_converter.c      — CAI変換器（x86_64機械語直接生成 + ELF.o出力）
└── benchmark/               — ベンチマークファイル群
    ├── bench_fib.iia/.sml/.cpp
    ├── bench_sum.iia/.sml/.cpp
    ├── bench_frontend_short/long.iia/.cpp
    ├── run_benchmark.sh      — QBEバックエンド vs C++ ベンチマーク
    └── run_benchmark_cai.sh  — CAIバックエンド vs C++ ベンチマーク
```

**GitHub:** https://github.com/ge099ew/Similarity

---

## 3. 言語仕様

### ファイル形式
| 拡張子 | 説明 |
|---|---|
| `.iia` | 低レイヤー構文（本来の形式） |
| `.sml` | シュガーシンタックス（.iiaにトランスパイル） |
| `.cai` | CAI IR（テキスト形式） |

### シュガーシンタックス（.sml）の略語対応
| 旧(.iia) | 新(.sml) |
|---|---|
| `Variable` | `Var` |
| `Function` | `Func` |
| `Function_public` | `Func_pub` |
| `Application` | `App` |
| ブラケットは`[]`のまま | |

### 基本パターン
```
カテゴリ[操作{引数}]
```

### 変数
```iia
Variable[let{int(x:10)}]
Variable[unclet{float(PI:3.14)}]
Variable[struct{User:String(name), int(age)}]
Variable[let{user:User(name:"John", age:25)}]
```

### 演算子・比較
```iia
+{int(a:b)}  -{int(a:b)}  *{int(a:b)}  /{int(a:b)}
equal(a:b)  notequal(a:b)  less(a:b)  lesseq(a:b)  greater(a:b)  greatereq(a:b)
```

### 制御フロー
```iia
If[check{less(hp:0)}, True[...], False[...]]
Loop[for{int(i:0), less(i:10), step{1}}, Body[...]]
Loop[Count{int(i:10)}, Body[...]]
break{}  /  continue{}
```

### 関数
```iia
Function[name{ receive{int(x)}, 処理, return(x) }]
Function_public[name{...}]
call{name(args)}
```

### ポインタ・配列・cast
```iia
Variable[let{int(ptr:addr{x})}]
Mem[risk{ Variable[let{int(val:deref{ptr})}] }]
Variable[let{int(val:index{arr(i)})}]
Variable[let{float(y:cast{float(x)})}]
```

### 非同期・エラー・モジュール
```iia
Async[{ share(x), Mutation[variable{int(x:30)}] }]
Await[task]
Error[try{処理}, Ok[...], Err[type{FileNotFound}, msg{"..."}]]
Fatal[type{OutOfMemory}, msg{"回復不能"}]
Import[math{}]
Extern[C{lib{"SDL2"}, draw{receive{int(x)}, return{}}}]
```

---

## 4. 安全性システム（typecheck）

| コード | 内容 |
|---|---|
| TC1001 | null許容型のnullチェックなしアクセス |
| TC2001〜TC2010 | 型ミスマッチ・未宣言変数・配列型違反等 |
| TC3002 | risk{}外でのderef使用 |
| TC4001 | 整数オーバーフロー（32bit範囲超え） |
| TC5001 | share: 未宣言変数 |
| TC5002 | Async内でshare宣言なしにMutation |

---

## 5. サポートシステム

### Echo（project.eho）
- プロジェクト単位で1つの`project.eho`を生成（コンパイルのたびに上書き）
- riskブロックを[1][2][3]と通し番号で全件列挙
- riskが0件でも生成
- **`--ir-only`モードではEcho/Cellはスキップ**

### Cell（project.cel）
```
name: MyProject
version: 0.1.0
dependencies:
  - math
```

---

## 6. CAI（Common Assembly Instructions）

### 概要
QBEを完全に置き換えるSimilarity独自のIR。

### パイプライン
```
.iia → Go(caigen) → CAI(.cai テキスト) → cai_conv → .o(ELF) → gcc(リンクのみ) → バイナリ
```

### 現在の状況
- **caigen.go**: AST→CAI IRテキスト生成 ✅
- **cai_converter.c**: CAI→x86_64機械語直接生成 ✅
  - x86_64命令エンコーディングを自前実装
  - ELF64リロケータブルオブジェクト(.o)を直接出力
  - GCCのアセンブラ（as）を完全排除
  - GCCはリンク（ld）のみ使用
  - peephole最適化（EAX追跡による冗長load/store削減）
  - レジスタ割り当て（callee-saved: rbx/r12-r15）
  - leaf関数検出・引数カウント最適化

### 次のステップ
1. ldの代替（ELF実行ファイル直接生成、.plt/.got実装）
2. APE形式（Cosmopolitan Libc、Windows/Mac/Linux同一バイナリ）

### 使い方
```bash
# CAI変換器ビルド
gcc -O2 -o cai_conv cai_converter/cai_converter.c

# CAIで実行
./sim --cai your_file.iia
./sim --cai your_file.sml
```

### CAI命令セット（主要）
```
func $name / export func $name / endfunc
alloc %dst size
store %ptr %val
load  %dst %ptr
add/sub/mul/div %dst %a %b
clt/cle/ceq/cne/cgt/cge %dst %a %b
label name / jmp label / jnz %cond true false
call %dst $func args... / ret %val
itof/ftoi %dst %src
```

---

## 7. stdlib（標準ライブラリ）

**stdlib/math.go** にGoで実装。QBE IR・C・CAI形式全てに対応。

- `absolute_value(x)` → 絶対値 ✅
- `maximum(a, b)` → 最大値 ✅

---

## 8. ベンチマーク結果（100回平均・コールドスタート）

### QBEバックエンド vs C++ (-O0)

| 比較項目 | Similarity | C++ | 勝敗 |
|---|---|---|---|
| fibonacci(40) 実行速度 | 713ms | 453ms | C++ |
| 総和（0〜1億）実行速度 | 23ms | 67ms | **Similarity 2.8倍** |
| フロントエンド（短いファイル） | 2.25ms | 8.02ms | **Similarity 3.6倍** |
| フロントエンド（長いファイル） | 2.89ms | 8.22ms | **Similarity 2.8倍** |

### CAIバックエンド vs C++ (-O0)（最新）

| 比較項目 | CAI | C++ |
|---|---|---|
| fibonacci(40) | 745ms | 452ms |
| 総和（0〜1億） | 114ms | 66ms |

※ CAIは最適化継続中。fibonacciの負けはCAI変換器の改善で解消予定。

**ベンチマーク実行:**
```bash
bash benchmark/run_benchmark.sh        # QBE vs C++
bash benchmark/run_benchmark_cai.sh   # CAI vs C++
```

---

## 9. 実装状況

| 機能 | 状態 |
|---|---|
| lexer/parser | ✅ |
| codegen（QBE） | ✅ |
| ポインタ（addr/deref） | ✅ |
| 配列アクセス（index） | ✅ |
| cast（int↔float） | ✅ |
| 構造体（struct） | ✅ |
| Mem[risk{}] | ✅ |
| Async/Await（pthread） | ✅ |
| share()（データ競合検出） | ✅ |
| typecheck | ✅ |
| Echo（project.eho） | ✅ |
| Cell（project.cel） | ✅ |
| return()構文 | ✅ |
| stdlib/math | ✅ |
| シュガーシンタックス（.sml） | ✅ |
| CAI IR（caigen.go） | ✅ |
| CAI変換器（x86_64直接生成） | ✅ |
| asの代替（機械語直接生成） | ✅ |
| Webサイト（ベンチマーク・進捗ページ） | ✅ |
| ldの代替（ELF実行ファイル直接生成） | 🔶 実装中 |
| 標準ライブラリ拡張（io等） | 🔶 未着手 |
| Python互換性 | 🔶 未着手 |
| Rust互換性 | 🔶 未着手 |
| Java互換性 | 🔶 未着手 |
| C#互換性 | 🔶 未着手 |
| Odin互換性 | 🔶 未着手 |
| JavaScript/TypeScript互換性 | 🔶 未着手 |
| Go互換性 | 🔶 未着手 |
| Zig互換性 | 🔶 未着手 |
| GPU本実装 | 🔶 CAI安定後 |
| APE形式（マルチOS） | 🔶 未着手 |
| cgen.goのstruct対応 | 🔶 未着手 |
| 自己ホスト | 📅 長期目標 |

---

## 10. 未実装タスク一覧

1. **ldの代替**（ELF実行ファイル直接生成、.plt/.got実装）
2. **APE形式**（Cosmopolitan Libc、マルチOS対応）
3. 標準ライブラリ拡張（io等）
4. 各言語互換性レイヤー（Python/Rust/Java/C#/Odin/JS/Go/Zig）
5. GPU本実装
6. cgen.goのstruct対応
7. Webサイト: ダウンロードURLの本番差し替え
8. 自己ホスト（長期目標）

---

## 11. テストファイル

```
test_all.iia    — 正常系テスト → result: 42
test_errors.iia — エラー検出テスト（TC4001オーバーフロー）
test_math.iia   — mathライブラリテスト → result: 5
```

**実行コマンド:**
```bash
go build -o sim ./cmd/
gcc -O2 -o cai_conv cai_converter/cai_converter.c

./sim test_math.iia              # QBEバックエンド
./sim --cai test_math.iia        # CAIバックエンド
echo "Y" | ./sim test_all.iia
./sim test_errors.iia            # TC4001でコンパイル中断
./sim --ir-only benchmark/bench_frontend_short.iia
bash benchmark/run_benchmark.sh
bash benchmark/run_benchmark_cai.sh
```

---

## 12. Webサイト構成

```
Similarity_Web/
├── 0home/                          — ホームページ
├── 1signboard/                     — ヘッダー・ナビゲーション
├── 2download/                      — ダウンロード（URL差し替え待ち）
├── 3Philosophy_and_Origins/        — 設計思想・誕生経緯（完成）
├── 4Results_and_Progress_Status/   — ベンチマーク・進捗（完成）
├── 5license/                       — ライセンス
└── icon/                           — アイコン
```

---

## 13. 開発者との接し方

- お世辞不要、正直なフィードバックを求める
- 数値の誇張は必ず訂正すること
- 日本語で会話
- コードはAIが書くが、設計・発想は全て開発者本人のもの
- 画像（スクリーンショット）でエラーを共有することが多い
- 修正・実装したファイルは必ず渡すこと（サマリーや差分だけでは不可）
- ビルドとテストを必ず確認してからファイルを渡すこと
