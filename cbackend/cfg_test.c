/*
 * cfg_test.c — CFG Builder のテスト
 *
 * BIRテキストを直接パースして CFG を構築し、
 * Block数・エッジ・LoopDepth・BackEdgeを検証する。
 *
 * コンパイル:
 *   gcc -O2 -Wall -std=c11 -o cfg_test cfg_test.c bir_parser.c cfg.c bir_dump.c
 * 実行:
 *   ./cfg_test
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include "backend.h"
#include "cfg.h"

/* ===== テストフレームワーク ===== */

static int g_tests = 0;
static int g_fails = 0;

#define CHECK(cond, msg) do { \
    g_tests++; \
    if (!(cond)) { \
        fprintf(stderr, "FAIL [%s:%d] %s\n", __func__, __LINE__, msg); \
        g_fails++; \
    } \
} while(0)

#define CHECK_EQ(a, b, msg) do { \
    g_tests++; \
    if ((a) != (b)) { \
        fprintf(stderr, "FAIL [%s:%d] %s: got %d, want %d\n", \
                __func__, __LINE__, msg, (int)(a), (int)(b)); \
        g_fails++; \
    } \
} while(0)

/* ===== BIRファイル書き出しヘルパー ===== */

static BackendProgram *parse_bir_str(const char *bir, const char *tmpfile) {
    FILE *f = fopen(tmpfile, "w");
    if (!f) { fprintf(stderr, "cannot open %s\n", tmpfile); return NULL; }
    fputs(bir, f);
    fclose(f);
    return bir_parse(tmpfile);
}

/* ===== CFGヘルパー ===== */

static CFGBlock *find_block_by_kind(const CFG *cfg, BlockKind kind) {
    for (int i = 0; i < cfg->nblocks; i++) {
        if (cfg->blocks[i]->kind == kind)
            return cfg->blocks[i];
    }
    return NULL;
}

static int count_blocks_by_kind(const CFG *cfg, BlockKind kind) {
    int n = 0;
    for (int i = 0; i < cfg->nblocks; i++) {
        if (cfg->blocks[i]->kind == kind) n++;
    }
    return n;
}

static bool has_succ(const CFGBlock *from, const CFGBlock *to) {
    for (int i = 0; i < from->nsuccs; i++) {
        if (from->succs[i] == to) return true;
    }
    return false;
}

static bool has_back_edge(const CFGBlock *from, const CFGBlock *to) {
    for (int i = 0; i < from->nsuccs; i++) {
        if (from->succs[i] == to && from->is_back_edge[i]) return true;
    }
    return false;
}

static bool has_pred(const CFGBlock *blk, const CFGBlock *pred) {
    for (int i = 0; i < blk->npreds; i++) {
        if (blk->preds[i] == pred) return true;
    }
    return false;
}

/* ===== テスト: fibonacci ===== */
/* fibonacci: If分岐のみ（LoopなしのCFG）
 *
 * BIR:
 *   FUNC fibonacci 0 4 0
 *   PARAM n 4 0
 *   LOCAL a 4 0
 *   LOCAL b 4 0
 *   BODY
 *   IF
 *   COND lesseq n 4 0 1 4 0
 *   IFTRUE
 *   RET 4 0 IDENT n 4 0
 *   IFFALSE
 *   STORE a 4 0 CALL fibonacci 4 0 1 EXPR - 4 0 IDENT n 4 0 LIT_INT 1
 *   STORE b 4 0 CALL fibonacci 4 0 1 EXPR - 4 0 IDENT n 4 0 LIT_INT 2
 *   RET 4 0 EXPR + 4 0 IDENT a 4 0 IDENT b 4 0
 *   ENDIF
 *   ENDFUNC
 */
static const char *BIR_FIB =
    "BIR 1\n"
    "FUNC fibonacci 0 4 0\n"
    "PARAM n 4 0\n"
    "LOCAL a 4 0\n"
    "LOCAL b 4 0\n"
    "BODY\n"
    "IF\n"
    "COND lesseq n 4 0 1 4 0\n"
    "IFTRUE\n"
    "RET 4 0 IDENT n 4 0\n"
    "IFFALSE\n"
    "STORE a 4 0 CALL fibonacci 4 0 1 EXPR - 4 0 IDENT n 4 0 LIT_INT 1\n"
    "STORE b 4 0 CALL fibonacci 4 0 1 EXPR - 4 0 IDENT n 4 0 LIT_INT 2\n"
    "RET 4 0 EXPR + 4 0 IDENT a 4 0 IDENT b 4 0\n"
    "ENDIF\n"
    "ENDFUNC\n"
    "\n"
    "FUNC main 1 4 0\n"
    "LOCAL result 4 0\n"
    "BODY\n"
    "STORE result 4 0 CALL fibonacci 4 0 1 LIT_INT 40\n"
    "RET 4 0 IDENT result 4 0\n"
    "ENDFUNC\n";

static void test_fibonacci(void) {
    printf("--- test_fibonacci ---\n");
    BackendProgram *prog = parse_bir_str(BIR_FIB, "/tmp/test_fib.bir");
    CHECK(prog != NULL, "prog != NULL");
    CHECK_EQ(prog->func_count, 2, "func_count");

    CFG *cfg = cfg_build(&prog->funcs[0]); /* fibonacci */
    CHECK(cfg != NULL, "cfg != NULL");

    /* Entry が存在する */
    CHECK(cfg->entry != NULL, "entry != NULL");
    CHECK_EQ(cfg->entry->kind, BLOCK_ENTRY, "entry kind");

    /* IfBlock が1つ存在する */
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_IF), 1, "1 if block");

    /* IfTrueBlock / IfFalseBlock が各1つ */
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_IF_TRUE),  1, "1 if_true block");
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_IF_FALSE), 1, "1 if_false block");

    /* MergeBlock（TrueもFalseもReturnで終わるのでMergeには到達しない → 0） */
    /* Mergeは生成されるが前任がない可能性 → nblocks を確認するだけ */

    /* IfBlockの条件が正しい */
    CFGBlock *if_blk = find_block_by_kind(cfg, BLOCK_IF);
    CHECK(if_blk != NULL, "if_blk != NULL");
    CHECK(if_blk->has_cond, "if_blk has_cond");
    CHECK(strcmp(if_blk->cond.op, "lesseq") == 0, "cond.op == lesseq");
    CHECK(strcmp(if_blk->cond.left, "n") == 0, "cond.left == n");
    CHECK_EQ(if_blk->cond.left_size, 4, "cond.left_size");
    CHECK_EQ(if_blk->nsuccs, 2, "if_blk has 2 succs");

    /* TrueBlock に RET が入っている */
    CFGBlock *true_blk = find_block_by_kind(cfg, BLOCK_IF_TRUE);
    CHECK(true_blk != NULL, "true_blk != NULL");
    CHECK(has_succ(if_blk, true_blk), "if_blk → true_blk");
    CHECK(has_pred(true_blk, if_blk), "true_blk pred = if_blk");

    /* FalseBlock に STORE が入っている */
    CFGBlock *false_blk = find_block_by_kind(cfg, BLOCK_IF_FALSE);
    CHECK(false_blk != NULL, "false_blk != NULL");
    CHECK(has_succ(if_blk, false_blk), "if_blk → false_blk");
    CHECK(has_pred(false_blk, if_blk), "false_blk pred = if_blk");

    /* ReturnBlock が2つ存在する（TrueとFalseそれぞれ） */
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_RETURN), 2, "2 return blocks");

    /* LoopBlockは存在しない */
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_HEADER), 0, "no loop_header");
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_BODY),   0, "no loop_body");

    cfg_dump(cfg);
    cfg_free(cfg);
    bir_free(prog);
    printf("\n");
}

/* ===== テスト: sum (1重ループ) ===== */
static const char *BIR_SUM =
    "BIR 1\n"
    "FUNC main 1 4 0\n"
    "LOCAL i 4 0\n"
    "LOCAL sum 4 0\n"
    "BODY\n"
    "STORE i 4 0 LIT_INT 0\n"
    "STORE sum 4 0 LIT_INT 0\n"
    "LOOP 0\n"
    "COND lesseq i 4 0 100000000 4 0\n"
    "LOOPBODY\n"
    "STORE sum 4 0 EXPR + 4 0 IDENT sum 4 0 IDENT i 4 0\n"
    "INCR i 4 0 INC\n"
    "ENDLOOP\n"
    "RET 4 0 IDENT sum 4 0\n"
    "ENDFUNC\n";

static void test_sum(void) {
    printf("--- test_sum ---\n");
    BackendProgram *prog = parse_bir_str(BIR_SUM, "/tmp/test_sum.bir");
    CHECK(prog != NULL, "prog != NULL");

    CFG *cfg = cfg_build(&prog->funcs[0]);
    CHECK(cfg != NULL, "cfg != NULL");

    /* Entry */
    CHECK(cfg->entry->kind == BLOCK_ENTRY, "entry kind");

    /* LoopHeader / LoopBody / LoopExit が各1つ */
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_HEADER), 1, "1 loop_header");
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_BODY),   1, "1 loop_body");
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_EXIT),   1, "1 loop_exit");

    CFGBlock *header = find_block_by_kind(cfg, BLOCK_LOOP_HEADER);
    CFGBlock *body   = find_block_by_kind(cfg, BLOCK_LOOP_BODY);
    CFGBlock *lexit  = find_block_by_kind(cfg, BLOCK_LOOP_EXIT);

    /* LoopHeaderの条件 */
    CHECK(header->has_cond, "header has_cond");
    CHECK(strcmp(header->cond.op, "lesseq") == 0, "header cond.op == lesseq");
    CHECK_EQ(header->loop_depth, 0, "header loop_depth == 0");

    /* LoopHeader → LoopBody（条件真） */
    CHECK(has_succ(header, body), "header → body");
    /* LoopHeader → LoopExit（条件偽） */
    CHECK(has_succ(header, lexit), "header → exit");
    CHECK_EQ(header->nsuccs, 2, "header 2 succs");

    /* LoopBody → LoopHeader（バックエッジ） */
    CHECK(has_succ(body, header), "body → header");
    CHECK(has_back_edge(body, header), "body → header is back_edge");

    /* Predの対称性 */
    CHECK(has_pred(body,   header), "body.pred contains header");
    CHECK(has_pred(header, body),   "header.pred contains body (back edge)");
    CHECK(has_pred(lexit,  header), "exit.pred contains header");

    /* LoopBodyのloop_depthが0 */
    CHECK_EQ(body->loop_depth, 0, "body loop_depth == 0");

    /* LoopBodyにMutationとIncrが入っている */
    CHECK_EQ(body->nstmts, 2, "body has 2 stmts");
    CHECK_EQ(body->stmts[0]->kind, STMT_STORE, "body stmt[0] == STORE");
    CHECK_EQ(body->stmts[1]->kind, STMT_INCR,  "body stmt[1] == INCR");

    /* LoopExit後にReturnがある */
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_RETURN), 1, "1 return block");

    cfg_dump(cfg);
    cfg_free(cfg);
    bir_free(prog);
    printf("\n");
}

/* ===== テスト: nested_loop (2重ループ) ===== */
static const char *BIR_NESTED =
    "BIR 1\n"
    "FUNC main 1 4 0\n"
    "LOCAL i 4 0\n"
    "LOCAL j 4 0\n"
    "LOCAL count 4 0\n"
    "BODY\n"
    "STORE i 4 0 LIT_INT 0\n"
    "STORE j 4 0 LIT_INT 0\n"
    "STORE count 4 0 LIT_INT 0\n"
    "LOOP 0\n"
    "COND less i 4 0 1000 4 0\n"
    "LOOPBODY\n"
    "STORE j 4 0 LIT_INT 0\n"
    "LOOP 1\n"
    "COND less j 4 0 1000 4 0\n"
    "LOOPBODY\n"
    "STORE count 4 0 EXPR + 4 0 IDENT count 4 0 LIT_INT 1\n"
    "INCR j 4 0 INC\n"
    "ENDLOOP\n"
    "INCR i 4 0 INC\n"
    "ENDLOOP\n"
    "RET 4 0 IDENT count 4 0\n"
    "ENDFUNC\n";

static void test_nested_loop(void) {
    printf("--- test_nested_loop ---\n");
    BackendProgram *prog = parse_bir_str(BIR_NESTED, "/tmp/test_nested.bir");
    CHECK(prog != NULL, "prog != NULL");

    CFG *cfg = cfg_build(&prog->funcs[0]);
    CHECK(cfg != NULL, "cfg != NULL");

    /* LoopHeader / LoopBody / LoopExit が各2つ（外側・内側） */
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_HEADER), 2, "2 loop_headers");
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_BODY),   2, "2 loop_bodies");
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_EXIT),   2, "2 loop_exits");

    /* 外側LoopHeader: depth=0 */
    /* 内側LoopHeader: depth=1 */
    int found_outer = 0, found_inner = 0;
    for (int i = 0; i < cfg->nblocks; i++) {
        CFGBlock *b = cfg->blocks[i];
        if (b->kind == BLOCK_LOOP_HEADER) {
            if (b->loop_depth == 0) found_outer++;
            if (b->loop_depth == 1) found_inner++;
        }
    }
    CHECK_EQ(found_outer, 1, "1 outer loop_header (depth=0)");
    CHECK_EQ(found_inner, 1, "1 inner loop_header (depth=1)");

    /* 全ブロックのバックエッジ数（ループごとに1つ） */
    int back_edges = 0;
    for (int i = 0; i < cfg->nblocks; i++) {
        for (int j = 0; j < cfg->blocks[i]->nsuccs; j++) {
            if (cfg->blocks[i]->is_back_edge[j]) back_edges++;
        }
    }
    CHECK_EQ(back_edges, 2, "2 back edges (one per loop)");

    cfg_dump(cfg);
    cfg_free(cfg);
    bir_free(prog);
    printf("\n");
}

/* ===== テスト: matrix (3重ループ) ===== */
static const char *BIR_MATRIX =
    "BIR 1\n"
    "FUNC main 1 4 0\n"
    "LOCAL N 4 0\n"
    "LOCAL sum 4 0\n"
    "LOCAL i 4 0\n"
    "LOCAL j 4 0\n"
    "LOCAL k 4 0\n"
    "BODY\n"
    "STORE N 4 0 LIT_INT 200\n"
    "STORE sum 4 0 LIT_INT 0\n"
    "STORE i 4 0 LIT_INT 0\n"
    "STORE j 4 0 LIT_INT 0\n"
    "STORE k 4 0 LIT_INT 0\n"
    "LOOP 0\n"
    "COND less i 4 0 N 4 0\n"
    "LOOPBODY\n"
    "STORE j 4 0 LIT_INT 0\n"
    "LOOP 1\n"
    "COND less j 4 0 N 4 0\n"
    "LOOPBODY\n"
    "STORE k 4 0 LIT_INT 0\n"
    "LOOP 2\n"
    "COND less k 4 0 N 4 0\n"
    "LOOPBODY\n"
    "STORE sum 4 0 EXPR + 4 0 IDENT sum 4 0 EXPR * 4 0 IDENT i 4 0 IDENT k 4 0\n"
    "INCR k 4 0 INC\n"
    "ENDLOOP\n"
    "INCR j 4 0 INC\n"
    "ENDLOOP\n"
    "INCR i 4 0 INC\n"
    "ENDLOOP\n"
    "RET 4 0 IDENT sum 4 0\n"
    "ENDFUNC\n";

static void test_matrix(void) {
    printf("--- test_matrix ---\n");
    BackendProgram *prog = parse_bir_str(BIR_MATRIX, "/tmp/test_matrix.bir");
    CHECK(prog != NULL, "prog != NULL");

    CFG *cfg = cfg_build(&prog->funcs[0]);
    CHECK(cfg != NULL, "cfg != NULL");

    /* 3重ループ: LoopHeader/Body/Exit が各3つ */
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_HEADER), 3, "3 loop_headers");
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_BODY),   3, "3 loop_bodies");
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_EXIT),   3, "3 loop_exits");

    /* depth=0, 1, 2 の LoopHeader が各1つ */
    int d[3] = {0, 0, 0};
    for (int i = 0; i < cfg->nblocks; i++) {
        CFGBlock *b = cfg->blocks[i];
        if (b->kind == BLOCK_LOOP_HEADER && b->loop_depth <= 2) {
            d[b->loop_depth]++;
        }
    }
    CHECK_EQ(d[0], 1, "1 loop_header depth=0");
    CHECK_EQ(d[1], 1, "1 loop_header depth=1");
    CHECK_EQ(d[2], 1, "1 loop_header depth=2");

    /* バックエッジが3つ存在する */
    int back_edges = 0;
    for (int i = 0; i < cfg->nblocks; i++) {
        CFGBlock *b = cfg->blocks[i];
        for (int j = 0; j < b->nsuccs; j++) {
            if (b->is_back_edge[j]) back_edges++;
        }
    }
    CHECK_EQ(back_edges, 3, "3 back edges");

    cfg_dump(cfg);
    cfg_free(cfg);
    bir_free(prog);
    printf("\n");
}

/* ===== テスト: Pred/Succ対称性 ===== */
static void test_edge_symmetry(void) {
    printf("--- test_edge_symmetry (sum) ---\n");
    BackendProgram *prog = parse_bir_str(BIR_SUM, "/tmp/test_sym.bir");
    CHECK(prog != NULL, "prog != NULL");

    CFG *cfg = cfg_build(&prog->funcs[0]);
    CHECK(cfg != NULL, "cfg != NULL");

    /* AがBのSuccならBのPredにAが含まれる */
    for (int i = 0; i < cfg->nblocks; i++) {
        CFGBlock *a = cfg->blocks[i];
        for (int j = 0; j < a->nsuccs; j++) {
            CFGBlock *b = a->succs[j];
            CHECK(has_pred(b, a), "pred/succ symmetry");
        }
    }

    cfg_free(cfg);
    bir_free(prog);
    printf("\n");
}

/* ===== テスト: EntryBlockは常に存在する ===== */
static void test_entry_always_exists(void) {
    printf("--- test_entry_always_exists ---\n");
    /* 最小のBIR: 変数1つとReturn */
    const char *bir =
        "BIR 1\n"
        "FUNC f 0 4 0\n"
        "LOCAL x 4 0\n"
        "BODY\n"
        "STORE x 4 0 LIT_INT 42\n"
        "RET 4 0 IDENT x 4 0\n"
        "ENDFUNC\n";
    BackendProgram *prog = parse_bir_str(bir, "/tmp/test_entry.bir");
    CHECK(prog != NULL, "prog != NULL");

    CFG *cfg = cfg_build(&prog->funcs[0]);
    CHECK(cfg != NULL, "cfg != NULL");
    CHECK(cfg->nblocks > 0, "nblocks > 0");
    CHECK(cfg->entry != NULL, "entry != NULL");
    CHECK_EQ(cfg->entry->kind, BLOCK_ENTRY, "entry.kind == BLOCK_ENTRY");
    CHECK_EQ(cfg->entry->id, 0, "entry.id == 0");

    cfg_free(cfg);
    bir_free(prog);
    printf("\n");
}

/* ===== テスト: If+Loop の組み合わせ ===== */
static void test_if_in_loop(void) {
    printf("--- test_if_in_loop ---\n");
    /* ループ内にIfがある */
    const char *bir =
        "BIR 1\n"
        "FUNC f 0 4 0\n"
        "LOCAL i 4 0\n"
        "LOCAL x 4 0\n"
        "BODY\n"
        "STORE i 4 0 LIT_INT 0\n"
        "STORE x 4 0 LIT_INT 0\n"
        "LOOP 0\n"
        "COND less i 4 0 10 4 0\n"
        "LOOPBODY\n"
        "IF\n"
        "COND eq i 4 0 5 4 0\n"
        "IFTRUE\n"
        "STORE x 4 0 LIT_INT 1\n"
        "IFFALSE\n"
        "STORE x 4 0 LIT_INT 0\n"
        "ENDIF\n"
        "INCR i 4 0 INC\n"
        "ENDLOOP\n"
        "RET 4 0 IDENT x 4 0\n"
        "ENDFUNC\n";
    BackendProgram *prog = parse_bir_str(bir, "/tmp/test_ifloop.bir");
    CHECK(prog != NULL, "prog != NULL");

    CFG *cfg = cfg_build(&prog->funcs[0]);
    CHECK(cfg != NULL, "cfg != NULL");

    /* LoopとIfが両方存在する */
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_LOOP_HEADER), 1, "1 loop_header");
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_IF),          1, "1 if block");
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_IF_TRUE),     1, "1 if_true");
    CHECK_EQ(count_blocks_by_kind(cfg, BLOCK_IF_FALSE),    1, "1 if_false");

    /* バックエッジが1つ */
    int back_edges = 0;
    for (int i = 0; i < cfg->nblocks; i++) {
        for (int j = 0; j < cfg->blocks[i]->nsuccs; j++) {
            if (cfg->blocks[i]->is_back_edge[j]) back_edges++;
        }
    }
    CHECK_EQ(back_edges, 1, "1 back edge");

    cfg_free(cfg);
    bir_free(prog);
    printf("\n");
}

/* ===== メイン ===== */

int main(void) {
    printf("===== CFG Tests =====\n\n");

    test_entry_always_exists();
    test_fibonacci();
    test_sum();
    test_nested_loop();
    test_matrix();
    test_edge_symmetry();
    test_if_in_loop();

    printf("===== Results =====\n");
    printf("Tests: %d  Failures: %d\n", g_tests, g_fails);
    return g_fails > 0 ? 1 : 0;
}
