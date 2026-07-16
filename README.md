# Similarity

**"No GC. No guessing. No C/C++"**

C/C++を玉座から引きずり降ろすために設計されたシステムプログラミング言語。

作者: 奇曲 宮夢 (Kikyoku Miyu)

---

## 概要

Similarityは、C/C++依存ゼロを目指したオリジナルのシステムプログラミング言語です。GCなし、コンパイラは推測しない、unsafe操作は明示必須、速度は妥協しない、という哲学のもとに設計されています。

---

## コンパイラパイプライン

```
.iia → lexer → parser → AST → typecheck → QBE IR → アセンブリ → バイナリ
```

将来的にはQBEをオリジナルIR（CAI）に置き換え、C/C++依存を完全排除します。

---

## 言語機能

### 基本
```iia
Variable[let{int(x:10)}]                   // 変数宣言
Variable[unclet{float(PI:3.14)}]           // 変更不可変数
Mutation[variable{int(x:30)}]              // 変数再代入
```

### 制御フロー
```iia
If[check{less(hp:0)}, True[...], False[...]]
Loop[for{int(i:0), less(i:10), step{1}}, Body[...]]
Loop[Count{int(i:10)}, Body[...]]
```

### 関数
```iia
Function[名前{
  receive{int(x)},
  処理,
  return(x)
}]
```

### ポインタ
```iia
Variable[let{int(x:42)}]
Variable[let{int(ptr:addr{x})}]    // アドレス取得

Mem[risk{
  Variable[let{int(val:deref{ptr})}]  // 参照外し
}]
```

### 配列・cast
```iia
Variable[let{Array_int(arr:0)}]
Variable[let{int(val:index{arr(i)})}]      // arr[i]
Variable[let{float(y:cast{float(x)})}]     // int→float
```

### 構造体
```iia
Variable[struct{User:String(name), int(age)}]   // 定義
Variable[let{user:User(name:"John", age:25)}]   // インスタンス生成
```

### 非同期
```iia
Variable[let{int(x:10)}]
Async[{
  share(x),                          // 共有変数の明示（必須）
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

### Echo（.eho）
コンパイル時にriskブロックを検出し、プロジェクト全体をスキャンしてレポートを生成します。

```
╔══════════════════════════════════════════╗
║        ⚠️   RISK BLOCK DETECTED  ⚠️        ║
╚══════════════════════════════════════════╝

  [1] main.iia : line 20-21
      → deref use

  safe: utils.iia, math.iia （riskブロックなし）

詳細は main.eho を確認してください。
コンパイルを続行しますか？ [Y/n]:
```

### Cell（.cel）
パッケージ管理ファイル。`project.cel`をプロジェクトルートに置きます。

```
name: MyProject
version: 0.1.0
dependencies:
  - discord
  - SDL2
```

---

## ベンチマーク結果

### 実行速度
| 言語 | 結果 | 時間 |
|------|------|------|
| Similarity (QBE) | 887459712 | 48.42ms |
| C++ (g++ -O2) | 887459712 | 46.578ms |

→ **ほぼ同等。計算結果も一致。**

### コンパイル速度（1000回平均）

条件: Similarityは`--ir-only`、C++は`-S`、大規模ファイル

| 言語 | 平均時間 |
|------|----------|
| Similarity | 0.057s |
| C++ | 0.312s |

→ **約5.5倍速い**

小規模ファイルでは約68倍速い。

---

## 使い方

配布サイトからダウンロードしてください。

```
./sim your_file.iia
./sim --ir-only your_file.iia   # QBE IRのみ生成（フロントエンドのみ）
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
| Echo（.eho） | ✅ |
| Cell（.cel） | ✅ |
| CAI IR | 🔶 設計確定、実装未着手 |
| 高級版構文 | 🔶 設計中 |
| 標準ライブラリ | 🔶 未着手 |
| GPU本実装 | 🔶 CAI完成後 |
| 自己ホスト | 📅 長期目標 |

---

## 設計原則

1. **コンパイラは推測しない** — 全て明示
2. **unsafe操作はMem[risk{}]で明示**（Echoが自動レポート）
3. **Async間の共有変数はshare()で明示**
4. **速度は妥協しない** — GCなし、ゼロコスト抽象化
5. **C/C++依存ゼロ**（CAI完成後）
