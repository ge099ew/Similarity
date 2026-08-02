#!/bin/bash
# Similarity 統合ベンチマーク
# Usage: bash benchmark/run_benchmark.sh [--repeat N] [--min-sec N] [--no-cai]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
SIM="$ROOT_DIR/sim"
CAI_CONV="$ROOT_DIR/cai_conv"

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

REPEAT=20
MIN_SEC=3
USE_CAI=1

while [[ $# -gt 0 ]]; do
  case $1 in
    --repeat)  REPEAT="$2";  shift 2 ;;
    --min-sec) MIN_SEC="$2"; shift 2 ;;
    --no-cai)  USE_CAI=0;    shift   ;;
    *)         shift ;;
  esac
done

SIM_FLAGS="--cai"
BACKEND_LABEL="CAI"
[ "$USE_CAI" = "0" ] && { SIM_FLAGS=""; BACKEND_LABEL="C-fallback"; }

TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
MD_OUT="$SCRIPT_DIR/results_$(date '+%Y%m%d_%H%M%S').md"
TMP="$(mktemp -d)"; trap "rm -rf $TMP" EXIT

echo -e "${BOLD}Similarity Benchmark${NC}  backend:${CYAN}${BACKEND_LABEL}${NC}  repeat:${REPEAT}  min-sec:${MIN_SEC}"
echo ""

# ===== ビルド =====
echo -e "${YELLOW}[Build]${NC} sim..."
cd "$ROOT_DIR" && go build -o sim ./cmd/ 2>/dev/null
if [ "$USE_CAI" = "1" ]; then
  echo -e "${YELLOW}[Build]${NC} cai_conv..."
  gcc -O2 -o cai_conv cai_converter/cai_converter.c 2>/dev/null
fi

# ===== ベンチ定義: "name|iia|cpp|c|rs" =====
BENCHES=(
  "fibonacci(40)|benchmark/bench_fib.iia|benchmark/bench_fib.cpp|benchmark/bench_fib.c|benchmark/bench_fib.rs"
  "sum(0..1e8)|benchmark/bench_sum.iia|benchmark/bench_sum.cpp|benchmark/bench_sum.c|benchmark/bench_sum.rs"
  "bubble_sort(5000)|benchmark/bench_bubble.iia|benchmark/bench_bubble.cpp|benchmark/bench_bubble.c|benchmark/bench_bubble.rs"
  "stress_call(1M)|benchmark/bench_call.iia|benchmark/bench_call.cpp|benchmark/bench_call.c|benchmark/bench_call.rs"
  "nested_loop(1Kx1K)|benchmark/bench_nested_loop.iia|benchmark/bench_nested_loop.cpp|benchmark/bench_nested_loop.c|benchmark/bench_nested_loop.rs"
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
    local now=$(date +%s)
    local elapsed=$(( now - start_epoch ))
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

compile_sim() {
  local iia="$ROOT_DIR/$1"
  [ -f "$iia" ] || return 1
  "$SIM" $SIM_FLAGS "$iia" >/dev/null 2>&1
}

compile_cpp() {
  local src="$ROOT_DIR/$1"
  local out="$TMP/$(basename ${src%.*})_cpp"
  [ -f "$src" ] || return 1
  g++ -O0 -o "$out" "$src" 2>/dev/null && echo "$out"
}

compile_c() {
  local src="$ROOT_DIR/$1"
  local out="$TMP/$(basename ${src%.*})_c"
  [ -f "$src" ] || return 1
  gcc -O0 -o "$out" "$src" 2>/dev/null && echo "$out"
}

compile_rust() {
  local src="$ROOT_DIR/$1"
  local out="$TMP/$(basename ${src%.*})_rs"
  [ -f "$src" ] || return 1
  rustc -C opt-level=0 -o "$out" "$src" 2>/dev/null && echo "$out"
}

# ===== 実行 =====
declare -A SIM_AVG CPP_AVG C_AVG RS_AVG
NAMES=()

run_lang() {
  local label="$1" bin="$2"
  local avg mn mx std cnt
  read -r avg mn mx std cnt <<< "$(measure "$bin")"
  printf "  %-14s avg=%sms  min=%s  max=%s  sigma=%s  (n=%s)\n" "$label" "$avg" "$mn" "$mx" "$std" "$cnt" >&2
  printf "%s" "$avg"
}

for bench in "${BENCHES[@]}"; do
  IFS='|' read -r name iia cpp_path c_path rs_path <<< "$bench"
  NAMES+=("$name")
  echo -e "${BOLD}── $name ──${NC}"

  # Similarity
  if compile_sim "$iia"; then
    out_path="${ROOT_DIR}/${iia%.iia}.out"
    SIM_AVG["$name"]=$(run_lang "Similarity" "$out_path" "$name")
  else
    SIM_AVG["$name"]="N/A"; echo "  Similarity   FAIL"
  fi

  # C++
  if bin=$(compile_cpp "$cpp_path"); then
    CPP_AVG["$name"]=$(run_lang "C++ (-O0)" "$bin" "$name")
  else
    CPP_AVG["$name"]="N/A"; echo "  C++ (-O0)    FAIL"
  fi

  # C
  if bin=$(compile_c "$c_path"); then
    C_AVG["$name"]=$(run_lang "C   (-O0)" "$bin" "$name")
  else
    C_AVG["$name"]="N/A"; echo "  C   (-O0)    FAIL"
  fi

  # Rust
  if bin=$(compile_rust "$rs_path"); then
    RS_AVG["$name"]=$(run_lang "Rust(-O0)" "$bin" "$name")
  else
    RS_AVG["$name"]="N/A"; echo "  Rust(-O0)    FAIL"
  fi

  echo ""
done

# ===== サマリー =====
echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}  Summary${NC}"
echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
printf "%-22s %10s %10s %10s %10s\n" "Benchmark" "Similarity" "C++(-O0)" "C(-O0)" "Rust(-O0)"
printf "%-22s %10s %10s %10s %10s\n" "──────────────────────" "──────────" "──────────" "──────────" "──────────"

{
echo "# Similarity Benchmark Results"
echo ""
echo "Generated: ${TIMESTAMP}  |  Backend: ${BACKEND_LABEL}  |  Repeat: ${REPEAT} / min: ${MIN_SEC}s"
echo ""
echo "| Benchmark | Similarity | C++ (-O0) | C (-O0) | Rust (-O0) | Best |"
echo "|-----------|----------:|----------:|--------:|-----------:|------|"
} > "$MD_OUT"

for name in "${NAMES[@]}"; do
  s="${SIM_AVG[$name]:-N/A}"
  cpp="${CPP_AVG[$name]:-N/A}"
  c="${C_AVG[$name]:-N/A}"
  rs="${RS_AVG[$name]:-N/A}"

  printf "%-22s %9sms %9sms %9sms %9sms\n" "$name" "$s" "$cpp" "$c" "$rs"

  # Markdown: 最速を判定
  best="N/A"
  if [ "$s" != "N/A" ] && [ "$cpp" != "N/A" ] && [ "$c" != "N/A" ] && [ "$rs" != "N/A" ]; then
    best=$(python3 -c "
vals={'Similarity':$s,'C++':$cpp,'C':$c,'Rust':$rs}
b=min(vals,key=vals.get)
print(f'{b} ({vals[b]:.2f}ms)')
")
  fi
  echo "| $name | ${s}ms | ${cpp}ms | ${c}ms | ${rs}ms | $best |" >> "$MD_OUT"
done

echo ""
echo -e "Markdown: ${CYAN}${MD_OUT}${NC}"
