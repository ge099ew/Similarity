#!/bin/bash

# Similarity CAIバックエンド vs C++ ベンチマーク
# 実行: bash benchmark/run_benchmark_cai.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
SIM="$ROOT_DIR/sim"
OUT="$SCRIPT_DIR/benchmark_results_cai.txt"
REPEAT=100

echo "準備中: Similarityをビルド..."
cd "$ROOT_DIR" && go build -o sim ./cmd/

echo "準備中: CAIバイナリをコンパイル..."
"$SIM" --cai "$SCRIPT_DIR/bench_fib.iia" > /dev/null 2>&1
cp "$SCRIPT_DIR/bench_fib.iia.out" "$SCRIPT_DIR/_cai_fib"

"$SIM" --cai "$SCRIPT_DIR/bench_sum.iia" > /dev/null 2>&1
cp "$SCRIPT_DIR/bench_sum.iia.out" "$SCRIPT_DIR/_cai_sum"

echo "準備中: C++バイナリをコンパイル..."
g++ -O0 -o "$SCRIPT_DIR/_cpp_fib" "$SCRIPT_DIR/bench_fib.cpp"
g++ -O0 -o "$SCRIPT_DIR/_cpp_sum" "$SCRIPT_DIR/bench_sum.cpp"

echo "準備完了。計測開始..."
echo ""

# 実行時間計測（バイナリ内のtimeを使う）
run_avg() {
    local binary="$1"
    local label="$2"
    local total=0
    for i in $(seq 1 $REPEAT); do
        local t
        t=$("$binary" 2>&1 | grep -oP '(?<=time: )[0-9]+' | head -1)
        if [ -z "$t" ]; then t="0"; fi
        total=$(echo "$total + $t" | bc)
        printf "  [%s] 進捗: %d/%d\r" "$label" "$i" "$REPEAT"
    done
    printf "\n"
    echo "scale=2; $total / $REPEAT" | bc
}

echo "=== Similarity (CAI) 実行速度 ==="
CAI_FIB=$(run_avg "$SCRIPT_DIR/_cai_fib" "CAI fibonacci")
echo "  fibonacci(40) : ${CAI_FIB}ms"

CAI_SUM=$(run_avg "$SCRIPT_DIR/_cai_sum" "CAI 総和")
echo "  総和           : ${CAI_SUM}ms"

echo ""
echo "=== C++ 実行速度 ==="
CPP_FIB=$(run_avg "$SCRIPT_DIR/_cpp_fib" "C++ fibonacci")
echo "  fibonacci(40) : ${CPP_FIB}ms"

CPP_SUM=$(run_avg "$SCRIPT_DIR/_cpp_sum" "C++ 総和")
echo "  総和           : ${CPP_SUM}ms"

# 結果ファイル出力
{
echo "Similarity CAI vs C++ Benchmark Results"
echo "Generated : $(date '+%Y-%m-%d %H:%M:%S')"
echo "Repeat    : ${REPEAT}回平均"
echo "========================================"
echo ""
echo "【Similarity (CAI) 実行速度】"
echo "----------------------------------------"
echo "  fibonacci(40) : ${CAI_FIB}ms"
echo "  総和           : ${CAI_SUM}ms"
echo ""
echo "【C++ (-O0) 実行速度】"
echo "----------------------------------------"
echo "  fibonacci(40) : ${CPP_FIB}ms"
echo "  総和           : ${CPP_SUM}ms"
echo ""

# 勝敗判定
echo "【勝敗】"
echo "----------------------------------------"
FIB_WIN=$(echo "$CAI_FIB < $CPP_FIB" | bc)
SUM_WIN=$(echo "$CAI_SUM < $CPP_SUM" | bc)
if [ "$FIB_WIN" = "1" ]; then
    RATIO=$(echo "scale=2; $CPP_FIB / $CAI_FIB" | bc)
    echo "  fibonacci : Similarity 勝利 (${RATIO}倍速い)"
else
    RATIO=$(echo "scale=2; $CAI_FIB / $CPP_FIB" | bc)
    echo "  fibonacci : C++ 勝利 (${RATIO}倍速い)"
fi
if [ "$SUM_WIN" = "1" ]; then
    RATIO=$(echo "scale=2; $CPP_SUM / $CAI_SUM" | bc)
    echo "  総和       : Similarity 勝利 (${RATIO}倍速い)"
else
    RATIO=$(echo "scale=2; $CAI_SUM / $CPP_SUM" | bc)
    echo "  総和       : C++ 勝利 (${RATIO}倍速い)"
fi
echo ""
echo "========================================"
echo "Benchmark完了。結果: $OUT"
} > "$OUT"

cat "$OUT"

# 一時ファイル削除
rm -f "$SCRIPT_DIR/_cai_fib" "$SCRIPT_DIR/_cai_sum" "$SCRIPT_DIR/_cpp_fib" "$SCRIPT_DIR/_cpp_sum"
