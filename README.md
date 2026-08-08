# Similarity

**No,GC. No,guessing. No,C/C++.**

*No garbage collector. No compiler guessing. No dependency on C/C++.*

Similarityは、メモリ管理・型・危険な操作・並行処理などを可能な限り明示的に扱うことを目指して設計されたシステムプログラミング言語です。

作者: 奇曲 宮夢 (Kikyoku Miyu)
バージョン: v0.1.0 (Prototype)

---

## Similarityとは

Similarityは、システムプログラミングにおける「暗黙的な動作」を減らし、プログラマがプログラムの動作を明確に記述できることを重視しています。

設計思想は次の3点に集約されます。

* **No garbage collector.** — GCによるメモリ管理を前提としない
* **No compiler guessing.** — コンパイラによる過度な推測に依存しない
* **No dependency on C/C++.** — C/C++やGCC・LLVMを実行基盤として必要としない

これらは単なるキャッチコピーではなく、Similarityの設計方針を表すものです。

---

# コンパイラ

Similarityのコンパイラは、ソースコードを解析し、CAIによって最終的なx86-64 ELFバイナリまで変換します。

```text
.iia
 ↓
Lexer
 ↓
Parser
 ↓
TypeChecker
 ↓
Analyzer
 ↓
BackendFunction ←CAI側のBackendFunctionに送る
 ↓
CAI
 ├─ BackendFunction ←Go側のBackendFunctionを受け取る
 ├─ CFG
 ├─ Instruction Selection
 ├─ Virtual Registers
 ├─ Register Allocation
 ├─ x86-64
 └─ ELF
```

## CAI

**CAI（Common Assembly Instructions）は、Similarityのバックエンド全体を指します。**

CAIは単独の中間表現ではありません。

Similarityの`BackendFunction`から始まり、

```text
BackendFunction
    ↓
CFG
    ↓
Instruction Selection
    ↓
Virtual Registers
    ↓
Register Allocation
    ↓
x86-64
    ↓
ELF
```

までのバックエンド処理全体がCAIです。

そのため、SimilarityはCAIのために別の中間表現を挟む設計を採用していません。

CAIは、Similarityのフロントエンドとx86-64コード生成を接続するバックエンドとして設計されています。

※また、CAIは現在C言語で書かれていますが、将来的にはC依存の無いもので書きます。

---

# BackendFunction

`BackendFunction`は、Analyzerによる解析結果をCAIへ渡すためのバックエンド用表現です。

```text
Analyzer
   ↓
BackendFunction
   ↓
CAI
```

BackendFunctionは、ソースコードのASTそのものではなく、バックエンドが必要とする情報を保持します。

CAIはBackendFunctionを入力としてCFGを構築し、その後のコード生成処理を行います。

---

# CFG

CAIでは、BackendFunctionから制御フローグラフ（CFG）を構築します。

```text
BackendFunction
      ↓
     CFG
```

CFGでは、プログラムを基本ブロックへ分割し、

* ブロック間の制御フロー
* successor
* predecessor
* 条件分岐
* ループ
* ループ深度

などを管理します。

例えば、

```text
entry
  ↓
loop_header
 ├──→ loop_exit
 ↓
loop_body
 └──→ loop_header
```

のような制御構造をCAI内部で表現します。

---

# Instruction Selection

CFG構築後、CAIは各BackendFunctionの処理をx86-64向けの命令へ変換していきます。

```text
CFG
 ↓
Instruction Selection
```

この段階では、演算・比較・分岐・関数呼び出し・メモリアクセスなどを、ターゲットとなる命令列へ落とし込みます。

Similarityでは、最終的な機械語生成までをCAI内部で管理することを目指しています。

---

# Virtual Registers

命令選択によって生成された値は、仮想レジスタとして管理されます。

```text
Instruction Selection
        ↓
Virtual Registers
```

仮想レジスタによって、命令選択段階と物理レジスタ割り当てを分離します。

---

# Register Allocation

仮想レジスタを実際のx86-64物理レジスタへ割り当てます。

```text
Virtual Registers
       ↓
Register Allocation
```

必要に応じてスタック領域への退避も行います。

この処理では、CFGやライブ情報などを利用して、可能な限り適切なレジスタ割り当てを行います。

---

# x86-64

Register Allocation後、CAIはx86-64命令を生成します。

```text
Register Allocation
        ↓
      x86-64
```

アセンブラや外部コンパイラを介さず、Similarity自身のバックエンドから機械語を生成することを目標としています。

---

# ELF

生成したx86-64機械語をELF形式へ配置します。

```text
x86-64
  ↓
ELF
```

これにより、外部リンカに依存せず、CAIから直接実行可能なELFバイナリを生成します。

---

# 言語仕様

## 基本構文

Similarityでは、基本的に以下の構造を採用します。

```text
カテゴリ[操作{引数}]
```

## 変数

```iia
Variable[let{int(x:10)}]
Variable[unclet{float(PI:3.14)}]

Mutation[variable{int(x:30)}]
```

## 演算子

```iia
+{int(a,b)}
-{int(a,b)}
*{int(a,b)}
/{int(a,b)}

++{i}
--{i}
```

## 比較

```iia
equal(a:b)
notequal(a:b)
less(a:b)
lesseq(a:b)
greater(a:b)
greatereq(a:b)
```

## 制御フロー

```iia
If[check{less(hp:0)},
  True{...},
  False{...}
]
```

```iia
Loop[
  check{less(i:10)},
  for{
    ...,
    ++{i}
  }
]
```

```iia
break{}
continue{}
```

## 関数

```iia
Function[add{
  receive{int(a), int(b)},
  return(+{int(a,b)})
}]

Function_public[main{
  receive{},
  Variable[let{int(result:call{add(1,2)})}],
  return(result)
}]
```

## 配列

```iia
Variable[let{Array_int(arr:10)}]

Mutation[array{int(arr:0:42)}]

Variable[let{int(val:index{arr(0)})}]
```

## ポインタ

危険なメモリアクセスは`Mem[risk{}]`によって明示します。

```iia
Variable[let{int(ptr:addr{x})}]

Mem[risk{
  Variable[let{int(val:deref{ptr})}]
}]
```

## 非同期処理

共有変数へのアクセスは`share()`によって明示します。

```iia
Async[{
  share(x),
  Mutation[variable{int(x:30)}]
}]
```

---

# 安全性システム

Similarityでは、危険な操作や型に関する問題をコンパイル時に検出します。

エラーは、

```text
行:列: TypeCheck Error [コード]: メッセージ
```

の形式で報告されます。

| コード           | 内容                          |
| ------------- | --------------------------- |
| TC1001        | null許容型のnullチェックなしアクセス      |
| TC2001〜TC2010 | 型ミスマッチ・未宣言変数・配列型違反など        |
| TC3002        | `risk{}`外での`deref`使用        |
| TC4001        | 整数オーバーフロー                   |
| TC5001        | `share`対象の未宣言変数             |
| TC5002        | Async内で`share`宣言なしにMutation |

---

# 標準ライブラリ

| ライブラリ  | 主な機能                        |
| ------ | --------------------------- |
| `math` | `absolute_value`, `maximum` |
| `io`   | `io_print`                  |

詳細は`docs/StandardLibrary.md`を参照してください。

---

# 実装状況

| 機能                      | 状態      |
| ----------------------- | ------- |
| Lexer / Parser          | ✅ 実装済み  |
| TypeChecker             | ✅ 実装済み  |
| Analyzer                | ✅ 実装済み  |
| BackendFunction         | ✅ 実装済み  |
| CAI CFG                 | ✅ 実装済み  |
| Instruction Selection   | 🚧 開発中  |
| Virtual Registers       | 🚧 開発中  |
| Register Allocation     | 📅 予定   |
| x86-64コード生成             | 📅 予定   |
| ELF生成                   | 📅 予定   |
| 配列                      | ✅ 実装済み  |
| ポインタ / `deref` / `addr` | ✅ 実装済み  |
| 構造体                     | ✅ 実装済み  |
| Async / `share`         | ✅ 実装済み  |
| Echo                    | ✅ 実装済み  |
| Cell                    | ✅ 実装済み  |
| 標準ライブラリ                 | ✅ 実装済み  |
| APE形式                   | 📅 長期目標 |
| 自己ホスト                   | 📅 長期目標 |
| GPU本実装                  | 📅 長期目標 |

※ 実装状況は開発の進行に合わせて更新されます。

---

# 設計原則

### 1. コンパイラは推測しない

プログラムの重要な動作を暗黙的な推測に依存させず、可能な限り明示的に記述します。

### 2. unsafe操作を明示する

危険なメモリアクセスは`Mem[risk{}]`として明示します。

### 3. 共有状態を明示する

Async間で共有される変数は`share()`によって明示します。

### 4. GCに依存しない

メモリ管理をガベージコレクタに隠蔽せず、システムプログラミングに適した明示的なモデルを目指します。

### 5. C/C++に依存しない

SimilarityのバックエンドであるCAIは、C/C++やGCC、LLVMを実行基盤として必要としない独立したコード生成系を目指します。

### 6. 中間表現を増やさない

FrontendとBackendの間に不要な中間表現を追加せず、`BackendFunction → CAI`という直接的な構造を維持します。

---

# 開発状況

Similarityは現在プロトタイプ段階です。

現在はフロントエンドからBackendFunction、そしてCAIのCFG構築までの基盤が整備されています。

次の主要な段階は、

```text
CFG
 ↓
Instruction Selection
 ↓
Virtual Registers
 ↓
Register Allocation
 ↓
x86-64
 ↓
ELF
```

です。

Similarityは、最終的に外部コンパイラやリンカに依存せず、自身のバックエンドによって実行可能なバイナリを生成できるシステムプログラミング言語を目指します。
