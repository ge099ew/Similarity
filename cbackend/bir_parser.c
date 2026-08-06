/*
 * bir_parser.c — BIR形式テキストを読んでBackendProgramを構築する。
 *
 * 設計原則:
 *   - 型推論しない
 *   - 名前解決しない
 *   - Analyzerが付与した情報（size / is_ptr / loop_depth）をそのまま読む
 *   - BIR形式は1行1トークン列。sscanf/fgetsで処理できる単純な構造。
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdbool.h>
#include "backend.h"

/* ===== メモリ管理 ===== */
/* 小さなアロケータ: mallocのラッパー（将来アリーナに置換しやすいように） */
static void *xmalloc(size_t n) {
    void *p = calloc(1, n);
    if (!p) { fprintf(stderr, "OOM\n"); exit(1); }
    return p;
}

static BStmt *alloc_stmts(int n) {
    return (BStmt *)xmalloc(sizeof(BStmt) * (size_t)n);
}

static BExpr *alloc_expr(void) {
    return (BExpr *)xmalloc(sizeof(BExpr));
}

/* ===== パーサ状態 ===== */
#define MAX_LINE 4096

typedef struct {
    FILE *fp;
    char  line[MAX_LINE];
    char  tokens[MAX_LINE];  /* strtok用バッファ */
    char *tok;               /* 現在のトークン */
    int   lineno;
} Parser;

static bool next_line(Parser *p) {
    while (fgets(p->line, MAX_LINE, p->fp)) {
        p->lineno++;
        /* 先頭の空白・改行を除去 */
        char *s = p->line;
        while (*s == ' ' || *s == '\t') s++;
        if (*s == '\n' || *s == '\r' || *s == '\0') continue;
        /* コメント行を無視 */
        if (s[0] == '#') continue;
        /* tokensバッファにコピーしてstrtok準備 */
        strncpy(p->tokens, s, MAX_LINE - 1);
        p->tokens[MAX_LINE - 1] = '\0';
        /* 末尾の改行を除去 */
        size_t len = strlen(p->tokens);
        while (len > 0 && (p->tokens[len-1] == '\n' || p->tokens[len-1] == '\r'))
            p->tokens[--len] = '\0';
        p->tok = strtok(p->tokens, " \t");
        return true;
    }
    return false;
}

/* 現在行の次トークンを返す（同一行内） */
static char *next_tok(Parser *p __attribute__((unused))) {
    return strtok(NULL, " \t");
}

/* ===== 式のパース ===== */
/* BIR形式の式はインライン（同一行上にトークン列で記述される） */
/* strtokで次々とトークンを消費して式ツリーを構築する */

static BExpr *parse_expr(Parser *p);

static BExpr *parse_expr(Parser *p) {
    char *kind = next_tok(p);
    if (!kind) return NULL;

    BExpr *e = alloc_expr();

    if (strcmp(kind, "LIT_INT") == 0) {
        e->kind    = EXPR_LIT_INT;
        char *val  = next_tok(p);
        e->int_val = val ? strtoll(val, NULL, 10) : 0;
        e->result_size   = 4;
        e->result_is_ptr = false;

    } else if (strcmp(kind, "LIT_FLOAT") == 0) {
        e->kind      = EXPR_LIT_FLOAT;
        char *val    = next_tok(p);
        e->float_val = val ? atof(val) : 0.0;
        e->result_size   = 4;
        e->result_is_ptr = false;

    } else if (strcmp(kind, "LIT_STR") == 0) {
        e->kind = EXPR_LIT_STR;
        char *val = next_tok(p);
        if (val) strncpy(e->str_val, val, sizeof(e->str_val) - 1);
        e->result_size   = 8;
        e->result_is_ptr = true;

    } else if (strcmp(kind, "IDENT") == 0) {
        e->kind = EXPR_IDENT;
        char *name = next_tok(p);
        if (name) strncpy(e->ident_name, name, MAX_NAME - 1);
        char *sz  = next_tok(p);
        char *ip  = next_tok(p);
        e->ident_size   = sz ? atoi(sz) : 4;
        e->ident_is_ptr = ip ? (atoi(ip) != 0) : false;
        e->result_size   = e->ident_size;
        e->result_is_ptr = e->ident_is_ptr;

    } else if (strcmp(kind, "EXPR") == 0) {
        e->kind = EXPR_BINOP;
        char *op = next_tok(p);
        if (op) strncpy(e->op, op, 3);
        char *sz = next_tok(p);
        char *ip = next_tok(p);
        e->result_size   = sz ? atoi(sz) : 4;
        e->result_is_ptr = ip ? (atoi(ip) != 0) : false;
        e->lhs = parse_expr(p);
        e->rhs = parse_expr(p);

    } else if (strcmp(kind, "CALL") == 0) {
        e->kind = EXPR_CALL;
        char *fname = next_tok(p);
        if (fname) strncpy(e->func_name, fname, MAX_NAME - 1);
        char *rsz = next_tok(p);
        char *rip = next_tok(p);
        char *ac  = next_tok(p);
        e->call_return_size   = rsz ? atoi(rsz) : 4;
        e->call_return_is_ptr = rip ? (atoi(rip) != 0) : false;
        e->result_size        = e->call_return_size;
        e->result_is_ptr      = e->call_return_is_ptr;
        e->argc = ac ? atoi(ac) : 0;
        for (int i = 0; i < e->argc && i < MAX_ARGS; i++) {
            e->args[i] = parse_expr(p);
        }

    } else if (strcmp(kind, "CAST") == 0) {
        e->kind = EXPR_CAST;
        char *sz = next_tok(p);
        char *ip = next_tok(p);
        e->result_size   = sz ? atoi(sz) : 4;
        e->result_is_ptr = ip ? (atoi(ip) != 0) : false;
        e->inner = parse_expr(p);

    } else if (strcmp(kind, "ARRLOAD") == 0) {
        e->kind = EXPR_ARRLOAD;
        char *name = next_tok(p);
        if (name) strncpy(e->arr_name, name, MAX_NAME - 1);
        char *sz = next_tok(p);
        char *ip = next_tok(p);
        e->elem_size     = sz ? atoi(sz) : 4;
        e->elem_is_ptr   = ip ? (atoi(ip) != 0) : false;
        e->result_size   = e->elem_size;
        e->result_is_ptr = e->elem_is_ptr;
        e->index = parse_expr(p);

    } else if (strcmp(kind, "ADDR") == 0) {
        e->kind = EXPR_ADDR;
        char *name = next_tok(p);
        if (name) strncpy(e->addr_name, name, MAX_NAME - 1);
        e->result_size   = 8;
        e->result_is_ptr = true;

    } else if (strcmp(kind, "DEREF") == 0) {
        e->kind = EXPR_DEREF;
        char *sz   = next_tok(p);
        char *name = next_tok(p);
        e->result_size   = sz ? atoi(sz) : 4;
        e->result_is_ptr = false;
        /* DEREF <size> <name>: addr_nameにDeref対象の変数名を格納 */
        if (name) strncpy(e->addr_name, name, MAX_NAME - 1);

    } else {
        /* 未知のトークン: ゼロリテラルとして扱う */
        e->kind    = EXPR_LIT_INT;
        e->int_val = 0;
        e->result_size   = 4;
        e->result_is_ptr = false;
    }

    return e;
}

/* ===== 条件のパース ===== */
/* COND <op> <left> <left_size> <left_is_ptr> <right> <right_size> <right_is_ptr> */
static BCond parse_cond(Parser *p) {
    BCond c = {0};
    /* p->tokは既に "COND" になっている */
    char *op    = next_tok(p);
    char *left  = next_tok(p);
    char *lsz   = next_tok(p);
    char *lip   = next_tok(p);
    char *right = next_tok(p);
    char *rsz   = next_tok(p);
    char *rip   = next_tok(p);
    if (op)    strncpy(c.op,    op,    sizeof(c.op)    - 1);
    if (left)  strncpy(c.left,  left,  sizeof(c.left)  - 1);
    if (right) strncpy(c.right, right, sizeof(c.right) - 1);
    c.left_size    = lsz ? atoi(lsz) : 4;
    c.left_is_ptr  = lip ? (atoi(lip) != 0) : false;
    c.right_size   = rsz ? atoi(rsz) : 4;
    c.right_is_ptr = rip ? (atoi(rip) != 0) : false;
    return c;
}

/* ===== 文リストのパース ===== */
/* 終端トークン: ENDFUNC / ENDLOOP / IFTRUE / IFFALSE / ENDIF */
static int parse_stmts(Parser *p, BStmt *buf, int max_stmts,
                        const char *term1, const char *term2);

static int parse_stmts(Parser *p, BStmt *buf, int max_count,
                        const char *term1, const char *term2)
{
    int n = 0;
    while (n < max_count && next_line(p)) {
        char *kw = p->tok;
        if (!kw) continue;

        /* 終端チェック */
        if ((term1 && strcmp(kw, term1) == 0) ||
            (term2 && strcmp(kw, term2) == 0)) {
            break;
        }

        BStmt *s = &buf[n++];

        if (strcmp(kw, "STORE") == 0) {
            s->kind = STMT_STORE;
            char *dst = next_tok(p);
            char *sz  = next_tok(p);
            char *ip  = next_tok(p);
            if (dst) strncpy(s->dst_name, dst, MAX_NAME - 1);
            s->dst_size   = sz ? atoi(sz) : 4;
            s->dst_is_ptr = ip ? (atoi(ip) != 0) : false;
            s->value = parse_expr(p);

        } else if (strcmp(kw, "INCR") == 0) {
            s->kind = STMT_INCR;
            char *name = next_tok(p);
            char *sz   = next_tok(p);
            char *ip   = next_tok(p);
            char *op   = next_tok(p);
            if (name) strncpy(s->incr_name, name, MAX_NAME - 1);
            s->incr_size   = sz ? atoi(sz) : 4;
            s->incr_is_ptr = ip ? (atoi(ip) != 0) : false;
            s->incr_is_dec = op && strcmp(op, "DEC") == 0;

        } else if (strcmp(kw, "LOOP") == 0) {
            s->kind = STMT_LOOP;
            char *dep = next_tok(p);
            s->loop.depth = dep ? atoi(dep) : 0;
            /* 次行はCOND */
            if (next_line(p) && strcmp(p->tok, "COND") == 0) {
                s->loop.cond = parse_cond(p);
            }
            /* 次行はLOOPBODY */
            if (next_line(p) && strcmp(p->tok, "LOOPBODY") == 0) {
                BStmt *body_buf = alloc_stmts(MAX_STMTS);
                s->loop.body_count = parse_stmts(p, body_buf, MAX_STMTS,
                                                  "ENDLOOP", NULL);
                s->loop.body = body_buf;
            }

        } else if (strcmp(kw, "IF") == 0) {
            s->kind = STMT_IF;
            /* 次行はCOND */
            if (next_line(p) && strcmp(p->tok, "COND") == 0) {
                s->bif.cond = parse_cond(p);
            }
            /* 次行はIFTRUE */
            BStmt *true_buf  = alloc_stmts(MAX_STMTS);
            BStmt *false_buf = alloc_stmts(MAX_STMTS);
            if (next_line(p) && strcmp(p->tok, "IFTRUE") == 0) {
                s->bif.true_count = parse_stmts(p, true_buf, MAX_STMTS,
                                                  "IFFALSE", NULL);
                s->bif.true_stmts = true_buf;
            }
            /* IFFALSEはparse_stmtsが "IFFALSE" で止まって返ってくる */
            s->bif.false_count = parse_stmts(p, false_buf, MAX_STMTS,
                                               "ENDIF", NULL);
            s->bif.false_stmts = false_buf;

        } else if (strcmp(kw, "RET") == 0) {
            s->kind = STMT_RET;
            char *sz = next_tok(p);
            char *ip = next_tok(p);
            s->ret_size   = sz ? atoi(sz) : 4;
            s->ret_is_ptr = ip ? (atoi(ip) != 0) : false;
            s->ret_val = parse_expr(p);

        } else if (strcmp(kw, "RET_VOID") == 0) {
            s->kind = STMT_RET_VOID;

        } else if (strcmp(kw, "ARRSTORE") == 0) {
            s->kind = STMT_ARRSTORE;
            char *name = next_tok(p);
            char *sz   = next_tok(p);
            char *ip   = next_tok(p);
            if (name) strncpy(s->arr_name, name, MAX_NAME - 1);
            s->elem_size   = sz ? atoi(sz) : 4;
            s->elem_is_ptr = ip ? (atoi(ip) != 0) : false;
            s->arr_index = parse_expr(p);
            s->arr_val   = parse_expr(p);

        } else if (strcmp(kw, "BREAK") == 0) {
            s->kind = STMT_BREAK;

        } else if (strcmp(kw, "CONTINUE") == 0) {
            s->kind = STMT_CONTINUE;

        } else {
            /* 未知のキーワード: カウントを戻してスキップ */
            n--;
        }
    }
    return n;
}

/* ===== BIRファイル全体のパース ===== */
BackendProgram *bir_parse(const char *path) {
    FILE *fp = fopen(path, "r");
    if (!fp) {
        fprintf(stderr, "BIRファイルを開けません: %s\n", path);
        return NULL;
    }

    Parser p = {0};
    p.fp = fp;

    /* ヘッダ確認: "BIR 1" */
    if (!next_line(&p) || strcmp(p.tok, "BIR") != 0) {
        fprintf(stderr, "BIRヘッダが不正です\n");
        fclose(fp);
        return NULL;
    }

    BackendProgram *prog = (BackendProgram *)xmalloc(sizeof(BackendProgram));
    prog->funcs      = (BackendFunction *)xmalloc(sizeof(BackendFunction) * 256);
    prog->func_count = 0;

    while (next_line(&p)) {
        if (strcmp(p.tok, "FUNC") != 0) continue;

        BackendFunction *fn = &prog->funcs[prog->func_count++];
        memset(fn, 0, sizeof(*fn));

        /* FUNC <name> <is_public> <return_size> <return_is_ptr> */
        char *name = next_tok(&p);
        char *pub  = next_tok(&p);
        char *rsz  = next_tok(&p);
        char *rip  = next_tok(&p);
        if (name) strncpy(fn->name, name, MAX_NAME - 1);
        fn->is_public     = pub ? (atoi(pub) != 0) : false;
        fn->return_size   = rsz ? atoi(rsz) : 4;
        fn->return_is_ptr = rip ? (atoi(rip) != 0) : false;

        /* PARAM / LOCAL / BODY まで読む */
        bool in_body = false;
        BStmt *body_buf = alloc_stmts(MAX_STMTS);
        int    body_n   = 0;

        while (!in_body && next_line(&p)) {
            char *kw = p.tok;
            if (!kw) continue;

            if (strcmp(kw, "PARAM") == 0) {
                if (fn->param_count < MAX_PARAMS) {
                    BVariable *v = &fn->params[fn->param_count++];
                    char *vname = next_tok(&p);
                    char *vsz   = next_tok(&p);
                    char *vip   = next_tok(&p);
                    if (vname) strncpy(v->name, vname, MAX_NAME - 1);
                    v->size   = vsz ? atoi(vsz) : 4;
                    v->is_ptr = vip ? (atoi(vip) != 0) : false;
                }
            } else if (strcmp(kw, "LOCAL") == 0) {
                if (fn->local_count < MAX_LOCALS) {
                    BVariable *v = &fn->locals[fn->local_count++];
                    char *vname = next_tok(&p);
                    char *vsz   = next_tok(&p);
                    char *vip   = next_tok(&p);
                    if (vname) strncpy(v->name, vname, MAX_NAME - 1);
                    v->size   = vsz ? atoi(vsz) : 4;
                    v->is_ptr = vip ? (atoi(vip) != 0) : false;
                }
            } else if (strcmp(kw, "BODY") == 0) {
                body_n = parse_stmts(&p, body_buf, MAX_STMTS,
                                      "ENDFUNC", NULL);
                in_body = true;
            }
        }

        fn->stmts      = body_buf;
        fn->stmt_count = body_n;
    }

    fclose(fp);
    return prog;
}

/* ===== メモリ解放 ===== */
static void free_expr(BExpr *e) {
    if (!e) return;
    if (e->kind == EXPR_BINOP) {
        free_expr(e->lhs);
        free_expr(e->rhs);
    } else if (e->kind == EXPR_CALL) {
        for (int i = 0; i < e->argc; i++) free_expr(e->args[i]);
    } else if (e->kind == EXPR_CAST || e->kind == EXPR_DEREF) {
        free_expr(e->inner);
    } else if (e->kind == EXPR_ARRLOAD) {
        free_expr(e->index);
    }
    free(e);
}

static void free_stmts(BStmt *stmts, int n) {
    if (!stmts) return;
    for (int i = 0; i < n; i++) {
        BStmt *s = &stmts[i];
        switch (s->kind) {
        case STMT_STORE:
            free_expr(s->value);
            break;
        case STMT_LOOP:
            free_stmts(s->loop.body, s->loop.body_count);
            free(s->loop.body);
            break;
        case STMT_IF:
            free_stmts(s->bif.true_stmts,  s->bif.true_count);
            free_stmts(s->bif.false_stmts, s->bif.false_count);
            free(s->bif.true_stmts);
            free(s->bif.false_stmts);
            break;
        case STMT_RET:
            free_expr(s->ret_val);
            break;
        case STMT_ARRSTORE:
            free_expr(s->arr_index);
            free_expr(s->arr_val);
            break;
        default:
            break;
        }
    }
}

void bir_free(BackendProgram *prog) {
    if (!prog) return;
    for (int i = 0; i < prog->func_count; i++) {
        BackendFunction *fn = &prog->funcs[i];
        free_stmts(fn->stmts, fn->stmt_count);
        free(fn->stmts);
    }
    free(prog->funcs);
    free(prog);
}
