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

- エントリーポイント: `export func $main`
- 公開関数: `export func $name`

---

## 型

| 型 | サイズ | 説明 |
|---|---|---|
| `i32` | 4B | 32bit整数。演算・比較の基本型 |
| `i64` | 8B | 64bit整数。ポインタ・アドレス用 |
| `f32` | 4B | 32bit浮動小数点（SSE2） |

---

## 命令セット

### メモリ（32bit）

```
alloc  <dst> <size>        # スタック上にsize bytesを確保
store  <ptr> <val>         # [ptr] = val（i32, 4バイト）
load   <dst> <ptr>         # dst = [ptr]（i32, 4バイト）
```

### メモリ（64bit / ポインタ）

```
storep <ptr> <val>         # [ptr] = val（i64, 8バイト）
loadp2 <dst> <ptr>         # dst = [ptr]（i64, 8バイト）
addp   <dst> <ptr> <off>   # dst = ptr + off（64bitポインタ + 32bitオフセット）
```

### メモリ（バイト単位）

```
loadb  <dst> <ptr>         # dst = *(uint8_t*)ptr（1バイトゼロ拡張ロード）
storeb <ptr> <val>         # *(uint8_t*)ptr = val の下位8bit
```

### i32演算

```
add  <dst> <a> <b>         # dst = a + b（i32）
sub  <dst> <a> <b>         # dst = a - b（i32）
mul  <dst> <a> <b>         # dst = a * b（i32）
div  <dst> <a> <b>         # dst = a / b（i32、符号付き）
```

### i64演算

```
add64 <dst> <a> <b>        # dst = a + b（i64）
sub64 <dst> <a> <b>        # dst = a - b（i64）
mul64 <dst> <a> <b>        # dst = a * b（i64）
div64 <dst> <a> <b>        # dst = a / b（i64、符号付き）
mov64 <dst> <src>          # dst = src（i64コピー）
```

### i32比較

```
clt  <dst> <a> <b>         # dst = (a <  b) ? 1 : 0（i32）
cle  <dst> <a> <b>         # dst = (a <= b) ? 1 : 0
ceq  <dst> <a> <b>         # dst = (a == b) ? 1 : 0
cne  <dst> <a> <b>         # dst = (a != b) ? 1 : 0
cgt  <dst> <a> <b>         # dst = (a >  b) ? 1 : 0
cge  <dst> <a> <b>         # dst = (a >= b) ? 1 : 0
```

### i64比較

```
clt64 <dst> <a> <b>        # dst = (a <  b) ? 1 : 0（i64）
cle64 <dst> <a> <b>        # dst = (a <= b) ? 1 : 0
ceq64 <dst> <a> <b>        # dst = (a == b) ? 1 : 0
cne64 <dst> <a> <b>        # dst = (a != b) ? 1 : 0
cgt64 <dst> <a> <b>        # dst = (a >  b) ? 1 : 0
cge64 <dst> <a> <b>        # dst = (a >= b) ? 1 : 0
```

### f32演算（SSE2）

```
addf  <dst> <a> <b>        # dst = a + b（f32, addss）
subf  <dst> <a> <b>        # dst = a - b（f32, subss）
mulf  <dst> <a> <b>        # dst = a * b（f32, mulss）
divf  <dst> <a> <b>        # dst = a / b（f32, divss）
itof2 <dst> <src>          # dst = (f32)src（i32→f32, cvtsi2ss）
ftoi2 <dst> <src>          # dst = (i32)src（f32→i32, cvttss2si 切り捨て）
```

### 制御フロー

```
label <name>               # ラベル定義
jmp   <label>              # 無条件ジャンプ（rel32）
jnz   <cond> <t> <f>      # condが非ゼロなら<t>、ゼロなら<f>へ
```

### 関数

```
call  <dst> <$func> [args] # 関数呼び出し。戻り値をdstに格納
ret   <val>                # 値を返して関数を終了
ret                        # void return
```

### syscall（x86_64 Linux）

```
syscall <dst> <nr> <arg0> <arg1> <arg2>
```

- `dst` = 戻り値を格納する仮想レジスタ
- `nr`  = syscall番号（即値 or 変数）
- `arg0`〜`arg2` = 引数（最大3個、即値 / 変数 / `$シンボル`）
- x86_64 ABI: rax=nr, rdi=arg0, rsi=arg1, rdx=arg2

**主要syscall番号（x86_64 Linux）:**

| 番号 | 名前 | 用途 |
|---|---|---|
| 0 | read | ファイル読み込み |
| 1 | write | ファイル書き込み |
| 2 | open | ファイルオープン |
| 3 | close | ファイルクローズ |
| 60 | exit | プロセス終了 |
| 228 | clock_gettime | 時刻取得 |

### データ（文字列定数）

```
data  $label "文字列"      # 文字列を.rodataに配置しシンボルとして登録
```

エスケープシーケンス: `\n` `\t` `\\` `\"`

### 型変換（将来実装）

```
itof  <dst> <src>          # i32 → f32（itof2を使用推奨）
ftoi  <dst> <src>          # f32 → i32（ftoi2を使用推奨）
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
| syscall直接呼び出し | write / read / open / close / clock_gettime / exit |
| i32演算 | add/sub/mul/div + 6比較 |
| i64演算 | add64/sub64/mul64/div64 + 6比較 + mov64 |
| f32演算 | addf/subf/mulf/divf + itof2/ftoi2（SSE2） |
| バイト操作 | loadb/storeb |
| ポインタ操作 | storep/loadp2/addp |
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
  label  base
  ret    %n
  label  recurse
  sub    %n1 %n 1
  call   %a $fibonacci %n1
  sub    %n2 %n 2
  call   %b $fibonacci %n2
  add    %result %a %b
  ret    %result
endfunc

# io_print（NUL終端文字列をstdoutに出力）
data $hello "Hello, Similarity!\n"

export func $main
  syscall %ret 1 1 $hello 19
  ret    0
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
| i64演算の拡張 | 高 |
| arm64対応 | 中 |
| APE形式（Cosmopolitan、マルチOS） | 中 |
| Phase 2: バイナリ形式 | 低 |
| 自己ホスト（SimilarityでCAI変換器を書き直す） | 長期 |
