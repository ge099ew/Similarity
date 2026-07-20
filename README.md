# Similarity

**"No GC. No guessing. No C/C++"**

C/C++を玉座から引きずり降ろすために設計されたシステムプログラミング言語。

作者: 奇曲 宮夢 (Kikyoku Miyu)

---

## 概要

SimilarityはC/C++依存ゼロを目指したオリジナルのシステムプログラミング言語です。GCなし、コンパイラは推測しない、unsafe操作は明示必須、速度は妥協しない、という哲学のもとに設計されています。

---

## コンパイラパイプライン

```
.iia → lexer → parser → AST → typecheck → echo → codegen → QBE IR → バイナリ
.sml → transpiler → .iia → 上記パイプライン

--caiフラグ使用時:
.iia → lexer → parser → AST → typecheck → caigen → CAI IR → cai_conv → バイナリ
```

**CAI（Common Assembly Instructions / 共通アセンブリ命令）** はSimilarity独自のIRです。
`cai_conv`はx86_64機械語を直接生成し、GCCのアセンブラ（as）を使いません。
GCCはリンク（ld）のみに使用します。最終的にldも自前実装に置き換えます。

---

## ファイル形式

| 拡張子 | 説明 |
|---|---|
| `.iia` | 低レイヤー構文（本来の形式） |
| `.sml` | シュガーシンタックス（`.iia`にトランスパイルして使用） |
| `.cai` | CAI IR（テキスト形式、Phase1） |

---

## 言語機能

### 基本パターン
```
カテゴリ[操作{引数}]
```

### 変数
```iia
Variable[let{int(x:10)}]
Variable[unclet{float(PI:3.14)}]
Mutation[variable{int(x:30)}]
```

### シュガーシンタックス（.sml）
```sml
Var[let{int(x:10)}]
Func[name{...}]
Func_pub[name{...}]
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
Function[name{
  receive{int(x)},
  処理,
  return(x)
}]
Function_public[name{...}]
call{name(args)}
```

### ポインタ
```iia
Variable[let{int(x:42)}]
Variable[let{int(ptr:addr{x})}]
Mem[risk{
  Variable[let{int(val:deref{ptr})}]
}]
```

### 配列・cast
```iia
Variable[let{Array_int(arr:0)}]
Variable[let{int(val:index{arr(i)})}]
Variable[let{float(y:cast{float(x)})}]
```

### 構造体
```iia
Variable[struct{User:String(name), int(age)}]
Variable[let{user:User(name:"John", age:25)}]
```

### 非同期
```iia
Async[{
  share(x),
  Mutation[variable{int(x:30)}]
}]
Await[task]
```

### エラーハンドリング
```iia
Error[try{処理}, Ok[...], Err[type{FileNotFound}, msg{"..."}]]
Fatal[type{OutOfMemory}, msg{"回復不能"}]
```

### モジュール
```iia
Import[math{}]
Import[discord{}]
Extern[C{lib{"SDL2"}, draw{receive{int(x)}, return{}}}]
```

---

## 安全性システム（コンパイル時）

| エラーコード | 内容 |
|---|---|
| TC1001 | null許容型のnullチェックなしアクセス |
| TC2001〜TC2010 | 型ミスマッチ・未宣言変数・配列型違反等 |
| TC3002 | risk{}外でのderef使用 |
| TC4001 | 整数オーバーフロー（32bit範囲超え） |
| TC5001 | share: 未宣言変数 |
| TC5002 | Async内でshare宣言なしにMutation |

---

## サポートシステム

### Echo（project.eho）
コンパイル時にプロジェクト全体をスキャンしてriskブロックを検出。`project.eho`としてプロジェクト単位で1つ生成されます。

```
╔══════════════════════════════════════════╗
║        ⚠️   RISK BLOCK DETECTED  ⚠️        ║
╚══════════════════════════════════════════╝

  [1] main.iia : line 20-21
      → deref use

コンパイルを続行しますか？ [Y/n]:
```

### Cell（project.cel）
```
name: MyProject
version: 0.1.0
dependencies:
  - math
  - discord
```

---

## ベンチマーク結果（100回平均・コールドスタート）

### 実行速度

| 比較項目 | Similarity | C++ (-O0) |
|---|---|---|
| fibonacci(40) | 713ms | 453ms |
| 総和（0〜1億） | 23ms | 67ms（**2.8倍速い**） |

### フロントエンド速度（コンパイル時間）

| 比較項目 | Similarity | C++ |
|---|---|---|
| 短いファイル | 2.25ms | 8.02ms（**3.6倍速い**） |
| 長いファイル | 2.89ms | 8.22ms（**2.8倍速い**） |

---

## 使い方

```bash
# ビルド
go build -o sim ./cmd/
gcc -O2 -o cai_conv cai_converter/cai_converter.c

# 実行
./sim your_file.iia             # QBEバックエンド
./sim --cai your_file.iia       # CAIバックエンド（自前アセンブラ）
./sim --ir-only your_file.iia   # QBE IRのみ生成
./sim your_file.sml             # シュガーシンタックス
```

---

## 実装状況

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
| typecheck（コンパイル時安全性） | ✅ |
| Echo（project.eho） | ✅ |
| Cell（project.cel） | ✅ |
| return()構文 | ✅ |
| シュガーシンタックス（.sml） | ✅ |
| stdlib/math | ✅ |
| CAI IR | ✅ |
| CAI変換器（x86_64直接生成） | ✅ |
| asの代替（機械語直接生成） | ✅ |
| ldの代替（ELF直接生成） | 🔶 実装中 |
| 標準ライブラリ拡張 | 🔶 未着手 |
| 各言語互換性レイヤー | 🔶 未着手 |
| GPU本実装 | 🔶 CAI安定後 |
| APE形式（マルチOS） | 🔶 未着手 |
| 自己ホスト | 📅 長期目標 |

---

## 設計原則

1. **コンパイラは推測しない** — 全て明示
2. **unsafe操作はMem[risk{}]で明示**（Echoが自動レポート）
3. **Async間の共有変数はshare()で明示**
4. **速度は妥協しない** — GCなし、ゼロコスト抽象化
5. **C/C++依存ゼロ** — CAI変換器がasを排除、ldも順次置き換え

---

## 開発進捗

### 現在のフォーカス
CAIバックエンドの最適化・C/C++依存の段階的排除

### Phase 1: フロントエンド完成（✅ 完了）
- lexer / parser / AST
- typecheck（null安全・型整合性・オーバーフロー検出・データ競合防止）
- Echo（project.eho）/ Cell（project.cel）
- QBEバックエンド
- シュガーシンタックス（.sml）
- stdlib/math

### Phase 2: CAIバックエンド（🔶 進行中）
- CAI IR設計・実装 ✅
- caigen（Go→CAI IR生成）✅
- cai_conv（CAI→x86_64機械語直接生成）✅
- asの代替（GCCのアセンブラを排除）✅
- ldの代替（ELF直接生成）🔶 実装中
- APE形式（Cosmopolitan、マルチOS対応）📅

### Phase 3: エコシステム（📅 未着手）
- 標準ライブラリ拡張（io等）
- 各言語互換性レイヤー（Python/Rust/Java/C#/Odin/JS/Go/Zig）
- GPU本実装
- 自己ホスト
