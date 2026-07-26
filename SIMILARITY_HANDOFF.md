# Similarity言語 引き継ぎドキュメント

> 新しいチャットでこのドキュメントを渡すことで開発を継続できます。

---

## 1. プロジェクトの目的

C/C++を玉座から引きずり降ろすために設計されたシステムプログラミング言語。

**キャッチコピー:** "No GC. No guessing. No C/C++"

**作者:** 奇曲 宮夢 (Kikyoku Miyu)

**GitHub:** https://github.com/ge099ew/Similarity

---

## 2. アーキテクチャ

```
.iia → lexer → parser → AST → typecheck → echo → codegen → QBE IR → バイナリ
.sml → transpiler → .iia → 上記パイプライン

--caiフラグ使用時:
.iia → caigen → CAI IR(.cai) → cai_conv → ELF実行ファイル（GCC不要）
```

**ディレクトリ構成:**
```
Similarity/
├── cmd/main.go              — エントリーポイント（--ir-only / --cai フラグ対応）
├── lexer/lexer.go
├── parser/parser.go         — ★ 負数リテラルバグ修正済み
├── ast/ast.go
├── codegen/codegen.go       — QBE IR生成
├── caigen/caigen.go         — CAI IR生成
├── cgen/cgen.go             — Cフォールバック
├── transpiler/transpiler.go — .sml→.iiaトランスパイラ
├── typecheck/
│   └── typecheck.go         — ★ 全エラーに行番号・列番号付与済み
├── echo/echo.go             — project.eho生成
├── cel/cel.go               — project.cel管理
├── stdlib/math.go
├── error/error.go
├── cai_converter/
│   └── cai_converter.c      — ★ .text/.rodataセクション分離済み + data命令追加
└── benchmark/
    ├── bench_fib.iia/.sml/.cpp
    ├── bench_sum.iia/.sml/.cpp
    ├── bench_frontend_short/long.iia/.cpp
    ├── run_benchmark.sh      — QBE vs C++
    └── run_benchmark_cai.sh  — CAI vs C++
```

---

## 3. 言語仕様

### ファイル形式
| 拡張子 | 説明 |
|---|---|
| `.iia` | 低レイヤー構文 |
| `.sml` | シュガーシンタックス（.iiaにトランスパイル） |
| `.cai` | CAI IR（テキスト形式） |

### シュガーシンタックス（.sml）略語
| 旧(.iia) | 新(.sml) |
|---|---|
| `Variable` | `Var` |
| `Function` | `Func` |
| `Function_public` | `Func_pub` |
| `Application` | `App` |

### 基本パターン
```
カテゴリ[操作{引数}]
```

### 変数・演算子・制御フロー
```iia
Variable[let{int(x:10)}]
Variable[let{int(x:-5)}]      ← 負数リテラル対応済み（parser.go修正）
Variable[unclet{float(PI:3.14)}]
+{int(a:b)}  -{int(a:b)}  *{int(a:b)}  /{int(a:b)}
equal(a:b)  notequal(a:b)  less(a:b)  lesseq(a:b)  greater(a:b)  greatereq(a:b)
If[check{less(hp:0)}, True[...], False[...]]
Loop[for{int(i:0), less(i:10), step{1}}, Body[...]]
break{}  /  continue{}
```

### 関数・ポインタ・配列・cast
```iia
Function[name{ receive{int(x)}, 処理, return(x) }]
Function_public[name{...}]
call{name(args)}
Variable[let{int(ptr:addr{x})}]
Mem[risk{ Variable[let{int(val:deref{ptr})}] }]
Variable[let{int(val:index{arr(i)})}]
Variable[let{float(y:cast{float(x)})}]
```

### 構造体・非同期・エラー・モジュール
```iia
Variable[struct{User:String(name), int(age)}]
Variable[let{user:User(name:"John", age:25)}]
Async[{ share(x), Mutation[variable{int(x:30)}] }]
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

**★ 全エラーが `行:列: TypeCheck Error [コード]: メッセージ` 形式で出力されるようになった**

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
- 未知のキー検出（行番号付きエラー）
- バージョン形式チェック（x.y.z）
- 依存関係の重複検出
- `key: value`形式チェック

---

## 6. CAI（Common Assembly Instructions）

### 現在の状況（完成済み）
- **caigen.go**: AST→CAI IRテキスト生成 ✅
- **cai_converter.c**: CAI→x86_64機械語直接生成 ✅
  - asの代替（GCCのアセンブラ完全排除）✅
  - ldの代替（ELF64実行ファイル直接生成）✅
  - syscall直接呼び出し（write/clock_gettime/exit）✅
  - GCC完全不要 ✅
  - fchmod syscallで実行権限付与（shell不要）✅
  - 未解決シンボルを致命エラー化 ✅
  - peephole最適化（EAX追跡）✅
  - レジスタ割り当て（callee-saved: rbx/r12-r15）✅
  - **★ .text/.rodataセクション分離済み** ✅
    - PT_LOAD #1: .text（RX）0x400000
    - PT_LOAD #2: .rodata（R）pageアライン後
    - rodataが空のときはPHDR 1つ（後方互換）
  - **★ CAI IR `data`命令を新設** ✅
    - `data $label "文字列"` → .rodataに配置しシンボル登録
    - エスケープシーケンス（\n \t \\ \"）対応済み

### 残課題
1. アラインメント一般化
2. PIE/ASLR対応
3. APE形式（Cosmopolitan、マルチOS）

### 使い方
```bash
gcc -O2 -o cai_conv cai_converter/cai_converter.c
./sim --cai your_file.iia
```

### CAI命令セット（主要）
```
func $name / export func $name / endfunc
alloc %dst size / store %ptr %val / load %dst %ptr
add/sub/mul/div %dst %a %b
clt/cle/ceq/cne/cgt/cge %dst %a %b
label name / jmp label / jnz %cond true false
call %dst $func args... / ret %val
data $label "文字列"          ← 新規追加
```

---

## 7. stdlib（標準ライブラリ）

- `absolute_value(x)` → 絶対値 ✅（負数リテラルバグ修正済み）
- `maximum(a, b)` → 最大値 ✅

---

## 8. ベンチマーク結果（100回平均・コールドスタート）

### QBEバックエンド vs C++ (-O0)
| 比較項目 | Similarity | C++ | 勝敗 |
|---|---|---|---|
| fibonacci(40) | 713ms | 453ms | C++ |
| 総和（0〜1億） | 23ms | 67ms | **Similarity 2.8倍** |
| フロントエンド（短いファイル） | 2.25ms | 8.02ms | **Similarity 3.6倍** |
| フロントエンド（長いファイル） | 2.89ms | 8.22ms | **Similarity 2.8倍** |

### CAIバックエンド vs C++ (-O0)
| 比較項目 | CAI | C++ |
|---|---|---|
| fibonacci(40) | ~745ms | ~452ms |
| 総和（0〜1億） | ~92ms | ~66ms |

※ CAIは最適化継続中。

```bash
bash benchmark/run_benchmark.sh        # QBE vs C++
bash benchmark/run_benchmark_cai.sh   # CAI vs C++
```

---

## 9. 実装状況

| 機能 | 状態 |
|---|---|
| lexer/parser | ✅（負数リテラルバグ修正済み） |
| codegen（QBE） | ✅ |
| ポインタ/配列/cast/構造体 | ✅ |
| Mem[risk{}] | ✅ |
| Async/Await/share() | ✅ |
| typecheck（行番号・列番号付きエラー） | ✅ |
| Echo（project.eho） | ✅ |
| Cell（project.cel） | ✅ |
| stdlib/math | ✅ |
| シュガーシンタックス（.sml） | ✅ |
| CAI IR（caigen.go） | ✅ |
| CAI変換器（x86_64直接生成） | ✅ |
| as代替（機械語直接生成） | ✅ |
| ld代替（ELF直接生成） | ✅ |
| syscall直接呼び出し | ✅ |
| GCC完全不要（CAIパイプライン） | ✅ |
| **セクション分離（.text/.rodata）** | ✅ |
| **CAI data命令（文字列定数→.rodata）** | ✅ |
| アラインメント一般化 | 🔶 未着手 |
| PIE/ASLR対応 | 🔶 未着手 |
| APE形式（マルチOS） | 🔶 未着手 |
| 標準ライブラリ拡張（io等） | 🔶 未着手 |
| 各言語互換性レイヤー | 🔶 未着手 |
| ASTへの位置情報（LSP対応） | ✅（typecheckに反映済み） |
| string→enum化 | 🔶 未着手 |
| GPU本実装 | 🔶 未着手 |
| 自己ホスト | 📅 長期目標 |

---

## 10. 未実装タスク一覧

1. アラインメント一般化（CAIバックエンド）
2. PIE/ASLR対応
3. APE形式（Cosmopolitan Libc、マルチOS対応）
4. 標準ライブラリ拡張（io等）
5. 各言語互換性レイヤー（Python/Rust/Java/C#/Odin/JS/Go/Zig）
6. string→enum化（TypeKind, OpKind等）
7. GPU本実装
8. Webサイト: ダウンロードURL本番差し替え
9. 自己ホスト（長期目標）

---

## 11. テストファイル

```bash
go build -o sim ./cmd/
gcc -O2 -o cai_conv cai_converter/cai_converter.c

./sim test_math.iia              # result: 5
./sim --cai test_math.iia        # result: 5（GCC不要）
echo "Y" | ./sim test_all.iia   # result: 42
./sim test_errors.iia            # 5:22: TypeCheck Error [TC4001]...
bash benchmark/run_benchmark.sh
bash benchmark/run_benchmark_cai.sh
```

---

## 12. Webサイト構成

```
Similarity_Web/
├── 0home/
├── 1signboard/
├── 2download/         — ダウンロードURL差し替え待ち
├── 3Philosophy_and_Origins/    — 完成
├── 4Results_and_Progress_Status/ — 完成
├── 5license/
└── icon/
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
