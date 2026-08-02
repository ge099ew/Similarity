# Similarity 言語リファレンス

Similarityはシステムプログラミングを目的として設計された言語です。
GCなし、コンパイラは推測しない、unsafe操作は明示必須という哲学のもとに設計されています。

---

## ファイル形式

| 拡張子 | 説明 |
|---|---|
| `.iia` | 低レイヤー構文（本来の形式） |
| `.sml` | シュガーシンタックス（`.iia`にトランスパイル） |
| `.cai` | CAI IR（中間表現、テキスト形式） |

---

## 基本パターン

```
カテゴリ[操作{引数}]
```

---

## 変数

```iia
Variable[let{int(x:10)}]          # ミュータブル整数変数
Variable[let{int(x:-5)}]          # 負数
Variable[unclet{float(PI:3.14)}]  # イミュータブル（再代入不可）
Mutation[variable{int(x:30)}]     # 再代入
```

---

## 型

| 型 | 説明 |
|---|---|
| `int` | 32bit整数 |
| `float` | 32bit浮動小数点 |
| `String` | 文字列 |
| `bool` | 真偽値 |
| `Array_int` | 整数配列 |
| `Array_float` | 浮動小数点配列 |

---

## 演算子

```iia
+{int(a,b)}   # 加算
-{int(a,b)}   # 減算
*{int(a,b)}   # 乗算
/{int(a,b)}   # 除算
++{i}         # インクリメント（i += 1）
--{i}         # デクリメント（i -= 1）
```

---

## 比較

```iia
equal(a:b)     # a == b
notequal(a:b)  # a != b
less(a:b)      # a < b
lesseq(a:b)    # a <= b
greater(a:b)   # a > b
greatereq(a:b) # a >= b
```

---

## 制御フロー

```iia
If[check{less(hp:0)},
  True{
    # 条件が真のとき
  },
  False{
    # 条件が偽のとき
  }
]

Loop[
  check{less(i:10)},
  for{
    # ループ本体
    ++{i}
  }
]

break{}
continue{}
```

---

## 関数

```iia
Function[name{
  receive{int(x), int(y)},
  ...
  return(x)
}]

Function_public[name{...}]   # 公開関数

call{name(arg1, arg2)}
```

---

## 配列

```iia
Variable[let{Array_int(arr:10)}]       # 10要素のint配列
Mutation[array{int(arr:0:42)}]         # arr[0] = 42
Variable[let{int(val:index{arr(0)})}]  # val = arr[0]
```

---

## ポインタ

```iia
Variable[let{int(ptr:addr{x})}]        # アドレス取得
Mem[risk{
  Variable[let{int(val:deref{ptr})}]   # 参照外し（risk必須）
}]
```

---

## 構造体

```iia
Variable[struct{User:String(name), int(age)}]
Variable[let{user:User(name:"John", age:25)}]
```

---

## 非同期

```iia
Async[{
  share(x),                          # 共有変数の明示
  Mutation[variable{int(x:30)}]
}]
Await[task]
```

---

## エラーハンドリング

```iia
Error[try{...},
  Ok[...],
  Err[type{FileNotFound}, msg{"file not found"}]
]

Fatal[type{OutOfMemory}, msg{"回復不能"}]
```

---

## モジュール

```iia
Import[math{}]
Import[io{}]

Extern[C{lib{"SDL2"}, draw{receive{int(x)}, return{}}}]
```
