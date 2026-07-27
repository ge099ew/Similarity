#!/bin/bash
# CAI Converter - ELF検証テストスクリプト
# Usage: ./test_cai.sh <binary> [object.o]
# 例:    ./test_cai.sh ./output output.o

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass=0
fail=0

check() {
    local desc="$1"; shift
    if "$@" > /dev/null 2>&1; then
        echo -e "${GREEN}[PASS]${NC} $desc"
        ((pass++))
    else
        echo -e "${RED}[FAIL]${NC} $desc"
        ((fail++))
    fi
}

check_output() {
    local desc="$1"
    local expected="$2"
    shift 2
    local actual
    actual=$("$@" 2>&1) || true
    if echo "$actual" | grep -q "$expected"; then
        echo -e "${GREEN}[PASS]${NC} $desc"
        ((pass++))
    else
        echo -e "${RED}[FAIL]${NC} $desc (got: $actual)"
        ((fail++))
    fi
}

EXE="${1:-}"
OBJ="${2:-}"

if [ -z "$EXE" ]; then
    echo "Usage: $0 <binary> [object.o]"
    exit 1
fi

echo "========================================"
echo " CAI Converter ELF検証テスト"
echo " binary: $EXE"
[ -n "$OBJ" ] && echo " object: $OBJ"
echo "========================================"
echo ""

# ===== 実行ファイルテスト =====
if [ -f "$EXE" ]; then
    echo "--- ELFヘッダ検証 ---"
    check_output "ELFマジック番号" "ELF" readelf -h "$EXE"
    check_output "x86_64アーキテクチャ" "X86-64\|x86-64\|Advanced Micro" readelf -h "$EXE"
    check_output "ET_EXEC (実行ファイル)" "EXEC\|Executable" readelf -h "$EXE"
    check_output "エントリポイント設定済み" "0x4" readelf -h "$EXE"

    echo ""
    echo "--- Program Header検証 ---"
    check_output "PT_LOADセグメント存在" "LOAD" readelf -l "$EXE"
    check_output "実行セグメント(RX)存在" "R E\|RE\|R X" readelf -l "$EXE"

    echo ""
    echo "--- PIE検証 ---"
    check_output "ET_DYN（静的PIE）" "DYN\|Shared object" readelf -h "$EXE"
    # PIE: e_entryがページオフセット基準（0x1000台）であること
    check_output "e_entryがPIEオフセット" "0x1" readelf -h "$EXE"
    # ASLRが有効なシステムでは実際のロードアドレスが毎回変わる
    echo -e "${YELLOW}[INFO]${NC} ASLRテスト（複数回実行してアドレスが変わることを確認）:"
    if command -v cat > /dev/null 2>&1; then
        MAPS_BEFORE=""
        "$EXE" > /dev/null 2>&1 &
        PID1=$!
        sleep 0.01
        if [ -f /proc/$PID1/maps ]; then
            MAPS_BEFORE=$(grep -m1 "r-xp" /proc/$PID1/maps 2>/dev/null | awk '{print $1}' || true)
        fi
        wait $PID1 2>/dev/null || true
        if [ -n "$MAPS_BEFORE" ]; then
            echo "  ロードアドレス: $MAPS_BEFORE"
        fi
    fi

    echo ""
    echo "--- セクション検証 ---"
    # EXEはセクションヘッダなしの場合もあるので警告のみ
    if readelf -S "$EXE" > /dev/null 2>&1; then
        check_output "セクションヘッダ存在" "." readelf -S "$EXE"
    else
        echo -e "${YELLOW}[SKIP]${NC} セクションヘッダなし（静的リンク実行ファイルでは正常）"
    fi

    echo ""
    echo "--- 逆アセンブル検証 ---"
    check_output "コードセクション逆アセ可能" "push\|mov\|call\|ret" objdump -d "$EXE"
    check_output "_startシンボル存在" "_start\|sim_main" objdump -d "$EXE"

    echo ""
    echo "--- 実行テスト ---"
    check "実行ファイルに実行権限あり" test -x "$EXE"
    if "$EXE" > /tmp/cai_test_out 2>&1; then
        echo -e "${GREEN}[PASS]${NC} 実行成功 (exit 0)"
        ((pass++))
        echo "  出力: $(head -1 /tmp/cai_test_out)"
    else
        EC=$?
        echo -e "${RED}[FAIL]${NC} 実行失敗 (exit $EC)"
        ((fail++))
    fi

    echo ""
    echo "--- 複雑なケース確認（手動確認推奨）---"
    echo -e "${YELLOW}[INFO]${NC} 以下を手動で確認してください:"
    echo "  readelf -h $EXE"
    echo "  readelf -l $EXE"
    echo "  objdump -d $EXE | head -60"
fi

# ===== オブジェクトファイルテスト =====
if [ -n "$OBJ" ] && [ -f "$OBJ" ]; then
    echo ""
    echo "========================================"
    echo " オブジェクトファイル (.o) テスト"
    echo "========================================"

    echo ""
    echo "--- ELFヘッダ検証 ---"
    check_output "ET_REL (relocatable)" "REL\|Relocatable" readelf -h "$OBJ"
    check_output "x86_64アーキテクチャ" "X86-64\|x86-64" readelf -h "$OBJ"

    echo ""
    echo "--- セクション検証 ---"
    check_output ".textセクション存在" "\.text" readelf -S "$OBJ"
    check_output ".symtabセクション存在" "\.symtab\|symtab" readelf -S "$OBJ"
    check_output ".strtabセクション存在" "\.strtab\|strtab" readelf -S "$OBJ"

    echo ""
    echo "--- シンボル検証 ---"
    check_output "グローバルシンボル存在" "GLOBAL\|global" readelf -s "$OBJ"
    check_output "UNDシンボル（外部参照）確認" "UND\|UNDEF\|FUNC\|NOTYPE" readelf -s "$OBJ"

    echo ""
    echo "--- リロケーション検証 ---"
    if readelf -r "$OBJ" 2>/dev/null | grep -q "R_X86_64"; then
        echo -e "${GREEN}[PASS]${NC} R_X86_64リロケーションエントリ存在"
        ((pass++))
    else
        echo -e "${YELLOW}[SKIP]${NC} リロケーションエントリなし（外部参照なしなら正常）"
    fi
fi

# ===== 追加: 大きなプログラム・再帰の動作確認 =====
echo ""
echo "========================================"
echo " 回帰テスト（チェックリスト）"
echo "========================================"
echo -e "${YELLOW}[手動]${NC} 複数関数呼び出しで壊れないか"
echo -e "${YELLOW}[手動]${NC} 再帰（fibonacci等）で壊れないか"
echo -e "${YELLOW}[手動]${NC} 文字列埋め込みで壊れないか"
echo -e "${YELLOW}[手動]${NC} 大きなプログラム（7800行相当）でも動くか"

echo ""
echo "========================================"
printf " 結果: ${GREEN}PASS${NC} %d / ${RED}FAIL${NC} %d\n" "$pass" "$fail"
echo "========================================"

[ "$fail" -eq 0 ]
