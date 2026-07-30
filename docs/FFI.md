# FFI（外部関数インターフェース）

SimilarityはC/C++ライブラリを直接呼び出せます。

---

## C関数の宣言

```iia
Extern[C{
  lib{"SDL2"},
  draw{receive{int(x), int(y)}, return{}}
}]
```

---

## 呼び出し

```iia
call{draw(10, 20)}
```

---

## ABI

SimilarityはSystem V AMD64 ABI（Linux標準）に準拠しています。
C/C++との構造体互換・関数呼び出し互換を保証します。
