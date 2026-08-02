#!/bin/bash
# Similarity 統合ベンチマーク
# Usage: bash benchmark/run_benchmark.sh [--repeat N] [--min-sec N]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
SIM="$ROOT_DIR/sim"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

REPEAT=20
MIN_SEC=3

while [[ $# -gt 0 ]]; do
  case $1 in
    --repeat)  REPEAT="$2"; shift 2 ;;
    --min-sec) MIN_SEC="$2"; shift 2 ;;
    *) shift ;;
  esac
done

TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
MD_OUT="$SCRIPT_DIR/results_$(date '+%Y%m%d_%H%M%S').md"
TMP="$(mktemp -d)"; trap "rm -rf $TMP" EXIT

echo -e "${BOLD}Similarity Benchmark${NC}  backend:${CYAN}CAI${NC}  repeat:${REPEAT}  min-sec:${MIN_SEC}"
echo ""

# ===== ビルド =====
echo -e "${YELLOW}[Build]${NC} sim..."
cd "$ROOT_DIR" && go build -o sim ./cmd/ 2>/dev/null
echo -e "${YELLOW}[Build]${NC} cai_conv..."
gcc -O2 -o cai_conv cai_converter/cai_converter.c 2>/dev/null

# ===== 事前チェック: 全iiaがCAIで動くか =====
echo ""
echo -e "${YELLOW}[Check]${NC} Similarity CAI動作確認..."
BENCHES_IIA=(
  "benchmark/fibonacci/bench_fib.iia"
  "benchmark/sum/bench_sum.iia"
  "benchmark/bubble_sort/bench_bubble.iia"
  "benchmark/stress/bench_call.iia"
  "benchmark/control/bench_nested_loop.iia"
  "benchmark/eratosthenes/bench_eratosthenes.iia"
  "benchmark/matrix/bench_matrix.iia"
  "benchmark/recursion/bench_ackermann.iia"
)

ALL_OK=1
for iia in "${BENCHES_IIA[@]}"; do
  full="$ROOT_DIR/$iia"
  if [ ! -f "$full" ]; then
    echo "  MISSING: $iia"
    ALL_OK=0
    continue
  fi
  err=$("$SIM" --cai "$full" 2>&1 | grep -E "^=== (Parser|Lexer|Type)|Link Error" | head -1 || true)
  if [ -n "$err" ]; then
    echo "  FAIL: $iia  ← $err"
    ALL_OK=0
  else
    out="${full%.iia}.out"
    r=$("$out" 2>&1 | grep -oE 'result: [0-9-]+' | head -1)
    echo "  OK  : $iia  $r"
  fi
done

if [ "$ALL_OK" = "0" ]; then
  echo ""
  echo "エラーが検出されました。ベンチマークを中断します。"
  exit 1
fi

echo ""

# ===== ベンチ定義: "name|iia|cpp|c|rs" =====
BENCHES=(
  "fibonacci(40)|benchmark/fibonacci/bench_fib.iia|benchmark/fibonacci/bench_fib.cpp|benchmark/fibonacci/bench_fib.c|benchmark/fibonacci/bench_fib.rs"
  "sum(0..1e8)|benchmark/sum/bench_sum.iia|benchmark/sum/bench_sum.cpp|benchmark/sum/bench_sum.c|benchmark/sum/bench_sum.rs"
  "bubble_sort(5000)|benchmark/bubble_sort/bench_bubble.iia|benchmark/bubble_sort/bench_bubble.cpp|benchmark/bubble_sort/bench_bubble.c|benchmark/bubble_sort/bench_bubble.rs"
  "stress_call(1M)|benchmark/stress/bench_call.iia|benchmark/stress/bench_call.cpp|benchmark/stress/bench_call.c|benchmark/stress/bench_call.rs"
  "nested_loop(1Kx1K)|benchmark/control/bench_nested_loop.iia|benchmark/control/bench_nested_loop.cpp|benchmark/control/bench_nested_loop.c|benchmark/control/bench_nested_loop.rs"
  "primes(10000)|benchmark/eratosthenes/bench_eratosthenes.iia|benchmark/eratosthenes/bench_eratosthenes.cpp|benchmark/eratosthenes/bench_eratosthenes.c|benchmark/eratosthenes/bench_eratosthenes.rs"
  "matrix(200^3)|benchmark/matrix/bench_matrix.iia|benchmark/matrix/bench_matrix.cpp|benchmark/matrix/bench_matrix.c|benchmark/matrix/bench_matrix.rs"
  "ackermann(3,7)|benchmark/recursion/bench_ackermann.iia|benchmark/recursion/bench_ackermann.cpp|benchmark/recursion/bench_ackermann.c|benchmark/recursion/bench_ackermann.rs"
)

# ===== 計測関数 =====
measure() {
  local bin="$1"
  local times=()
  local count=0
  local start_epoch
  start_epoch=$(date +%s)
  while true; do
    local raw t
    raw=$("$bin" 2>/dev/null) || true
    t=$(echo "$raw" | grep -oE '[0-9]+(\.[0-9]+)?ms' | head -1 | grep -oE '[0-9]+(\.[0-9]+)?' || echo "0")
    [ -z "$t" ] && t="0"
    times+=("$t")
    count=$((count+1))
    local now elapsed
    now=$(date +%s)
    elapsed=$(( now - start_epoch ))
    [ "$count" -ge "$REPEAT" ] && [ "$elapsed" -ge "$MIN_SEC" ] && break
  done
  python3 - "${times[@]}" << 'PYEOF'
import sys, math
nums = [float(x) for x in sys.argv[1:]]
n = len(nums)
avg = sum(nums)/n
mn = min(nums); mx = max(nums)
std = math.sqrt(sum((x-avg)**2 for x in nums)/n)
print(f"{avg:.2f} {mn:.2f} {mx:.2f} {std:.2f} {n}")
PYEOF
}

run_lang() {
  local label="$1" bin="$2"
  local avg mn mx std cnt
  read -r avg mn mx std cnt <<< "$(measure "$bin")"
  printf "  %-14s avg=%sms  min=%s  max=%s  sigma=%s  (n=%s)\n" "$label" "$avg" "$mn" "$mx" "$std" "$cnt" >&2
  printf "%s" "$avg"
}

compile_cpp() { local src="$ROOT_DIR/$1" out="$TMP/$(basename ${src%.*})_cpp"; [ -f "$src" ] || return 1; g++ -O0 -o "$out" "$src" 2>/dev/null && echo "$out"; }
compile_c()   { local src="$ROOT_DIR/$1" out="$TMP/$(basename ${src%.*})_c";   [ -f "$src" ] || return 1; gcc -O0 -o "$out" "$src" 2>/dev/null && echo "$out"; }
compile_rust(){ local src="$ROOT_DIR/$1" out="$TMP/$(basename ${src%.*})_rs";  [ -f "$src" ] || return 1; rustc -C opt-level=0 -o "$out" "$src" 2>/dev/null && echo "$out"; }

# ===== 実行 =====
declare -A SIM_AVG CPP_AVG C_AVG RS_AVG
NAMES=()

for bench in "${BENCHES[@]}"; do
  IFS='|' read -r name iia cpp_path c_path rs_path <<< "$bench"
  NAMES+=("$name")
  echo -e "${BOLD}── $name ──${NC}"

  out_path="${ROOT_DIR}/${iia%.iia}.out"
  SIM_AVG["$name"]=$(run_lang "Similarity(CAI)" "$out_path")

  if bin=$(compile_cpp "$cpp_path" 2>/dev/null); then
    CPP_AVG["$name"]=$(run_lang "C++(-O0)" "$bin")
  else CPP_AVG["$name"]="N/A"; echo "  C++(-O0)       SKIP"; fi

  if bin=$(compile_c "$c_path" 2>/dev/null); then
    C_AVG["$name"]=$(run_lang "C(-O0)" "$bin")
  else C_AVG["$name"]="N/A"; echo "  C(-O0)         SKIP"; fi

  if bin=$(compile_rust "$rs_path" 2>/dev/null); then
    RS_AVG["$name"]=$(run_lang "Rust(-O0)" "$bin")
  else RS_AVG["$name"]="N/A"; echo "  Rust(-O0)      SKIP"; fi

  echo ""
done

# ===== サマリー =====
echo -e "${BOLD}══════════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}  Summary${NC}"
echo -e "${BOLD}══════════════════════════════════════════════════════════════${NC}"
printf "%-22s %12s %12s %12s %12s\n" "Benchmark" "Similarity" "C++(-O0)" "C(-O0)" "Rust(-O0)"
printf "%-22s %12s %12s %12s %12s\n" "──────────────────────" "────────────" "────────────" "────────────" "────────────"

{
echo "# Similarity Benchmark Results"
echo ""
echo "Generated: ${TIMESTAMP}  |  Backend: CAI  |  Repeat: ${REPEAT} / min: ${MIN_SEC}s"
echo ""
echo "| Benchmark | Similarity(CAI) | C++(-O0) | C(-O0) | Rust(-O0) | Best |"
echo "|-----------|----------------:|---------:|-------:|----------:|------|"
} > "$MD_OUT"

for name in "${NAMES[@]}"; do
  s="${SIM_AVG[$name]:-N/A}"
  cpp="${CPP_AVG[$name]:-N/A}"
  c="${C_AVG[$name]:-N/A}"
  rs="${RS_AVG[$name]:-N/A}"

  printf "%-22s %11sms %11sms %11sms %11sms\n" "$name" "$s" "$cpp" "$c" "$rs"

  best="N/A"
  if [ "$s" != "N/A" ] && [ "$cpp" != "N/A" ] && [ "$c" != "N/A" ] && [ "$rs" != "N/A" ]; then
    best=$(python3 -c "
vals={'Similarity':$s,'C++':$cpp,'C':$c,'Rust':$rs}
b=min(vals,key=vals.get)
print(f'{b}({vals[b]:.2f}ms)')
")
  fi
  echo "| $name | ${s}ms | ${cpp}ms | ${c}ms | ${rs}ms | $best |" >> "$MD_OUT"
done

echo ""
echo -e "Markdown: ${CYAN}${MD_OUT}${NC}"
