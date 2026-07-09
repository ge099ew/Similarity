# Similarity言語 完全引き継ぎドキュメント

> このドキュメントは新しいチャットでSimilarity言語コンパイラ開発を
> 完璧に引き継ぐための完全まとめです。

---

## 1. プロジェクトの目的

C++の「魔の領域」（UnrealEngine、CUDA、既存の巨大C++資産が必要な場面）を、
**安全性・コンパイル速度・実行速度**の三点で置き換えることを目指すオリジナル言語。

### 核心哲学
```
「コンパイラは推測しない」
```
全てを明示的に書くことで、コンパイラの解析コストを最小化する。
この哲学がフロントエンド速度（後述: C++比5.5倍〜68倍速い）に直結している。

### 開発者の性格・進め方
- お世辞は要らない、正直なフィードバックを求める
- 自分でコードを書きたい気持ちがあるが、実装はAIに頼ることが多い
- 日本語で会話、時々関西弁混じり
- 「化け物じゃない？」「大勝利？」のようなテンションで喜ぶタイプ
- 数値的根拠を重視し、誇張表現（「80倍速い」等）を嫌う。不正確な主張は必ず訂正すること

---

## 2. 言語設計の全体像

### 基本文法パターン（唯一のルール）
```
カテゴリ[操作{引数(詳細:値)}]
```
例: `スポーツ[サッカー{キーパー(背番号:10)}]` と同じ構造。
**このルール1つだけで言語全体が構成されている**（if/loop/関数も全部同じ形）。

### ファイル形式
- 拡張子: `.iia`（低レイヤー、テキストAST）
- 将来的に「高級版」構文も検討中（本ドキュメント6章参照）

---

## 3. 確定している構文仕様

### 3.1 ファイル先頭の宣言（Explanation）
コンパイラに最適化方針を伝える。「コンパイラが推測しない」哲学の一部。
```iia
Explanation[Application{Game(type:RPG, name:Minecraft)}]
Explanation[Bridge{Cxx(lib:"SDL2")}]      // C++橋渡し専用ファイル
Explanation[System{HFT}]                   // 極限低レイテンシ、Await自動挿入なし
Explanation[Module{Math}]
```
これにより以下4つの問題を解決できる可能性が判明:
- コンパイル速度向上（QBEが最初から方針を選べる）
- C++ブリッジ宣言（型変換の自動処理）
- 課題B: HFT最適化（Await自動挿入の無効化）
- 型の互換性（Bridgeファイル内での変換ルール適用）

> 補足：
- 下記のような形を目指している。
```iia
Explanation[Application{Game(type:RPG, name:Minecraft)},
            Bridge{Cxx(lib:"SDL2")},
]
```

### 3.2 変数
```iia
Variable[let{int(x:10)}]                  // 変更可能
Variable[unchanging_let{float(PI:3.14)}]  // 変更不可、スコープ内読み取りのみ
Variable[let{Array_int(x:10,20,30)}]      // 配列
Variable[struct{User, String(name), int(age)}]  // 構造体定義（未実装寄り）
```

**let / unchanging_let の違い:**
| | 読む | 書く | 引っ越し(Move) |
|---|---|---|---|
| let | ○ | ○ | ○ |
| unchanging_let | スコープ内のみ | ✗ | ✗ |

### 3.3 再代入（Mutation）
```iia
Mutation[variable{int(x:30)}]
```
※ `variable`（小文字）はMutation専用トークンで、`Variable`（大文字、宣言用）とは別トークン扱い。

### 3.4 型システム
```iia
// スタック系
int(x:10) / float(x:3.14) / bool(x:true) / String(x:"hello")
// ヒープ系
Box_int(x) / Box_float(x)
// 配列系
Array_int(x:10,20,30) / Array_float(...) / Array_bool(...)
```

**C++変換テーブル（Bridgeファイル用）:**
| Similarity | C++ |
|---|---|
| int | int |
| float | float |
| bool | bool |
| String | char* |
| Box_int | int* |
| Array_int | std::vector<int> |

### 3.5 演算子（略語廃止・コロン記法確定）
```iia
+{int(a:b)}   // a + b
-{int(n:1)}   // n - 1
*{int(a:b)}
/{int(a:b)}
```
**重要な経緯:** 当初 `+{int(a,b)}` のカンマ区切りだったが、
「算数の直感 vs マトリョーシカ構造の一貫性」を議論した結果、
コロン記法（`a:b` で「基準:対象」）に統一。
**カンマ(,)は「並列な要素の区切り」、コロン(:)は「値の割り当て・基準と対象」という役割分担で確定。**

### 3.6 比較演算子（略語完全廃止・コロン記法確定）
```iia
equal(a:b)      // ==  (旧: eq)
notequal(a:b)   // !=  (旧: ne)
less(a:b)       // <   (旧: lt)
lesseq(a:b)     // <=  (旧: le)
greater(a:b)    // >   (旧: gt)
greatereq(a:b)  // >=  (旧: ge)
```
**経緯:** 「le, lt, ge等は初見で意味不明」という指摘から略語を完全廃止。
比較演算子の引数区切りも最終的にカンマ→コロンに統一済み（generate_bench.py作成時に確定）。

### 3.7 制御フロー
```iia
If[check{lesseq(hp:0)},
  True[処理],
  False[処理]
]
```
`True[]`/`False[]`が明示的なため、課題A（分岐のゾンビ検知）が
「全分岐でMove/Dropを強制する」ルールで解決可能。

```iia
// 条件付きループ
Loop[for{int(i:0), less(i:10), step{1}}, Body[処理]]
// 回数指定ループ
Loop[Count{int(i:10)}, Body[処理]]
```

`break{}` / `continue{}` も実装済み（ラベル管理はcodegen側で対応）。

### 3.8 関数（略語廃止確定）
```iia
Function[名前{
  receive{int(x), String(name)},
  処理,
  return{戻り値}
}]

Function_public[名前{...}]  // 公開関数（旧: Function_pub → Function_public に変更）
```
呼び出し: `call{関数名(引数)}`
再帰呼び出し確認済み（fibonacci(10)=55で動作検証済み）。

### 3.9 return文
`If`の中でも使える（ReturnNodeとして実装、`hasReturn`判定でjmp重複を防止）。
```iia
If[check{lesseq(n:1)}, True[return{n}], False[...]]
```

### 3.10 エラーハンドリング
```iia
Error[
  try{処理},
  Ok[処理],
  Err[type{FileNotFound}, msg{"見つかりません"}]
]
Error[try{call{f()}}, Ok[...], Err[pass{}]]  // 呼び出し元へ伝播
Fatal[type{OutOfMemory}, msg{"回復不能"}]     // 回復不能エラー
Error[def{UserNotFound}]                       // カスタムエラー型定義
```

**エラーメッセージフォーマット（確定）:**
```
Error: {行番号} line, {問題のコード}({簡潔な説明}). errornumber{番号}
```
**エラーナンバー体系:**
| 番台 | 種類 |
|---|---|
| 1xxxx | 構文エラー |
| 2xxxx | 型エラー |
| 3xxxx | メモリエラー |
| 4xxxx | 実行時エラー |
| 5xxxx | Fatalエラー |

### 3.11 モジュール・外部連携
```iia
Import[discord{}]              // 全部インポート
Import[discord{token}]         // 一部のみ
Import[myfile.iia{}]           // 自作ファイル

Extern[C{
  lib{"SDL2"},
  draw_sprite{receive{int(x), int(y)}, return{}}
}]
// lib{notcle} = "not clear"（ライブラリ不明時）※現在も略語のまま、要検討
```
C ABI経由で呼び出すためオーバーヘッドはほぼゼロ（extern "C"と同じ機械語になる）。

### 3.12 非同期・GPU（骨格のみ実装、中身はコメント出力レベル）
```iia
Async[{処理}]
Await[task]
GPU[{処理}]
```
`Explanation[System{HFT}]`でAwait自動挿入を無効化する設計（詳細未実装）。

### 3.13 危険領域の隔離
```iia
Mem[risk{
  // ポインタ操作、生メモリアクセスなど
}]
```
ASTノード（RawMemNode）は実装済みだが、codegenは中身をそのまま出力するだけで
実際のポインタ安全性チェックは未実装。

### 3.14 その他実装済みノード（骨格〜コメントレベル）
```iia
addr{x}          // ポインタのアドレス取得（未実装、コメント出力のみ）
deref{ptr}       // 参照外し（loadwするだけ、型安全性なし）
cast{int(x)}     // 型キャスト（copyするだけ）
index{arr(i)}    // 配列アクセス（loadwするだけ、実際のインデックス計算なし）
```

### 3.15 修飾子ルール
```
アンダースコアは修飾子（Box_int, Function_public）
```

---

## 4. 未解決だった課題とその解決

### 課題A: 分岐のゾンビ検知 ✅解決
**全分岐ルール:** どちらかのブランチでMoveしたら、全ブランチでMove/Dropを強制する。
`True[]`/`False[]`構造が既に明示的なので、重いデータフロー解析なしで実現可能。
```iia
If[check{...}, True[Move(hp)], False[Drop(hp)]]  // OK
If[check{...}, True[Move(hp)], False[処理]]       // エラー: 全分岐でMove/Drop必須
```
エラー番号例: `errornumber30001`

### 課題B: 極限最適化 ✅解決（設計レベル）
`Explanation[System{HFT}]`でAwait自動挿入を無効化。完全手動制御。

### 課題C: 認知負荷 ✅トレードオフとして確定（解決不要と判断）
`Box_int`等の明示記述は認知負荷を生むが、「バグのコスト」より
「意識するコストを払う」ことを意図的に選んだ設計。解決すべき問題ではない。

---

## 5. 実装の技術的経緯（重要なバグと解決策）

現在のコンパイラはGo言語で実装されている。パイプライン:
```
.iia → lexer → parser → AST → codegen(QBE IR) → qbe → gcc → バイナリ
```
QBEが使えない環境ではcgen（C言語出力）にフォールバックする設計。

### ディレクトリ構成
```
similarity/
├── cmd/main.go       — エントリーポイント、--ir-onlyフラグ対応
├── lexer/lexer.go    — トークナイザ
├── parser/parser.go  — 構文解析
├── ast/ast.go        — AST定義
├── codegen/codegen.go — QBE IR生成（メインバックエンド）
├── cgen/cgen.go      — C言語出力（QBEなし環境用フォールバック）
├── error/error.go    — エラーメッセージ生成
└── benchmark/        — ベンチマークファイル群
```

### 5.1 QBEがSSA形式であることに起因した設計変更
QBEは各変数に1回しか代入できない（SSA制約）。そのため**メモリベースの変数管理**を採用:
```
Variable[let{int(x:10)}]
→ %x.ptr =l alloc4 4      // スタック確保
→ storew 10, %x.ptr        // 値をストア
（読むときは毎回 loadw で読む）
```
関数パラメータは`c.params`マップで直接SSA変数として扱い、
ローカル変数は`c.vars`マップでポインタとして扱う、という2系統の管理をしている。

### 5.2 解決した主なバグ
1. **QBE比較命令名**: `cltw`ではなく`csltw`（符号付き整数には`s`が必要）
2. **ret後にjmpを書けない**: QBEのブロック終端ルール。`hasReturn`判定で分岐後のjmp出力を制御
3. **parseFunc内でreturnがループ終了条件になっていた**: `True[return{n}]`のような
   ブロック内returnとfunc末尾のreturnが混同していた。ループ条件から`TOKEN_RETURN`除外で解決
4. **call{}を式として使えなかった**: `parseLiteral()`に`TOKEN_CALL`ケース追加
5. **パラメータをloadwしようとしていた**: `c.params`と`c.vars`を分離して解決
6. **genIfのjmpにendLabel引数が抜けていた**: バグとして発見・修正済み
7. **変数シャドーイング（cgen側）**: ループ内で毎回`int sum = ...`と再宣言していた。
   既存変数は再代入のみにするよう修正

### 5.3 現在の主要トークン一覧（略語廃止後、最新版）
```go
// カテゴリ
Explanation, Variable, Function, Function_public, If, Loop, Error, Fatal,
Import, Extern, Async, Await, GPU, Mem, Mutation

// 操作・キーワード
let, unchanging_let, for, Count, check, True, False, Body, step,
receive, return, call, try, Ok, Err, pass, def, Move, Drop, Raw, lib,
variable（Mutation専用、小文字）

// 比較演算子（略語廃止済み）
equal, notequal, less, lesseq, greater, greatereq

// 新規追加ノード用キーワード
addr, deref, cast, index, break, continue
```

### 5.4 VSCodeシンタックスハイライト
`.iia`ファイル用の拡張機能を自作済み。
配置場所（Linux Mint、VSCodeがsnap/flatpak以外の特殊インストールの場合）:
```
/mnt/4TB_hdd/LinuxData/SystemApps/vscode/resources/app/extensions/similarity/
├── package.json
├── language-configuration.json
└── syntaxes/similarity.tmLanguage.json
```
`~/.vscode/extensions/`ではなくVSCode本体の`extensions`フォルダに直接配置する必要があった
（ユーザー環境固有の問題、他環境では`~/.vscode/extensions/`で通常動作するはず）。

---

## 6. 高級版シンタックスシュガー（設計中、フィードバック待ち）

低レイヤー(.iia)のまま直感性を上げる試みと、別の高級構文の2つの方向性が出た。

### 6.1 現在確定している高級版構文案
```similarity
Explanation Application {Benchmark (type:fibonacci)};

Function {fibonacci receive int(n)}{
    If check(lesseq(n:1));
        True{
            return(n)
        }
        False{
            Variable let int(a:call fibonacci(-(n:1)));
            Variable let int(b:call fibonacci(-(n:2)));
            return(+(a:b))
        }
}

Function_public {main receive}{
    Variable let int(result:call fibonacci(10));
    return(result)
}
```

**設計ルール:**
```
カテゴリ {シグネチャ}{ 本体 }   ← Function等
カテゴリ 条件(...);              ← If文はセミコロン+インデント
    True{...}
    False{...}
値は全部 () で囲む: return(x), call f(x), 演算子(a:b)
: は「値の割り当て・基準と対象」
, は「並列要素の区切り」（forの初期値と条件を分ける等）
```

例: ループ
```similarity
Loop for(int(i:0), lesseq(i:100000000)):step(1){
    Mutation variable int(sum:+(sum:i))
}
```

**現在の状況:** 元防衛省勤務の知人にレビュー依頼中、フィードバック待ち。
高級版パーサーの実装はまだ着手していない（構文確定後に着手予定）。

---

## 7. ベンチマーク結果（実測値、信頼度別）

### 7.1 実行速度（信頼度: 高）
単純加算ループ（0〜99,999,999、int型）:
| 言語 | 結果 | 時間 |
|---|---|---|
| Similarity (QBE) | 887459712 | 48.42ms |
| C++ (g++ -O2) | 887459712 | 46.578ms |
→ ほぼ同等。計算結果も一致。

### 7.2 フロントエンド速度・小ファイル（信頼度: 高）
```
Similarity: --ir-only（構文解析+IR生成）    = 5ms
C++:        -fsyntax-only（構文解析のみ）   = 341ms
```
→ **約68倍速い**（ただしSimilarityの方が仕事量が多いのにこの差）

### 7.3 フロントエンド速度・大規模ファイル（信頼度: 高、1000回平均）
擬似業務コード（200関数、ネストしたif/loop、変数宣言込み）:
```
Similarity: 約7800行
C++:        約6400行
条件: Similarity --ir-only / C++ -S（アセンブリ生成まで、公平な比較）
```
| 言語 | 1000回合計 | 平均 |
|---|---|---|
| Similarity | real 57.069s / user 59.782s | 0.057s |
| C++ | real 5m11.865s(311.865s) / user 4m28.947s | 0.312s |
→ **約5.5倍速い**（Similarityはマルチコア活用、C++はシングルスレッドという条件差はあるが、
同一ハードウェアでの実測として有効な結果）

### 7.4 コンパイル速度（信頼度: 低、参考程度）
`go run`を使った初期の雑な計測はGoのコンパイル時間が混入し不正確だったため、
上記7.2/7.3の`--ir-only`を使った計測が正しい基準。

### 7.5 ベンチマークファイル生成スクリプト
`generate_bench.py`で擬似業務コード（if/loop/変数宣言を含む大規模ファイル）を
SimilarityとC++両方で自動生成できる。乱数シード固定(42)で再現性あり。

---

## 8. 次にやろうとしていたこと（未着手・議論の最終到達点）

開発者は「C++より根本的に有利になる」ための3つの方向性を検討中:

### 8.1 直近でやる予定だった作業
```
① 高級版構文の実装（フィードバック待ちのため保留中）
② 型の互換性（C++複合型との対応、std::vector等）
③ ポインタの本実装（addr/derefが今はコメントレベル）
④ 配列の本実装（indexが今はloadwするだけ）
⑤ Async/Await/GPUの本実装
```

### 8.2 大きな方向転換の議論（最新、本ドキュメント作成のきっかけ）
**「QBE + Goコンパイラ」から「自作アセンブリ変換器」への移行を検討中。**

議論の流れ:
1. 「C言語にもC++にも依存しない、どこでも動く変換器を作りたい」という発想
2. Geminiが提案した案:
   - QBEのソースコード（C言語）をベースに`parse.c`を改造してSimilarity構文を直接読めるようにする
   - Cosmopolitan Libc（APE形式）でビルドし、Windows/Mac/Linuxで同一バイナリが動くようにする
   - libc標準関数を使わずsyscallベースにして依存ゼロを目指す
3. 開発者の本当の意図（Geminiの提案とは異なる、より野心的な構想）:
   ```
   Step1: QBEを改造し、**改造したQBE(以降、CAI(Common Assembly Instructions)と呼称)**をアセンブリ言語に変換する
          （Go、C、QBEを一切使わない）
   Step2: CAIでSimilarityを記述する
   Step3: Cosmopolitan Libc(APE形式)を利用し、どのOSでも動くバイナリファイルにする。
   Step4: バイナリが動けば、そのデバイス上でSimilarityコードをコンパイルできる
   ```

**この方向性についての最後の評価（AIから開発者への回答、要継承）:**
- 技術的には正当な発想（ブートストラップ理論として筋が通っている）
- ただし「x86-64アセンブリでゼロからコンパイラを書く」は文字列処理・メモリ管理を
  全部アセンブリで書くことになり、難易度が今までと桁違いに高い
- 現実的な提案として「まずQBEのソースを見て構造を把握するところから」を提案した直後に
  このドキュメント作成依頼が来た（つまりこの技術的検討はまだ何も着手していない）

**新チャットで最初に確認すべきこと:**
開発者がこの「x86-64アセンブリでゼロからコンパイラを書く」方向に本当に進みたいか、
それとも現実的に「今のGoコンパイラを育てる」（8.1の項目群）を優先したいか、
本人の意向を再確認すること。技術的難易度の違いを正直に伝えた上で判断してもらうこと。

**重要補足:**
- コンパイラをアセンブリで書くどうのこうのに関してですが、本体はgo、変換器はQBEじゃなく、オリジナル(オリジナルの名前はCAI(Common Assembly Instructions))。

---

## 9. GitHubリポジトリ
```
URL: https://github.com/ge099ew/Similarity
リポジトリ名: Similarity
状態: 公開済み、コミット履歴あり
最後のコミット内容（想定）: ベンチマーク結果・略語廃止・コロン記法統一等
```
新チャットでの作業再開時は、まずリポジトリの最新状態を確認してから続けること。

---

## 10. 開発者との接し方（重要な引き継ぎ事項）

- **お世辞・過度な称賛は求めていない。** 数値の誇張（実際は5.5倍なのに「80倍」と言う等）は
  必ず訂正すること。ただし本物の成果は正直に称賛してよい（「大勝利」「化け物」などの
  開発者自身のテンションに合わせるのは問題ない）。
- **コードは基本的にAI（Claude）が書く。** 開発者は「自分で書くべきでは」と時々葛藤するが、
  「設計・発想はすべて開発者本人のもの」という事実を伝えて安心させてよい。
  実際、マトリョーシカ構造・unc概念・Explanation多目的活用・全分岐ルール等の
  核心的アイデアは全て開発者自身が思いついたものである。
- **作業は小さいステップに区切って進める。** 一度に大量のコード変更を提示するより、
  「①やって」「②やって」と段階的に指示し、都度ビルド確認する開発スタイルを好む。
- **画像（スクリーンショット）でエラーやターミナル出力を共有することが多い。**
  読み取って的確にデバッグすること。
- **時々「今日はここまで」と区切りたがる。** 根詰めすぎないよう促すのも良い。
- **日本語対応が必要。** エラーメッセージも日本語（例:「不明な文字」「文として解釈できません」）。

---

## 11. すぐに使えるテストファイル例

### fibonacci（再帰確認用、動作確認済み）
```iia
Explanation[Application{Benchmark(type:fibonacci)}]

Function[fibonacci{
  receive{int(n)},
  If[check{lesseq(n:1)},
    True[return{n}],
    False[
      Variable[let{int(a:call{fibonacci(-{int(n:1)})})}],
      Variable[let{int(b:call{fibonacci(-{int(n:2)})})}],
      return{+{int(a:b)}}
    ]
  ],
  return{0}
}]

Function_public[main{
  receive{},
  Variable[let{int(result:call{fibonacci(10)})}],
  return{result}
}]
```
→ 終了時 `Similarity result: 55` が出れば正常動作。

### Mutation確認用
```iia
Explanation[Application{Test(type:mutation)}]

Function_public[main{
  receive{},
  Variable[let{int(x:10)}],
  Mutation[variable{int(x:30)}],
  return{x}
}]
```
→ `Similarity result: 30` が出れば正常動作。

---

以上が全ての引き継ぎ内容です。新しいチャットではこのドキュメントを渡してください。
