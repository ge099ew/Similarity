# Cell（project.cel）

CellはSimilarityのパッケージ管理設定ファイルです。

---

## フォーマット

```
name: MyProject
version: 0.1.0
dependencies:
  - math
  - io
  - string
```

---

## 検証機能

- 未知のキー検出（行番号付きエラー）
- バージョン形式チェック（`x.y.z`）
- 依存関係の重複検出
- `Import[xxx{}]` との整合性チェック

---

## エラー例

```
project.cel:8: Unknown key 'naem'
project.cel:3: Invalid version format '0.1' (expected x.y.z)
```
