# CAI（Common Assembly Instructions / 共通アセンブリ命令）仕様書

> Similarityのオリジナルバックエンド。QBEを完全に置き換え、C/C++依存ゼロを実現する。

---

## フェーズ

| フェーズ | 形式 | 状態 |
|---|---|---|
| Phase 1 | テキスト形式（デバッグ用） | 実装中 |
| Phase 2 | バイナリ形式 | 未着手 |

---

## ファイル拡張子

- `.cai` — CAI テキスト形式

---

## 基本構造

```
# コメント
func <name>
  <命令>
  ...
endfunc
```

エントリーポイントは `func sim_main` とする。
公開関数は `export func <name>` とする。

---

## 型

| 型 | 説明 |
|---|---|
| `i32` | 32bit整数 |
| `i64` | 64bit整数（ポインタ） |
| `f32` | 32bit浮動小数点 |

---

## 命令セット（Phase 1 テキスト形式）

### メモリ
```
alloc  <dst> <size>        # スタック上にsize bytesを確保し、アドレスをdstに格納
store  <ptr> <val>         # ptrのアドレスにvalを書き込む（i32）
storep <ptr> <val>         # ptrのアドレスにvalを書き込む（i64/ポインタ）
storef <ptr> <val>         # ptrのアドレスにvalを書き込む（f32）
load   <dst> <ptr>         # ptrのアドレスからi32を読み込みdstに格納
loadp  <dst> <ptr>         # ptrのアドレスからi64を読み込みdstに格納
loadf  <dst> <ptr>         # ptrのアドレスからf32を読み込みdstに格納
```

### 演算
```
add  <dst> <a> <b>         # dst = a + b (i32)
sub  <dst> <a> <b>         # dst = a - b (i32)
mul  <dst> <a> <b>         # dst = a * b (i32)
div  <dst> <a> <b>         # dst = a / b (i32)
addf <dst> <a> <b>         # dst = a + b (f32)
subf <dst> <a> <b>         # dst = a - b (f32)
```

### 比較
```
clt  <dst> <a> <b>         # dst = (a < b)  ? 1 : 0
cle  <dst> <a> <b>         # dst = (a <= b) ? 1 : 0
ceq  <dst> <a> <b>         # dst = (a == b) ? 1 : 0
cne  <dst> <a> <b>         # dst = (a != b) ? 1 : 0
cgt  <dst> <a> <b>         # dst = (a > b)  ? 1 : 0
cge  <dst> <a> <b>         # dst = (a >= b) ? 1 : 0
```

### 制御フロー
```
label <name>               # ラベル定義
jmp   <label>              # 無条件ジャンプ
jnz   <cond> <t> <f>      # condが非ゼロなら<t>、ゼロなら<f>へジャンプ
```

### 関数
```
call  <dst> <func> [args]  # 関数呼び出し。戻り値をdstに格納
ret   <val>                # 関数から戻る
ret                        # void return
```

### 型変換
```
itof  <dst> <src>          # i32 → f32
ftoi  <dst> <src>          # f32 → i32
```

### 外部
```
extern <name>              # 外部シンボル宣言
```

---

## レジスタ命名規則

- `%<name>` — 仮想レジスタ（例: `%x`, `%t1`, `%ret2`）
- `$<name>` — 関数名（例: `$fibonacci`, `$sim_main`）

---

## サンプル

```cai
# fibonacci
func fibonacci
  alloc  %n.ptr 4
  store  %n.ptr %arg0
  load   %n 0%n.ptr
  alloc  %cond.ptr 4
  cle    %cond1 %n 1
  jnz    %cond1 base recurse
label base
  ret    %n
label recurse
  alloc  %a.ptr 4
  sub    %n1 %n 1
  call   %a fibonacci %n1
  store  %a.ptr %a
  sub    %n2 %n 2
  call   %b fibonacci %n2
  add    %result %a %b
  ret    %result
endfunc

export func sim_main
  alloc  %result.ptr 4
  call   %result fibonacci 10
  store  %result.ptr %result
  load   %t1 %result.ptr
  ret    %t1
endfunc
```

---

## CAI変換器

- **入力**: `.cai`（テキスト形式）
- **出力**: ネイティブバイナリ（x86_64）
- **実装言語**: C（踏み台・一回限り）
- **配布形式**: Cosmopolitan Libc APE形式（C依存ゼロ）
- **対応アーキテクチャ**: x86_64（初期）、arm64（将来）

