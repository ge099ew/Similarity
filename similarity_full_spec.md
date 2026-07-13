# Similarity 言語仕様書 完全版

> C/C++を玉座から引きずり降ろすために設計されたシステムプログラミング言語。

---

## 概要

| 項目 | 内容 |
|------|------|
| 目的 | C/C++依存ゼロのシステムプログラミング言語 |
| 拡張子 | `.iia` |
| コンパイラ | Go製（動作中） |
| バックエンド | CAI（開発中） / QBE（現在使用中） |
| 出力 | ネイティブバイナリ（C/C++トランスパイルではない） |
| マルチOS | Cosmopolitan Libc（APE形式）予定 |

---

## アーキテクチャ

```
人間が書くコード（高級版・シンタックスシュガー）
         ↓
      .iia（テキストAST・低レイヤー）
         ↓
    typecheck（コンパイル時安全性チェック）
         ↓
    CAI IR（開発中） / QBE IR（現在）
         ↓
    ネイティブバイナリ
```

- `.iia` はコンパイラが「推測しなくていい」完全明示的な中間表現
- 高級版は `.iia` へ1対1で変換される糖衣構文
- CAIはQBEを完全に置き換えるオリジナルIR（C/C++依存ゼロ）

---

## サポートシステム

### Cell（.cel）
パッケージ管理ファイル。依存関係・バージョン情報を記述。

### Echo（.eho）
開発者サポートシステム。コンパイル時にriskブロックを検出してレポートを生成。

**コンパイル中（CLI警告）:**
```
⚠️  risk block detected → test_all.eho を確認してください。
```

**コンパイル後（.ehoレポートファイル）:**
```
Similarity Echo Report
Generated : 2026-07-13 18:58:25
Source    : test_all.iia
Risk Blocks: 1
----------------------------------------

[1] test_all.iia : line 20-21
    → deref use
    メモリ安全性は保証されません。
```

---

## 基本構文パターン

```
カテゴリ[操作{引数}]
```

修飾子はアンダースコアで付加する：
```
Box_int      // ヒープ配置のint
Func_pub     // 公開関数
```

---

## Explanation システム ✅ 決定

全ファイルの先頭に記述。コンパイラに「このファイルが何をするか」を最初から伝える。

```iia
Explanation[Application{Game(type:RPG, name:Minecraft)}]
Explanation[Bridge{Cxx(lib:"SDL2")}]
Explanation[System{HFT}]
Explanation[Module{Math}]
```

---

## 変数

```iia
Variable[let{int(x:10)}]                        // 変更可・整数x = 10
Variable[unclet{float(PI:3.14)}]                // 変更不可
Variable[struct{User, String(name), int(age)}]  // 構造体定義
Variable[let{User(name:"John", age:25)}]        // 構造体インスタンス
```

| | 読む | 書く | Move |
|---|---|---|---|
| `let` | ○ | ○ | ○ |
| `unclet` | スコープ内のみ | ✗ | ✗ |

---

## 型システム

```iia
int(x)           // スタック配置
Box_int(x)       // ヒープ配置
float(x)
String(x)
Array_int(x)     // int配列
Array_float(x)   // float配列
Array_bool(x)    // bool配列
```

---

## 演算子

```iia
+{int(a, b)}                    // a + b
*{int(c), +{int(a, b)}}         // c * (a + b)
```

**比較演算子**
```iia
equal(a:b)      // a == b
notequal(a:b)   // a != b
less(a:b)       // a < b
lesseq(a:b)     // a <= b
greater(a:b)    // a > b
greatereq(a:b)  // a >= b
```

---

## 制御フロー

**If文**
```iia
If[
  check{less(hp:0)},
  True[処理],
  False[処理]
]
```

**ループ（回数指定）**
```iia
Loop[
  Count{int(i:10)},
  Body[処理]
]
```

**ループ（条件付き）**
```iia
Loop[
  for{int(i:0), less(i:10), step{1}},
  Body[処理]
]
```

---

## 関数

```iia
Function[計算{
  receive{int(x), String(name)},
  処理,
  return(戻り値)
}]

// 公開関数
Function_pub[計算{...}]

// 呼び出し
call{計算(引数)}
```

---

## ポインタ

```iia
Variable[let{int(x:10)}]
Variable[let{int(ptr:addr{x})}]    // xのアドレスを取得

Mem[risk{
  Variable[let{int(val:deref{ptr})}]  // ptrが指す値を読む
}]
```

---

## 配列アクセス

```iia
Variable[let{Array_int(arr:0)}]
Variable[let{int(val:index{arr(i)})}]  // arr[i]
```

---

## cast

```iia
Variable[let{int(x:10)}]
Variable[let{float(y:cast{float(x)})}]  // int → float
Variable[let{int(z:cast{int(y)})}]      // float → int
```

---

## メモリ管理

- **閉じ括弧で自動解放** — GCなし
- **危険領域の隔離** — `Mem[risk{...}]` で手動管理を明示（Echoが自動検出・レポート）

---

## 安全性システム（コンパイル時）

typecheckパッケージがコンパイル時に以下を検出・中断する。

| エラーコード | 内容 |
|---|---|
| TC1001 | null許容型のnullチェックなしアクセス |
| TC2001 | 型ミスマッチ（代入） |
| TC2002 | 未宣言変数へのMutation |
| TC2003 | Mutation型ミスマッチ |
| TC2004 | 比較型ミスマッチ |
| TC2005 | 演算型ミスマッチ |
| TC2006 | cast元が数値型でない |
| TC2007 | cast先がサポート外 |
| TC2008 | 未宣言配列へのindex |
| TC2009 | 非配列型へのindex |
| TC2010 | 配列インデックスがint以外 |
| TC3001 | Awaitの対象が未宣言 |
| TC3002 | risk{}外でのderef使用 |
| TC4001 | 整数オーバーフロー（32bit範囲超え） |
| TC5001 | share: 未宣言変数 |
| TC5002 | Async内でshare宣言なしにMutation |

---

## 非同期・並行処理

```iia
Variable[let{int(x:10)}]

Async[{
  share(x),                          // x を共有変数として明示（必須）
  Mutation[variable{int(x:30)}]      // share宣言があれば変更可
}]

Await[task]                          // スレッドの完了を待つ
```

**share宣言なしにAsync内でMutationするとTC5002エラー。**

---

## エラーハンドリング

```iia
Error[
  try{処理},
  Ok[処理],
  Err[type{FileNotFound}, msg{"config.txt が見つかりません"}]
]

Fatal[type{OutOfMemory}, msg{"回復不能"}]
```

**エラーナンバー体系**

| 番台 | 種類 |
|------|------|
| `1xxxx` | 構文エラー |
| `2xxxx` | 型エラー |
| `3xxxx` | メモリエラー |
| `4xxxx` | 実行時エラー |
| `5xxxx` | Fatalエラー |

---

## モジュールシステム

```iia
Import[discord{}]
Import[discord{token, api}]
Import[myfile.iia{}]
```

---

## C/C++ interop

```iia
Extern[C{
  lib{"SDL2"},
  draw_sprite{receive{int(x), int(y)}, return{}}
}]

call{draw_sprite(10, 20)}
```

---

## CAI（Common Assembly Instructions / 共通アセンブリ命令）

QBEを完全に置き換えるオリジナルIR。

**パイプライン:**
```
.iia → Go(本体) → CAI(テキスト) → CAI変換器(APE形式) → バイナリ
```

**命令セット:**
```
func <name>          // 関数定義
alloc <name> <size>  // スタック確保
store <name> <value> // 値をストア
load  <name>         // 値を読む
add/sub/mul/div      // 演算
cmp / jlt/jle/jeq/jne/jgt/jge/jmp  // 比較・分岐
label <name>         // ラベル
call <name> <args>   // 関数呼び出し
extern <name>        // 外部関数
ret <value>          // return
```

**フォーマット:** Phase1はテキスト形式（デバッグ用）→ Phase2でバイナリ形式に移行

---

## 設計原則

1. **コンパイラは推測しない** — 全て明示
2. **スタック/ヒープは型で明示**
3. **unc は言語全体の不変の印**
4. **危険操作はMem[risk{}]で明示、Echoが自動レポート**
5. **C/C++依存ゼロ（CAI完成後）**
6. **Async間の共有変数はshare()で明示**
7. **アンダースコアが修飾子**（Box_int、Func_pub）

---

## 実装状況

| 機能 | 状態 |
|---|---|
| lexer/parser | ✅ 動作中 |
| codegen（QBE） | ✅ 動作中 |
| ポインタ（addr/deref） | ✅ 実装済み |
| 配列アクセス（index） | ✅ 実装済み |
| cast（int↔float） | ✅ 実装済み |
| Mem[risk{}] | ✅ 実装済み |
| Async/Await | ✅ 実装済み（pthread） |
| typecheck | ✅ 実装済み |
| share() | ✅ 実装済み |
| Echo（.eho） | ✅ 実装済み |
| CAI IR | 🔶 設計確定、実装未着手 |
| Cell（.cel） | 🔶 設計中 |
| 高級版構文 | 🔶 設計中 |
| 借用チェッカー代替 | ✅ share()で部分実装 |
| GPU | 🔶 CPUフォールバック中 |
| 自己ホスト | 📅 長期目標 |
