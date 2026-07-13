# このREADMEは古いままなので、誤ってる部分がある可能性があります。

# Similarity言語 進捗状況
 
## 概要
C++の「魔の領域」を安全・高速に置き換えることを目標としたオリジナルプログラミング言語。
 
---
 
## 実装済み ✅
 
### コンパイラパイプライン
```
.iia → lexer → parser → AST → QBE IR → アセンブリ → バイナリ
```
 
### 言語機能
- 変数宣言 `Variable[let{int(x:10)}]`
- 変数再代入 `Mutation[variable{int(x:30)}]`
- If文 `If[check{le(hp,0)}, True[...], False[...]]`
- ループ `Loop[for{...}, Body[...]]` / `Loop[Count{...}, Body[...]]`
- 関数定義 `Func[名前{receive{...}, 処理, return{...}}]`
- 再帰関数（fibonacci動作確認済み）
- return文（If文の中でも使用可能）
- エラーハンドリング `Error[try/Ok/Err]` / `Fatal[...]`
- モジュール `Import[...]` / `Extern[...]`
- 演算子 `+{int(a,b)}` `-{...}` `*{...}` `/{...}`
- 比較演算子 `eq` `le` `lt` `ge` `gt` `ne`
- `--ir-only` フラグ（フロントエンドのみ実行）
- VSCodeシンタックスハイライト（`.iia`ファイル対応）
---
 
## ベンチマーク結果
 
### 実行速度
| 言語 | 結果 | 時間 |
|------|------|------|
| Similarity (QBE) | 887459712 | 48.42ms |
| C++ (g++ -O2) | 887459712 | 46.578ms |
 
→ **ほぼ同等。計算結果も一致。**
 
## 大規模ファイルベンチマーク（1000回平均）
 
### 条件
```
Similarity: --ir-only（lexer+parser+AST+IR生成）
C++:        -S（lexer+parser+型チェック+アセンブリ生成）
ファイル規模: Similarity約7800行 / C++約6400行
試行回数: 1000回
```
 
| 言語 | 合計時間 | 平均時間 |
|------|----------|----------|
| Similarity | 57.069s | 0.057s |
| C++ | 311.865s | 0.312s |
 
```
Similarity: time sh -c 'for i in $(seq 1 1000); do ./sim --ir-only benchmark/bench_large.iia >/dev/null 2>&1; done'
C++       : time sh -c 'for i in $(seq 1 1000); do g++ -S benchmark/bench_large.cpp -o /dev/null 2>&1; done'
```

→ **約5.5倍速い（公平な条件での比較）**
 
### 補足
- 前回（小ファイル）は68倍、今回（大ファイル）は5.5倍
- ファイルが大きくなるとC++のオーバーヘッド比率が下がるため差が縮まるのは自然
- SimilarityはC++より仕事量が多い（IR生成まで）のに5.5倍速い
- Similarityはマルチコアを使用。(仕様)
- これは**本物の数字**
| 言語 | 計測方法 | 時間 |
|------|----------|------|
| Similarity | `--ir-only` | 0.005s |
| C++ | `-fsyntax-only` | 0.341s |
→ **約68倍速い。**
 
---
 
## 未実装
 
- 高級版シンタックスシュガー（自己ホスト）
- モジュールシステムの完全実装
- C++ライブラリとのinterop
- 型の互換性（C++複合型との対応）
- 非同期・GPU処理
---
 
## 今後の予定
 
```
① 高級版構文の実装（.iiaで高級版を書く）
② 自己ホスト（SimilarityでSimilarityを書く）
③ モジュールシステム完全実装
④ C++interop
```

