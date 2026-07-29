#!/bin/bash
# CAI Converter - ELF検証・命令テストスクリプト
# Usage: ./test_cai.sh <cai_conv_path> [binary]
# 例:    ./test_cai.sh ./cai_conv

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass=0
fail=0
skip=0

CAI_CONV="${1:-./cai_conv}"
TMPDIR_WORK="$(mktemp -d)"
trap "rm -rf $TMPDIR_WORK" EXIT

check() {
    local desc="$1"; shift
    if "$@" > /dev/null 2>&1; then
        echo -e "${GREEN}[PASS]${NC} $desc"
        pass=$((pass+1))
    else
        echo -e "${RED}[FAIL]${NC} $desc"
        fail=$((fail+1))
    fi
}

check_output() {
    local desc expected
    desc="$1"; expected="$2"; shift 2
    local actual
    actual=$("$@" 2>&1) || true
    if echo "$actual" | grep -q "$expected"; then
        echo -e "${GREEN}[PASS]${NC} $desc"
        pass=$((pass+1))
    else
        echo -e "${RED}[FAIL]${NC} $desc (got: $(echo $actual | head -c 80))"
        fail=$((fail+1))
    fi
}

check_result() {
    local desc expected bin
    desc="$1"; expected="$2"; bin="$3"
    local actual
    actual=$("$bin" 2>/dev/null | grep "result:" | grep -o '[0-9-]*' | head -1) || true
    if [ "$actual" = "$expected" ]; then
        echo -e "${GREEN}[PASS]${NC} $desc (result=$actual)"
        pass=$((pass+1))
    else
        echo -e "${RED}[FAIL]${NC} $desc (expected=$expected, got=$actual)"
        fail=$((fail+1))
    fi
}

compile_cai() {
    local src="$1" out="$2"
    "$CAI_CONV" "$src" "$out" > /dev/null 2>&1
}

echo "========================================"
echo " CAI Converter テストスイート"
echo " cai_conv: $CAI_CONV"
echo "========================================"
echo ""

# ===== ELF検証 =====
echo "--- ELF・PIE検証 ---"

# 基本テストバイナリを生成
cat > "$TMPDIR_WORK/basic.cai" << 'CAIEOF'
export func $main
  alloc %x.ptr 4
  store %x.ptr 42
  load  %r %x.ptr
  ret   %r
endfunc
CAIEOF
compile_cai "$TMPDIR_WORK/basic.cai" "$TMPDIR_WORK/basic"

check_output "ELFマジック番号"        "ELF"                    readelf -h "$TMPDIR_WORK/basic"
check_output "ET_DYN（静的PIE）"       "DYN"                    readelf -h "$TMPDIR_WORK/basic"
check_output "x86_64アーキテクチャ"   "X86-64\|x86-64"         readelf -h "$TMPDIR_WORK/basic"
check_output "PT_LOAD存在"             "LOAD"                   readelf -l "$TMPDIR_WORK/basic"
check_output "PT_GNU_STACK（NX）"      "GNU_STACK"              readelf -l "$TMPDIR_WORK/basic"
check_output "GNU_STACKフラグRWのみ"   "RW "                    readelf -l "$TMPDIR_WORK/basic"
check_output "PT_DYNAMIC存在"          "DYNAMIC"                readelf -l "$TMPDIR_WORK/basic"
check_output "PT_PHDR存在"             "PHDR"                   readelf -l "$TMPDIR_WORK/basic"
check_output "セクション.text存在"     "\.text"                 readelf -S "$TMPDIR_WORK/basic"
check_output "セクション.dynamic存在"  "\.dynamic"              readelf -S "$TMPDIR_WORK/basic"
check         "実行権限あり"            test -x "$TMPDIR_WORK/basic"
check_result  "基本実行（result=42）"  "42" "$TMPDIR_WORK/basic"

echo ""
echo "--- i32演算 ---"

cat > "$TMPDIR_WORK/i32.cai" << 'CAIEOF'
export func $main
  add  %r1 100 23
  sub  %r2 %r1 23
  mul  %r3 %r2 2
  div  %r4 %r3 4
  ret  %r4
endfunc
CAIEOF
compile_cai "$TMPDIR_WORK/i32.cai" "$TMPDIR_WORK/i32"
check_result "add/sub/mul/div (result=50)" "50" "$TMPDIR_WORK/i32"

cat > "$TMPDIR_WORK/cmp32.cai" << 'CAIEOF'
export func $main
  clt %r1 3 5
  cgt %r2 5 3
  ceq %r3 4 4
  cne %r4 3 4
  add %r5 %r1 %r2
  add %r6 %r5 %r3
  add %ret %r6 %r4
  ret %ret
endfunc
CAIEOF
compile_cai "$TMPDIR_WORK/cmp32.cai" "$TMPDIR_WORK/cmp32"
check_result "i32比較 clt/cgt/ceq/cne (result=4)" "4" "$TMPDIR_WORK/cmp32"

echo ""
echo "--- i64演算 ---"

cat > "$TMPDIR_WORK/i64.cai" << 'CAIEOF'
export func $main
  alloc  %a.ptr 8
  alloc  %b.ptr 8
  storep %a.ptr 1000000
  storep %b.ptr 1000000
  loadp2 %a %a.ptr
  loadp2 %b %b.ptr
  mul64  %r %a %b
  cgt64  %c %r 999999999
  ret    %c
endfunc
CAIEOF
compile_cai "$TMPDIR_WORK/i64.cai" "$TMPDIR_WORK/i64"
check_result "i64 mul64+cgt64: 1e6*1e6>999999999 (result=1)" "1" "$TMPDIR_WORK/i64"

cat > "$TMPDIR_WORK/i64cmp.cai" << 'CAIEOF'
export func $main
  alloc  %a.ptr 8
  storep %a.ptr 9999999999
  loadp2 %a %a.ptr
  cgt64  %r %a 9999999998
  ret    %r
endfunc
CAIEOF
compile_cai "$TMPDIR_WORK/i64cmp.cai" "$TMPDIR_WORK/i64cmp"
check_result "i64 cgt64 (result=1)" "1" "$TMPDIR_WORK/i64cmp"

echo ""
echo "--- f32演算（SSE2）---"

cat > "$TMPDIR_WORK/f32.cai" << 'CAIEOF'
export func $main
  itof2 %af 3
  itof2 %bf 2
  mulf  %rf %af %bf
  ftoi2 %ri %rf
  ret   %ri
endfunc
CAIEOF
compile_cai "$TMPDIR_WORK/f32.cai" "$TMPDIR_WORK/f32"
check_result "f32 mulf 3.0*2.0=6 (result=6)" "6" "$TMPDIR_WORK/f32"

cat > "$TMPDIR_WORK/f32ops.cai" << 'CAIEOF'
export func $main
  itof2 %af 10
  itof2 %bf 3
  addf  %r1 %af %bf
  subf  %r2 %af %bf
  divf  %r3 %af %bf
  ftoi2 %i1 %r1
  ftoi2 %i2 %r2
  ftoi2 %i3 %r3
  add   %r %i1 %i2
  add   %ret %r %i3
  ret   %ret
endfunc
CAIEOF
compile_cai "$TMPDIR_WORK/f32ops.cai" "$TMPDIR_WORK/f32ops"
check_result "f32 addf/subf/divf 13+7+3=23 (result=23)" "23" "$TMPDIR_WORK/f32ops"

echo ""
echo "--- バイト操作（loadb/storeb）---"

cat > "$TMPDIR_WORK/loadb.cai" << 'CAIEOF'
data $str "ABC"
export func $main
  loadb %c $str
  ret   %c
endfunc
CAIEOF
compile_cai "$TMPDIR_WORK/loadb.cai" "$TMPDIR_WORK/loadb"
check_result "loadb 'A'=65 (result=65)" "65" "$TMPDIR_WORK/loadb"

echo ""
echo "--- ポインタ操作（storep/loadp2/addp）---"

cat > "$TMPDIR_WORK/ptr.cai" << 'CAIEOF'
data $msg "OK"
export func $main
  alloc  %p.ptr 8
  storep %p.ptr $msg
  loadp2 %p %p.ptr
  addp   %p2 %p 1
  loadb  %c %p2
  ret    %c
endfunc
CAIEOF
compile_cai "$TMPDIR_WORK/ptr.cai" "$TMPDIR_WORK/ptr"
check_result "addp+loadb 'K'=75 (result=75)" "75" "$TMPDIR_WORK/ptr"

echo ""
echo "--- syscall ---"

cat > "$TMPDIR_WORK/syscall.cai" << 'CAIEOF'
data $hello "Hi\n"
export func $main
  syscall %ret 1 2 $hello 3
  ret    0
endfunc
CAIEOF
compile_cai "$TMPDIR_WORK/syscall.cai" "$TMPDIR_WORK/syscall"
check "syscall write(stderr)実行成功" "$TMPDIR_WORK/syscall"
# stderrに"Hi\n"が出力されているか
actual_err=$("$TMPDIR_WORK/syscall" 2>&1 1>/dev/null) || true
if echo "$actual_err" | grep -q "Hi"; then
    echo -e "${GREEN}[PASS]${NC} syscall write出力確認 (Hi)"
    pass=$((pass+1))
else
    echo -e "${RED}[FAIL]${NC} syscall write出力確認"
    fail=$((fail+1))
fi

echo ""
echo "--- 再帰（fibonacci）---"

cat > "$TMPDIR_WORK/fib.cai" << 'CAIEOF'
func $fib
  alloc %n.ptr 4
  store %n.ptr %arg0
  load  %n %n.ptr
  cle   %cond %n 1
  jnz   %cond fib_base fib_rec
  label fib_base
  ret   %n
  label fib_rec
  sub   %n1 %n 1
  call  %a $fib %n1
  sub   %n2 %n 2
  call  %b $fib %n2
  add   %r %a %b
  ret   %r
endfunc
export func $main
  call %r $fib 10
  ret  %r
endfunc
CAIEOF
compile_cai "$TMPDIR_WORK/fib.cai" "$TMPDIR_WORK/fib"
check_result "fibonacci(10)=55 (result=55)" "55" "$TMPDIR_WORK/fib"

echo ""
echo "--- readelf/objdump詳細検証 ---"
check_output "objdump逆アセンブル可能"  "push\|mov\|ret"  objdump -d "$TMPDIR_WORK/basic"
check_output "readelf Program Headers"   "PHDR\|LOAD"      readelf -l "$TMPDIR_WORK/basic"
check_output "readelf Section Headers"   "\.text"          readelf -S "$TMPDIR_WORK/basic"
check_output "readelf ELF Header"        "DYN"             readelf -h "$TMPDIR_WORK/basic"

echo ""
echo "========================================"
printf " 結果: ${GREEN}PASS${NC} %d / ${RED}FAIL${NC} %d\n" "$pass" "$fail"
echo "========================================"

[ "$fail" -eq 0 ]
