/*
 * cfg.c — CFG (Control Flow Graph) Builder
 *
 * 入力: BackendFunction（bir_parser.cが生成済み）
 * 出力: CFG（CFGBlockのポインタグラフ）
 *
 * 設計原則:
 *   - BackendFunctionの情報をそのまま使う（型推論・名前解決しない）
 *   - STMT_LOOP.depth はAnalyzerが付与済み → そのまま loop_depth に設定
 *   - STMT_LOOP / STMT_IF は再帰的に展開して CFGBlock を生成する
 *   - BStmtポインタは BackendFunction.stmts を直接参照（コピーしない）
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include "backend.h"
#include "cfg.h"

/* ===== メモリ管理 ===== */

static void *xmalloc(size_t n) {
    void *p = calloc(1, n);
    if (!p) { fprintf(stderr, "CFG: OOM\n"); exit(1); }
    return p;
}

/* ===== ブロック操作 ===== */

static CFGBlock *new_block(CFG *cfg, BlockKind kind, int loop_depth) {
    if (cfg->nblocks >= MAX_BLOCKS) {
        fprintf(stderr, "CFG: too many blocks\n");
        exit(1);
    }
    CFGBlock *b = (CFGBlock *)xmalloc(sizeof(CFGBlock));
    b->id         = cfg->nblocks;
    b->kind       = kind;
    b->loop_depth = loop_depth;
    cfg->blocks[cfg->nblocks++] = b;
    return b;
}

/* fromからtoへの有向エッジを張る */
static void add_edge(CFGBlock *from, CFGBlock *to) {
    if (from->nsuccs >= MAX_SUCCS) {
        fprintf(stderr, "CFG: too many succs on block %d\n", from->id);
        return;
    }
    if (to->npreds >= MAX_PREDS) {
        fprintf(stderr, "CFG: too many preds on block %d\n", to->id);
        return;
    }
    from->succs[from->nsuccs++] = to;
    to->preds[to->npreds++]     = from;
}

/* バックエッジ付きエッジ（LoopBodyからLoopHeaderへ） */
static void add_back_edge(CFGBlock *from, CFGBlock *to) {
    int idx = from->nsuccs;
    add_edge(from, to);
    if (idx < MAX_SUCCS) {
        from->is_back_edge[idx] = true;
    }
}

/* ブロックにBStmtを追加（ポインタのみ、コピーしない） */
static void block_add_stmt(CFGBlock *b, const BStmt *s) {
    if (b->nstmts >= MAX_BLOCK_STMTS) {
        fprintf(stderr, "CFG: too many stmts in block %d\n", b->id);
        return;
    }
    b->stmts[b->nstmts++] = s;
}

/* ===== ループスタック（break/continue解決用） ===== */
#define MAX_LOOP_DEPTH 64

typedef struct {
    CFGBlock *headers[MAX_LOOP_DEPTH];
    CFGBlock *exits[MAX_LOOP_DEPTH];
    int       top;
} LoopStack;

static void loop_push(LoopStack *ls, CFGBlock *header, CFGBlock *exit_blk) {
    if (ls->top >= MAX_LOOP_DEPTH) {
        fprintf(stderr, "CFG: loop nesting too deep\n");
        return;
    }
    ls->headers[ls->top] = header;
    ls->exits[ls->top]   = exit_blk;
    ls->top++;
}

static void loop_pop(LoopStack *ls) {
    if (ls->top > 0) ls->top--;
}

static CFGBlock *loop_current_header(LoopStack *ls) {
    return ls->top > 0 ? ls->headers[ls->top - 1] : NULL;
}

static CFGBlock *loop_current_exit(LoopStack *ls) {
    return ls->top > 0 ? ls->exits[ls->top - 1] : NULL;
}

/* ===== CFG Builder（再帰） ===== */

/*
 * build_stmts: stmts[0..n-1] を current ブロックから処理する。
 * 戻り値: フォールスルー先のブロック（終端した場合は NULL）
 */
static CFGBlock *build_stmts(CFG *cfg, LoopStack *ls,
                               const BStmt *stmts, int n,
                               CFGBlock *current, int depth);

static CFGBlock *build_stmt(CFG *cfg, LoopStack *ls,
                              const BStmt *s, CFGBlock *current, int depth);

static CFGBlock *build_stmts(CFG *cfg, LoopStack *ls,
                               const BStmt *stmts, int n,
                               CFGBlock *current, int depth)
{
    for (int i = 0; i < n; i++) {
        if (current == NULL) break; /* 終端後は到達不能 */
        current = build_stmt(cfg, ls, &stmts[i], current, depth);
    }
    return current;
}

static CFGBlock *build_stmt(CFG *cfg, LoopStack *ls,
                              const BStmt *s, CFGBlock *current, int depth)
{
    switch (s->kind) {

    /* ===== 通常文: 現在のブロックに追加するだけ ===== */
    case STMT_STORE:
    case STMT_INCR:
    case STMT_ARRSTORE:
        block_add_stmt(current, s);
        return current;

    /* ===== Return: ReturnBlockを生成して終端 ===== */
    case STMT_RET:
    case STMT_RET_VOID: {
        CFGBlock *ret_blk = new_block(cfg, BLOCK_RETURN, depth);
        block_add_stmt(ret_blk, s);
        add_edge(current, ret_blk);
        /* ExitBlockへも接続 */
        if (cfg->exit_block) {
            add_edge(ret_blk, cfg->exit_block);
        }
        return NULL; /* 終端 */
    }

    /* ===== If: Cond → True/False → Merge ===== */
    case STMT_IF: {
        const BIf *bif = &s->bif;

        /* Condブロック */
        CFGBlock *cond_blk = new_block(cfg, BLOCK_IF, depth);
        cond_blk->has_cond = true;
        cond_blk->cond     = bif->cond;
        add_edge(current, cond_blk);

        /* Mergeブロック（先に作成してTrue/Falseから接続） */
        CFGBlock *merge_blk = new_block(cfg, BLOCK_MERGE, depth);

        /* Trueブロック */
        CFGBlock *true_blk = new_block(cfg, BLOCK_IF_TRUE, depth);
        add_edge(cond_blk, true_blk);
        CFGBlock *true_end = build_stmts(cfg, ls,
                                          bif->true_stmts, bif->true_count,
                                          true_blk, depth);
        if (true_end != NULL) {
            add_edge(true_end, merge_blk);
        }

        /* Falseブロック */
        CFGBlock *false_blk = new_block(cfg, BLOCK_IF_FALSE, depth);
        add_edge(cond_blk, false_blk);
        CFGBlock *false_end = build_stmts(cfg, ls,
                                           bif->false_stmts, bif->false_count,
                                           false_blk, depth);
        if (false_end != NULL) {
            add_edge(false_end, merge_blk);
        }

        return merge_blk;
    }

    /* ===== Loop: Header → Body → Header（バックエッジ）/ Exit ===== */
    case STMT_LOOP: {
        const BLoop *bl = &s->loop;
        int ldepth = bl->depth; /* Analyzerが付与済み → そのまま使う */

        /* LoopHeaderブロック */
        CFGBlock *header_blk = new_block(cfg, BLOCK_LOOP_HEADER, ldepth);
        header_blk->has_cond = true;
        header_blk->cond     = bl->cond;
        add_edge(current, header_blk);

        /* LoopExitブロック（条件偽で脱出） */
        CFGBlock *exit_blk = new_block(cfg, BLOCK_LOOP_EXIT, ldepth);

        /* Header → Exit（条件偽） */
        add_edge(header_blk, exit_blk);

        /* スタックにpush（break/continue解決用） */
        loop_push(ls, header_blk, exit_blk);

        /* LoopBodyブロック */
        CFGBlock *body_blk = new_block(cfg, BLOCK_LOOP_BODY, ldepth);
        /* Header → Body（条件真）*/
        add_edge(header_blk, body_blk);

        /* Bodyの内容を処理 */
        CFGBlock *body_end = build_stmts(cfg, ls,
                                          bl->body, bl->body_count,
                                          body_blk, ldepth);
        /* Body → Header（バックエッジ） */
        if (body_end != NULL) {
            add_back_edge(body_end, header_blk);
        }

        /* スタックからpop */
        loop_pop(ls);

        return exit_blk;
    }

    /* ===== Break: LoopExitへジャンプ ===== */
    case STMT_BREAK: {
        CFGBlock *exit_blk = loop_current_exit(ls);
        if (exit_blk) {
            add_edge(current, exit_blk);
        }
        return NULL; /* 終端 */
    }

    /* ===== Continue: LoopHeaderへジャンプ ===== */
    case STMT_CONTINUE: {
        CFGBlock *header_blk = loop_current_header(ls);
        if (header_blk) {
            add_back_edge(current, header_blk);
        }
        return NULL; /* 終端 */
    }

    default:
        /* 未知のstmt: スキップ */
        return current;
    }
}

/* ===== CFGのビルドエントリ ===== */

CFG *cfg_build(const BackendFunction *func) {
    CFG *cfg = (CFG *)xmalloc(sizeof(CFG));
    cfg->func = func;

    /* Exitブロック（全Returnが収束する仮想終端） */
    cfg->exit_block = new_block(cfg, BLOCK_RETURN, 0);
    /* ※ exit_blockはIDを0ではなく後で付与される（nblocks順） */
    /* 順序の都合上、exit_blockは後から作るほうが自然 */
    /* → 一度free してから再構築する */
    /* ここでは exit_block を最後に追加する設計にする */

    /* 再設計: exit_blockは先に作らず、Returnごとに独立ブロックとする */
    /* cfg->exit_block は使わない（NULL のまま） */
    free(cfg->exit_block);
    cfg->exit_block = NULL;
    cfg->nblocks = 0; /* リセット */

    /* Entryブロック */
    CFGBlock *entry = new_block(cfg, BLOCK_ENTRY, 0);
    cfg->entry = entry;

    /* LoopStack初期化 */
    LoopStack ls = {0};

    /* Body全体を処理 */
    build_stmts(cfg, &ls, func->stmts, func->stmt_count, entry, 0);

    return cfg;
}

/* ===== Program単位ビルド ===== */

CFGProgram *cfg_build_program(const BackendProgram *prog) {
    CFGProgram *p = (CFGProgram *)xmalloc(sizeof(CFGProgram));
    p->ncfgs = prog->func_count;
    p->cfgs  = (CFG **)xmalloc(sizeof(CFG *) * (size_t)p->ncfgs);
    for (int i = 0; i < prog->func_count; i++) {
        p->cfgs[i] = cfg_build(&prog->funcs[i]);
    }
    return p;
}

/* ===== メモリ解放 ===== */

void cfg_free(CFG *cfg) {
    if (!cfg) return;
    for (int i = 0; i < cfg->nblocks; i++) {
        free(cfg->blocks[i]);
    }
    free(cfg);
}

void cfg_program_free(CFGProgram *p) {
    if (!p) return;
    for (int i = 0; i < p->ncfgs; i++) {
        cfg_free(p->cfgs[i]);
    }
    free(p->cfgs);
    free(p);
}

/* ===== CFGダンプ ===== */

static const char *block_kind_str(BlockKind k) {
    switch (k) {
    case BLOCK_ENTRY:       return "entry";
    case BLOCK_NORMAL:      return "normal";
    case BLOCK_IF:          return "if";
    case BLOCK_IF_TRUE:     return "if_true";
    case BLOCK_IF_FALSE:    return "if_false";
    case BLOCK_MERGE:       return "merge";
    case BLOCK_LOOP_HEADER: return "loop_header";
    case BLOCK_LOOP_BODY:   return "loop_body";
    case BLOCK_LOOP_EXIT:   return "loop_exit";
    case BLOCK_RETURN:      return "return";
    default:                return "?";
    }
}

/* 式の簡易文字列表現（dump用） */
static void print_expr_short(const BExpr *e) {
    if (!e) { printf("(nil)"); return; }
    switch (e->kind) {
    case EXPR_LIT_INT:   printf("%lld", (long long)e->int_val); break;
    case EXPR_LIT_FLOAT: printf("%g", e->float_val); break;
    case EXPR_LIT_STR:   printf("\"%s\"", e->str_val); break;
    case EXPR_IDENT:     printf("%s", e->ident_name); break;
    case EXPR_BINOP:
        print_expr_short(e->lhs);
        printf(" %s ", e->op);
        print_expr_short(e->rhs);
        break;
    case EXPR_CALL:
        printf("call %s(%d args)", e->func_name, e->argc);
        break;
    case EXPR_CAST:
        printf("cast(");
        print_expr_short(e->inner);
        printf(")");
        break;
    case EXPR_ARRLOAD:
        printf("%s[", e->arr_name);
        print_expr_short(e->index);
        printf("]");
        break;
    case EXPR_ADDR:   printf("addr(%s)", e->addr_name); break;
    case EXPR_DEREF:
        printf("deref(");
        print_expr_short(e->inner);
        printf(")");
        break;
    default: printf("expr(%d)", e->kind); break;
    }
}

static void dump_stmt_short(const BStmt *s) {
    switch (s->kind) {
    case STMT_STORE:
        printf("    STORE %s = ", s->dst_name);
        print_expr_short(s->value);
        printf("  [sz=%d ptr=%d]\n", s->dst_size, s->dst_is_ptr);
        break;
    case STMT_INCR:
        printf("    INCR %s %s\n",
               s->incr_name, s->incr_is_dec ? "--" : "++");
        break;
    case STMT_RET:
        printf("    RET ");
        print_expr_short(s->ret_val);
        printf("  [sz=%d ptr=%d]\n", s->ret_size, s->ret_is_ptr);
        break;
    case STMT_RET_VOID:
        printf("    RET_VOID\n");
        break;
    case STMT_ARRSTORE:
        printf("    ARRSTORE %s[...] = ...\n", s->arr_name);
        break;
    case STMT_BREAK:
        printf("    BREAK\n");
        break;
    case STMT_CONTINUE:
        printf("    CONTINUE\n");
        break;
    default:
        printf("    STMT(%d)\n", s->kind);
        break;
    }
}

static void dump_block(const CFGBlock *b) {
    printf("Block #%d  [%s]", b->id, block_kind_str(b->kind));
    if (b->loop_depth > 0) {
        printf("  depth=%d", b->loop_depth);
    }
    printf("\n");

    /* 条件（If / LoopHeader） */
    if (b->has_cond) {
        printf("  Cond: %s(%s[sz=%d ptr=%d] : %s[sz=%d ptr=%d])\n",
               b->cond.op,
               b->cond.left,  b->cond.left_size,  b->cond.left_is_ptr,
               b->cond.right, b->cond.right_size, b->cond.right_is_ptr);
    }

    /* 文 */
    for (int i = 0; i < b->nstmts; i++) {
        dump_stmt_short(b->stmts[i]);
    }

    /* 後継ブロック */
    if (b->nsuccs > 0) {
        printf("  Succs:");
        for (int i = 0; i < b->nsuccs; i++) {
            printf(" #%d", b->succs[i]->id);
            if (b->is_back_edge[i]) printf("(back)");
        }
        printf("\n");
    }

    /* 前任ブロック */
    if (b->npreds > 0) {
        printf("  Preds:");
        for (int i = 0; i < b->npreds; i++) {
            printf(" #%d", b->preds[i]->id);
        }
        printf("\n");
    }

    printf("--------------------------------\n");
}

void cfg_dump(const CFG *cfg) {
    if (!cfg) return;
    const BackendFunction *fn = cfg->func;
    printf("===== Function: %s%s =====\n\n",
           fn->name, fn->is_public ? " [public]" : "");
    for (int i = 0; i < cfg->nblocks; i++) {
        dump_block(cfg->blocks[i]);
    }
    printf("\n");
}

void cfg_program_dump(const CFGProgram *p) {
    if (!p) return;
    printf("===== DUMP: cfg =====\n\n");
    for (int i = 0; i < p->ncfgs; i++) {
        cfg_dump(p->cfgs[i]);
    }
    printf("===== END CFG =====\n");
}
