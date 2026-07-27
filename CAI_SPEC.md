# CAI（Common Assembly Instructions / 共通アセンブリ命令）仕様書

> Similarityのオリジナルバックエンド。GCC・as・ld完全不要。C/C++依存ゼロ。

---

## なぜCAIが必要か

既存のコンパイラバックエンドはいずれも外部ツールに依存している。

| バックエンド | 依存するもの |
|---|---|
| GCCバックエンド | GCC全体 |
| LLVMバックエンド | LLVM全体 |
| QBE | QBEバイナリ + GCCリンカ |
| **CAI** | **なし。C1本のみ（踏み台、一回限り）** |

CAIはSimilarityが自分自身をビルドするための足場であり、最終的にはCAI変換器自体もSimilarityで書き直す（自己ホスト）。

---

## フェーズ

| フェーズ | 形式 | 状態 |
|---|---|---|
| Phase 1 | テキスト形式（現行） | ✅ 実装済み |
| Phase 2 | バイナリ形式（高速化） | 未着手 |

---

## ファイル拡張子

- `.cai` — CAI テキスト形式（Phase 1）

---

## 基本構造

```
# コメント
func $name
  <命令>
  ...
endfunc
```

- エントリーポイント: `export func $main`（変換器内で`sim_main`に変換）
- 公開関数: `export func $name`

---

## 型

| 型 | サイズ | 説明 |
|---|---|---|
| `i32` | 4B | 32bit整数。演算・比較の基本型 |
| `i64` | 8B | 64bit整数。ポインタ・アドレス用 |
| `f32` | 4B | 32bit浮動小数点（将来実装） |

**なぜi32中心か:** x86_64のABIでは32bit演算が最も効率的。64bitへの符号拡張（movsx）が必要な場面のみi64を使う。

---

## 命令セット（Phase 1 テキスト形式）

### メモリ

```
alloc  <dst> <size>        # スタック上にsize bytesを確保
store  <ptr> <val>         # [ptr] = val（i32）
load   <dst> <ptr>         # dst = [ptr]（i32）
```

**なぜスタック中心か:** ヒープ管理はGCを必要とする。SimilarityはGCなしの設計のため、
基本的にスタックアロケーションを使い、ヒープが必要な場合はsyscall（mmap）を直接呼ぶ。

### 演算

```
add  <dst> <a> <b>         # dst = a + b（i32）
sub  <dst> <a> <b>         # dst = a - b（i32）
mul  <dst> <a> <b>         # dst = a * b（i32）
div  <dst> <a> <b>         # dst = a / b（i32、符号付き）
```

**なぜ4命令のみか:** Phase 1の目標は「動くこと」であり、網羅性より正確性を優先した。
浮動小数点・64bit演算はPhase 2で追加する。

### 比較

```
clt  <dst> <a> <b>         # dst = (a <  b) ? 1 : 0
cle  <dst> <a> <b>         # dst = (a <= b) ? 1 : 0
ceq  <dst> <a> <b>         # dst = (a == b) ? 1 : 0
cne  <dst> <a> <b>         # dst = (a != b) ? 1 : 0
cgt  <dst> <a> <b>         # dst = (a >  b) ? 1 : 0
cge  <dst> <a> <b>         # dst = (a >= b) ? 1 : 0
```

結果は必ず0または1（i32）。`jnz`と組み合わせて使う。

### 制御フロー

```
label <name>               # ラベル定義
jmp   <label>              # 無条件ジャンプ（rel32）
jnz   <cond> <t> <f>      # condが非ゼロなら<t>、ゼロなら<f>へ
```

**なぜjnzのみか:** `if/else`・`loop`・`break`はすべて`jnz`+`jmp`の組み合わせで表現できる。
命令セットを最小化することで変換器の実装が単純になる。

### 関数

```
call  <dst> <$func> [args] # 関数呼び出し。戻り値をdstに格納
ret   <val>                # 値を返して関数を終了
ret                        # void return
```

引数渡しはx86_64 System V ABI準拠（rdi, rsi, rdx, rcx, r8, r9、最大6個）。

### データ（文字列定数）

```
data  $label "文字列"      # 文字列を.rodataに配置しシンボルとして登録
```

エスケープシーケンス対応: `\n` `\t` `\\` `\"`

**なぜ.rodataか:** 文字列定数は変更不要のため、実行不可・書き込み不可の読み取り専用セグメントに置く。
これにより誤った上書きをOS側で防止できる。

### 型変換

```
itof  <dst> <src>          # i32 → f32（将来実装）
ftoi  <dst> <src>          # f32 → i32（将来実装）
```

### 外部シンボル

```
extern <name>              # 外部シンボル宣言（.oファイル生成時に使用）
```

---

## レジスタ命名規則

- `%<name>` — 仮想レジスタ（例: `%x`, `%t1`, `%result`）
- `$<name>` — 関数・データラベル（例: `$fibonacci`, `$msg`）
- `%arg0` 〜 `%arg5` — 関数引数（予約済み）

---

## CAI変換器（cai_converter.c）

### 設計方針

**なぜCで書いたか:** 自己ホストの踏み台として、最も広く使えるCを選んだ。
最終的にはSimilarity自身でCAI変換器を書き直す。Cは一回限りの踏み台にすぎない。

### 実装済み機能

| 機能 | 詳細 |
|---|---|
| 機械語直接生成 | x86_64バイト列を直接emit（asを完全排除） |
| ELF直接生成 | ET_DYN（静的PIE）を直接構築（ldを完全排除） |
| 静的PIE | ET_DYN + ロードベース0x0。ASLR完全対応 |
| NX（スタック実行禁止） | PT_GNU_STACK（RWのみ、Xなし） |
| セクション分離 | .text（RX）/ .rodata（R）/ .dynamic（RW） |
| セクションヘッダ | readelf・objdump・gdbでデバッグ可能 |
| PT_DYNAMIC | ET_DYNとして正規化、デバッガ対応 |
| PT_PHDR | プログラムヘッダの正規配置 |
| syscall直接呼び出し | write / clock_gettime / exit |
| レジスタ割り当て | callee-saved（rbx, r12-r15）、use_count順 |
| peephole最適化 | EAX追跡による冗長ロード除去 |
| 未解決シンボル | 致命エラー（exit(1)）で即停止 |

### 生成ELFのセグメント構成

```
PT_PHDR      [R]    プログラムヘッダテーブル自体
PT_LOAD      [R+X]  ELFヘッダ + PHDRs + .text
PT_LOAD      [R]    .rodata（文字列定数等、ある場合のみ）
PT_LOAD      [R+W]  .dynamic
PT_DYNAMIC   [R+W]  .dynamicセクション（ET_DYN正規化）
PT_GNU_STACK [R+W]  スタック実行禁止（NX有効）
```

### ファイルレイアウト

```
0x0000  ELFヘッダ（64B）
0x0040  Program Headers（PHDRs）
0x1000  .text（機械語、RX）
        （pageアライン後）
0xN000  .rodata（文字列定数、R）※あれば
        （pageアライン後）
0xM000  .dynamic（DT_DEBUG + DT_NULL、RW）
        .shstrtab
        セクションヘッダ
```

---

## サンプル

```cai
# fibonacci
func $fibonacci
  alloc  %n.ptr 4
  store  %n.ptr %arg0
  load   %n %n.ptr
  cle    %cond %n 1
  jnz    %cond base recurse
label base
  ret    %n
label recurse
  sub    %n1 %n 1
  call   %a $fibonacci %n1
  sub    %n2 %n 2
  call   %b $fibonacci %n2
  add    %result %a %b
  ret    %result
endfunc

export func $main
  call   %r $fibonacci 10
  ret    %r
endfunc
```

---

## 対応アーキテクチャ

| アーキテクチャ | 状態 |
|---|---|
| x86_64 (Linux) | ✅ 実装済み |
| arm64 | 未着手 |
| APE形式（マルチOS） | 未着手 |

---

## 今後の予定

| 項目 | 優先度 |
|---|---|
| io stdlib（syscall直接: open/read/write/close） | 高 |
| i64演算・f32演算の完全実装 | 高 |
| arm64対応 | 中 |
| APE形式（Cosmopolitan、マルチOS） | 中 |
| Phase 2: バイナリ形式 | 低 |
| 自己ホスト（SimilarityでCAI変換器を書き直す） | 長期 |
