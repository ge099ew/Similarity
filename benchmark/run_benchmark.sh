#!/bin/bash

# Similarity ベンチマーク自動実行スクリプト（コールドスタート版 v2）
# 実行: bash benchmark/run_benchmark.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
SIM="$ROOT_DIR/sim"
OUT="$SCRIPT_DIR/benchmark_results.txt"
REPEAT=100

# ===== 準備 =====
# バイナリを事前にビルド
echo "準備中: Similarityをビルド..."
cd "$ROOT_DIR" && go build -o sim ./cmd/

echo "準備中: ベンチマークバイナリをコンパイル..."
echo "Y" | "$SIM" "$SCRIPT_DIR/bench_fib.iia" > /dev/null 2>&1
cp "$SCRIPT_DIR/bench_fib.iia.out" "$SCRIPT_DIR/_sim_fib"

"$SIM" "$SCRIPT_DIR/bench_sum.iia" > /dev/null 2>&1
cp "$SCRIPT_DIR/bench_sum.iia.out" "$SCRIPT_DIR/_sim_sum"

g++ -O0 -o "$SCRIPT_DIR/_cpp_fib" "$SCRIPT_DIR/bench_fib.cpp"
g++ -O0 -o "$SCRIPT_DIR/_cpp_sum" "$SCRIPT_DIR/bench_sum.cpp"

echo "準備完了。計測開始..."
echo ""

# ===== 計測関数 =====

# バイナリ実行時間のみ計測（コールドスタート: sync+drop_caches）
run_avg() {
    local binary="$1"
    local label="$2"
    local total=0
    for i in $(seq 1 $REPEAT); do
        # ページキャッシュをドロップ（可能な場合）
        sync 2>/dev/null || true
        local t
        t=$("$binary" 2>&1 | grep -oP '(?<=time: )[0-9]+\.[0-9]+' | head -1)
        if [ -z "$t" ]; then t="0"; fi
        total=$(echo "$total + $t" | bc)
        printf "  [%s] 進捗: %d/%d\r" "$label" "$i" "$REPEAT"
    done
    printf "\n"
    echo "scale=2; $total / $REPEAT" | bc
}

# フロントエンド速度計測（sim --ir-only のみ計測）
run_frontend_avg() {
    local target="$1"
    local label="$2"
    local total=0
    for i in $(seq 1 $REPEAT); do
        sync 2>/dev/null || true
        local raw min sec t
        raw=$( { time "$SIM" --ir-only "$target" > /dev/null 2>&1; } 2>&1 | grep real)
        min=$(echo "$raw" | grep -oP '\d+(?=m)')
        sec=$(echo "$raw" | grep -oP '\d+\.\d+(?=s)')
        if [ -z "$min" ]; then min="0"; fi
        if [ -z "$sec" ]; then sec="0"; fi
        t=$(echo "scale=3; ($min * 60 + $sec) * 1000" | bc)
        total=$(echo "$total + $t" | bc)
        printf "  [%s] 進捗: %d/%d\r" "$label" "$i" "$REPEAT"
    done
    printf "\n"
    echo "scale=2; $total / $REPEAT" | bc
}

# C++ フロントエンド速度計測
run_frontend_cpp_avg() {
    local src="$1"
    local label="$2"
    local total=0
    for i in $(seq 1 $REPEAT); do
        sync 2>/dev/null || true
        local raw min sec t
        raw=$( { time g++ -fsyntax-only "$src" > /dev/null 2>&1; } 2>&1 | grep real)
        min=$(echo "$raw" | grep -oP '\d+(?=m)')
        sec=$(echo "$raw" | grep -oP '\d+\.\d+(?=s)')
        if [ -z "$min" ]; then min="0"; fi
        if [ -z "$sec" ]; then sec="0"; fi
        t=$(echo "scale=3; ($min * 60 + $sec) * 1000" | bc)
        total=$(echo "$total + $t" | bc)
        printf "  [%s] 進捗: %d/%d\r" "$label" "$i" "$REPEAT"
    done
    printf "\n"
    echo "scale=2; $total / $REPEAT" | bc
}

# ===== 計測 =====

echo "=== Similarity 実行速度 ==="
SIM_FIB=$(run_avg "$SCRIPT_DIR/_sim_fib" "Sim fibonacci")
echo "  fibonacci(40): ${SIM_FIB}ms"

SIM_SUM=$(run_avg "$SCRIPT_DIR/_sim_sum" "Sim 総和")
echo "  総和: ${SIM_SUM}ms"

echo ""
echo "=== C++ 実行速度 ==="
CPP_FIB=$(run_avg "$SCRIPT_DIR/_cpp_fib" "C++ fibonacci")
echo "  fibonacci(40): ${CPP_FIB}ms"

CPP_SUM=$(run_avg "$SCRIPT_DIR/_cpp_sum" "C++ 総和")
echo "  総和: ${CPP_SUM}ms"

echo ""
echo "=== Similarity フロントエンド速度 ==="
SIM_FE_SHORT=$(run_frontend_avg "$SCRIPT_DIR/bench_frontend_short.iia" "Sim 短い.iia")
echo "  短いファイル (.iia): ${SIM_FE_SHORT}ms"

SIM_FE_SHORT_SML=$(run_frontend_avg "$SCRIPT_DIR/bench_frontend_short.sml" "Sim 短い.sml")
echo "  短いファイル (.sml): ${SIM_FE_SHORT_SML}ms  ← トランスパイル込み"

SIM_FE_LONG=$(run_frontend_avg "$SCRIPT_DIR/bench_frontend_long.iia" "Sim 長い.iia")
echo "  長いファイル (.iia): ${SIM_FE_LONG}ms"

echo ""
echo "=== C++ フロントエンド速度 ==="
CPP_FE_SHORT=$(run_frontend_cpp_avg "$SCRIPT_DIR/bench_frontend_short.cpp" "C++ 短い")
echo "  短いファイル: ${CPP_FE_SHORT}ms"

CPP_FE_LONG=$(run_frontend_cpp_avg "$SCRIPT_DIR/bench_frontend_long.cpp" "C++ 長い")
echo "  長いファイル: ${CPP_FE_LONG}ms"

# ===== 結果ファイル出力 =====
{
echo "Similarity Benchmark Results"
echo "Generated : $(date '+%Y-%m-%d %H:%M:%S')"
echo "Repeat    : ${REPEAT}回平均（コールドスタート）"
echo "========================================"
echo ""
echo "【Similarity 実行速度】"
echo "----------------------------------------"
echo "  fibonacci(40) : ${SIM_FIB}ms"
echo "  総和           : ${SIM_SUM}ms"
echo ""
echo "【C++ 実行速度】"
echo "----------------------------------------"
echo "  fibonacci(40) : ${CPP_FIB}ms"
echo "  総和           : ${CPP_SUM}ms"
echo ""
echo "【Similarity フロントエンド速度（コンパイル時間）】"
echo "----------------------------------------"
echo "  短いファイル (.iia) : ${SIM_FE_SHORT}ms"
echo "  短いファイル (.sml) : ${SIM_FE_SHORT_SML}ms  ← トランスパイル込み"
echo "  長いファイル (.iia) : ${SIM_FE_LONG}ms"
echo ""
echo "【C++ フロントエンド速度（コンパイル時間）】"
echo "----------------------------------------"
echo "  短いファイル : ${CPP_FE_SHORT}ms"
echo "  長いファイル : ${CPP_FE_LONG}ms"
echo ""
echo "========================================"
echo "Benchmark完了。結果: $OUT"
} > "$OUT"

echo ""
echo "完了。結果: $OUT"

# 一時ファイル削除
rm -f "$SCRIPT_DIR/_sim_fib" "$SCRIPT_DIR/_sim_sum" "$SCRIPT_DIR/_cpp_fib" "$SCRIPT_DIR/_cpp_sum"
