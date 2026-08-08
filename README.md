# Similarity

**No,GC. No,guessing. No,C/C++.**

No garbage collector. No compiler guessing. No dependency on C/C++.

作者: 奇曲 宮夢 (Kikyoku Miyu)
バージョン: v0.1.0 (Prototype)

---

## Why Similarity?

Similarityは、システムプログラミングにおける「暗黙的な動作」を可能な限り排除し、プログラマがプログラムの動作を明確に把握できることを目指しています。

* **No GC** — ガベージコレクタによる暗黙的なメモリ管理に依存しない
* **No compiler guessing** — コンパイラによる意図しない推測を避け、操作を明示する
* **Explicit unsafe** — unsafeなメモリアクセスは `Mem[risk{}]` として明示する
* **Explicit sharing** — 非同期処理間の共有状態は `share()` で明示する
* **No dependency on C/C++** — 特定のC/C++コンパイラやツールチェーンに依存しないバックエンドを構築する

Similarityは単にC/C++とは異なる構文を持つ言語ではありません。

**フロントエンドから機械語生成までを、自身で制御できるシステムプログラミング基盤**を目指しています。

---

# Compiler Architecture

Similarityのコンパイラは、複数の明確な段階に分離されています。

```text
.iia
  ↓
Lexer
  ↓
Parser
  ↓
AST
  ↓
TypeChecker
  ↓
Analyzer
  ↓
BackendFunction
  ↓
BIR
  ↓
C Backend
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

各段階は明確な責務を持ち、FrontendとBackendの間にはBIRを配置しています。

現在は旧CAIベースのバックエンドから、新しいBIRベースのバックエンドへ移行しています。

---

# BIR

BIR（Backend Intermediate Representation）は、FrontendとBackendを接続する中間表現です。

例えば、関数呼び出しを含むプログラムは以下のようなBIRへ変換されます。

```text
BIR 1

FUNC inc 0 4 0
PARAM x 4 0
BODY
RET 4 0 EXPR + 4 0 IDENT x 4 0 LIT_INT 1
ENDFUNC

FUNC main 1 4 0
LOCAL v 4 0
LOCAL i 4 0
BODY
STORE v 4 0 LIT_INT 0
STORE i 4 0 LIT_INT 0
LOOP 0
COND less i 4 0 1000000 4 0
LOOPBODY
STORE v 4 0 CALL inc 4 0 1 IDENT v 4 0
INCR i 4 0 INC
ENDLOOP
RET 4 0 IDENT v 4 0
ENDFUNC
```

BIRはASTをBackendが扱いやすい形へ整理し、以降のCFG構築やInstruction Selectionで使用されます。

---

# Control Flow Graph

BIRから制御フローグラフ（CFG）を構築します。

現在のCFG実装では、以下の制御構造を扱えます。

* Entry block
* Return block
* If
* If true / false
* Merge
* Loop header
* Loop body
* Loop exit
* Back edge
* Nested loop
* Function call
* Multiple functions

例えば単純なループは、

```text
Block #0 [entry]
    ↓
Block #1 [loop_header]
    ├── false → Block #2 [loop_exit]
    └── true  → Block #3 [loop_body]
                    ↓
                 Block #1
```

のように表現されます。

Nested loopではloop depthも保持します。

```text
Block #4 [loop_header] depth=1
Block #7 [loop_header] depth=2
```

各Blockは、

* Successors
* Predecessors
* Condition
* Loop depth
* Instructions

などの情報を保持します。

---

# Instruction Selection

CFG構築後、Instruction SelectionによってBackend上の操作をx86-64命令へ対応付けます。

現在の開発では、この段階をStage 4として実装しています。

```text
CFG
 ↓
Instruction Selection
 ↓
Virtual Registers
```

Instruction Selectionでは、プログラムの演算や制御を機械命令へ変換可能な形に落とし込みます。

例えば、

```text
a = b + c
```

という抽象的な演算を、最終的なx86-64命令列へ変換するためのMachine-level representationへ変換します。

---

# Virtual Registers

Instruction Selectionでは、物理レジスタへ直接割り当てず、Virtual Registerを使用します。

```text
v0 = ...
v1 = ...
v2 = ...
```

その後、

```text
Virtual Registers
        ↓
Register Allocation
        ↓
Physical Registers
```

という流れでx86-64の物理レジスタへ割り当てます。

これにより、Instruction SelectionとRegister Allocationの責務を分離します。

---

# Register Allocation

Virtual Registerをx86-64の物理レジスタへ割り当てます。

```text
Virtual Register
       ↓
Register Allocation
       ↓
RAX
RBX
RCX
RDX
...
```

Register Allocationでは、各Virtual Registerの使用状況や干渉関係などを考慮し、最終的な機械語生成へ接続します。

---

# x86-64 and ELF

最終的にはx86-64機械語を生成し、ELF実行ファイルとして出力します。

目標とするBackend pipelineは、

```text
BIR
 ↓
CFG
 ↓
Instruction Selection
 ↓
Virtual Registers
 ↓
Register Allocation
 ↓
x86-64 Machine Code
 ↓
ELF
```

です。

Similarity自身のBackendでこの経路を構築することを目標としています。

---

# Language Syntax

Similarityの基本構文は、

```text
カテゴリ[操作{引数}]
```

という構造を持ちます。

## Variables

```iia
Variable[let{int(x:10)}]
Variable[unclet{float(PI:3.14)}]

Mutation[variable{int(x:30)}]
```

## Operators

```iia
+{int(a,b)}
-{int(a,b)}
*{int(a,b)}
/{int(a,b)}

++{i}
--{i}
```

## Comparisons

```iia
equal(a:b)
notequal(a:b)
less(a:b)
lesseq(a:b)
greater(a:b)
greatereq(a:b)
```

## Control Flow

```iia
If[check{less(hp:0)},
  True{...},
  False{...}
]
```

Loop:

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

---

# Functions

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

---

# Arrays

```iia
Variable[let{Array_int(arr:10)}]

Mutation[array{int(arr:0:42)}]

Variable[let{int(val:index{arr(0)})}]
```

---

# Pointers and Explicit Unsafe Operations

ポインタ操作は明示的なunsafe領域として扱います。

```iia
Variable[let{int(ptr:addr{x})}]

Mem[risk{
  Variable[let{int(val:deref{ptr})}]
}]
```

`Mem[risk{}]`によってunsafeなメモリアクセスを明示します。

---

# Async and Shared State

非同期処理間で共有される状態は明示的に宣言します。

```iia
Async[{
  share(x),
  Mutation[variable{int(x:30)}]
}]
```

`share()`によって共有対象を明示し、暗黙的な共有状態を避けます。

---

# Modules

```iia
Import[math{}]
Import[io{}]
```

---

# Type Safety

TypeCheckerでは型に関する問題をコンパイル時に検出します。

エラー形式:

```text
行:列: TypeCheck Error [コード]: メッセージ
```

主なエラーコード:

| Code          | Description                      |
| ------------- | -------------------------------- |
| TC1001        | null許容型のnullチェックなしアクセス           |
| TC2001〜TC2010 | 型ミスマッチ・未宣言変数・配列型違反等              |
| TC3002        | `risk{}` 外での `deref` 使用          |
| TC4001        | 整数オーバーフロー                        |
| TC5001        | `share` 対象の未宣言変数                 |
| TC5002        | `Async` 内で `share` 宣言なしにMutation |

---

# Standard Library

| Library | Main functions              |
| ------- | --------------------------- |
| `math`  | `absolute_value`, `maximum` |
| `io`    | `io_print`                  |

詳細:

```text
docs/StandardLibrary.md
```

---

# Development Status

Similarityは現在Prototype段階です。

| Component             | Status            |
| --------------------- | ----------------- |
| Lexer                 | ✅ Implemented     |
| Parser                | ✅ Implemented     |
| AST                   | ✅ Implemented     |
| TypeChecker           | ✅ Implemented     |
| Analyzer              | ✅ Implemented     |
| BackendFunction       | ✅ Implemented     |
| BIR                   | ✅ Implemented     |
| BIR Serializer        | ✅ Implemented     |
| C Backend             | ✅ Implemented     |
| CFG Construction      | ✅ Implemented     |
| Nested CFG            | ✅ Implemented     |
| Function Calls in CFG | ✅ Implemented     |
| Instruction Selection | 🚧 In Development |
| Virtual Registers     | 🚧 In Development |
| Register Allocation   | 📋 Planned        |
| x86-64 Backend        | 📋 Planned        |
| ELF Generation        | 📋 Planned        |
| APE / Multi-OS        | 📋 Long-term      |
| GPU Backend           | 📋 Long-term      |
| Self-hosting          | 📋 Long-term      |

---

# CFG Validation

CFGは複数のベンチマークおよびストレスケースによって検証しています。

```text
benchmark/fibonacci
benchmark/sum
benchmark/matrix
benchmark/stress/bench_call
benchmark/ackermann
benchmark/bubble_sort
benchmark/control/bench_nested_loop
benchmark/eratosthenes
benchmark/bench_frontend_long
```

検証対象には、

* 単純な関数
* 関数呼び出し
* 再帰
* if/else
* loop
* nested loop
* loop back edge
* 複数関数
* 大量関数
* 複雑な制御フロー

が含まれます。

`bench_frontend_long.bir`では51関数を読み込み、各関数のCFGを正常に構築できることを確認しています。

---

# Building

Similarity本体:

```bash
go build -o sim ./cmd/
```

C Backend:

```bash
make -C cbackend
```

Tests:

```bash
go test ./analyzer -v
go test ./backend -v
```

---

# Running

通常の実行:

```bash
./sim benchmark/fibonacci/bench_fib.iia
```

Backendの確認:

```bash
./sim benchmark/fibonacci/bench_fib.iia --dump-backend
```

CFGの確認:

```bash
./sim benchmark/fibonacci/bench_fib.iia --dump-cfg
```

`--dump-cfg`では、各関数についてBlock、Condition、Successor、Predecessor、Loop depthなどを確認できます。

---

# Project Structure

```text
Similarity/
├── cmd/
│   └── main.go
│
├── lexer/
├── parser/
├── ast/
├── typecheck/
├── analyzer/
│
├── backend/
│   └── runner.go
│
├── cbackend/
│   ├── ...
│   ├── Makefile
│   └── sim_backend
│
├── stdlib/
├── examples/
├── benchmark/
└── docs/
```

Backendは以下のように段階的に分離されています。

```text
Analyzer
    ↓
BackendFunction
    ↓
BIR
    ↓
C Backend
    ↓
CFG
    ↓
Instruction Selection
```

---

# Design Principles

## 1. No Garbage Collector

ガベージコレクションを前提としない。

## 2. No Compiler Guessing

コンパイラがプログラマの意図を過剰に推測することを避ける。

## 3. Explicit Unsafe

unsafeな操作は明示的に記述する。

```text
Mem[risk{...}]
```

## 4. Explicit Sharing

非同期処理間の共有状態を明示する。

```text
share(x)
```

## 5. No Dependency on C/C++

特定のC/C++コンパイラをSimilarityのコンパイル経路に必要としないことを目指す。

## 6. Clear Compiler Boundaries

各コンパイル段階の責務を明確に分離する。

```text
Frontend
Semantic Analysis
IR
Control Flow
Instruction Selection
Register Allocation
Machine Code
```

---

# Current Development Stage

Similarityは現在、FrontendからBackendまでの基本的なコンパイルパイプラインを構築し、**CFG Constructionを完了した段階**にあります。

現在の中心的な開発対象は、

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

最終的な目標は、Similarityのソースコードからx86-64 ELFバイナリまでを、明確に定義された自前のコンパイルパイプラインによって生成することです。

---

# License

SimilarityのソースコードはMIT Licenseの下で公開されています。

ただし、**Elestrovicの名称、ロゴ、および関連するブランド要素はMIT Licenseの許諾対象ではありません。**

詳細については `LICENSE` を参照してください。

---

# Elestrovic

Similarity is developed as part of **Elestrovic**.

**Technology for the next generation.**

**利便性に「確かな安全を。」**
