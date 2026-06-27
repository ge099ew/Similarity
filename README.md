# Similarity vs C++ 対照実験

## パイプライン

```
.iia → QBE IR (.ssa) → qbe → .s → gcc → バイナリ
```

QBEがインストールされていれば自動でQBEを使います。
なければCにフォールバックします。

---

## 実行手順

### ① Similarityをコンパイル

```bash
cd similarity  # プロジェクトルート
go run ./cmd/ benchmark/bench_sim.iia
```

生成されるファイル：
```
benchmark/bench_sim.iia.ssa         ← QBE IR
benchmark/bench_sim.iia.s           ← アセンブリ（QBE使用時）
benchmark/bench_sim.iia_wrapper.c   ← タイマーラッパー
benchmark/bench_sim.iia.out         ← バイナリ
```

### ② C++をコンパイル

```bash
g++ -O2 -o benchmark/bench_cpp benchmark/bench_cpp.cpp
```

### ③ 実行して比較

```bash
./benchmark/bench_sim.iia.out
./benchmark/bench_cpp
```

---

## コンパイル速度も測る（こちらが今一番意味ある指標）

```bash
time go run ./cmd/ benchmark/bench_sim.iia
time g++ -O2 -o /tmp/bench benchmark/bench_cpp.cpp
```

---

## 今の正直な限界

今のSimilarityはCを経由せずQBE IRを直接出してるので、
実行速度の比較はかなり公平に近いですが、最適化はQBEの最適化レベルに依存します。
「Similarityの設計がビルドを速くする」の証明は**コンパイル速度**の比較が本筋です。
