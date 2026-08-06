/*
 * bir_dump.c — BackendProgramの内容をデバッグ出力する。
 * --dump-cfg オプションで使用（Stage 3以降はCFG情報を出力する）。
 */

#include <stdio.h>
#include <string.h>
#include "backend.h"

static void dump_expr(const BExpr *e, int depth);
static void dump_stmts(const BStmt *stmts, int n, int depth);

static void indent(int d) {
    for (int i = 0; i < d * 2; i++) putchar(' ');
}

static void dump_expr(const BExpr *e, int depth) {
    if (!e) { printf("(nil)"); return; }
    switch (e->kind) {
    case EXPR_LIT_INT:
        printf("LIT_INT(%lld)", (long long)e->int_val);
        break;
    case EXPR_LIT_FLOAT:
        printf("LIT_FLOAT(%g)", e->float_val);
        break;
    case EXPR_LIT_STR:
        printf("LIT_STR(%s)", e->str_val);
        break;
    case EXPR_IDENT:
        printf("IDENT(%s sz=%d ptr=%d)", e->ident_name, e->ident_size, e->ident_is_ptr);
        break;
    case EXPR_BINOP:
        printf("BINOP(%s sz=%d ptr=%d [", e->op, e->result_size, e->result_is_ptr);
        dump_expr(e->lhs, depth);
        printf("] [");
        dump_expr(e->rhs, depth);
        printf("])");
        break;
    case EXPR_CALL:
        printf("CALL(%s rsz=%d rptr=%d argc=%d",
               e->func_name, e->call_return_size, e->call_return_is_ptr, e->argc);
        for (int i = 0; i < e->argc; i++) {
            printf(" [");
            dump_expr(e->args[i], depth);
            printf("]");
        }
        printf(")");
        break;
    case EXPR_CAST:
        printf("CAST(sz=%d ptr=%d [", e->result_size, e->result_is_ptr);
        dump_expr(e->inner, depth);
        printf("])");
        break;
    case EXPR_ARRLOAD:
        printf("ARRLOAD(%s esz=%d [", e->arr_name, e->elem_size);
        dump_expr(e->index, depth);
        printf("])");
        break;
    case EXPR_ADDR:
        printf("ADDR(%s)", e->addr_name);
        break;
    case EXPR_DEREF:
        printf("DEREF(sz=%d [", e->result_size);
        dump_expr(e->inner, depth);
        printf("])");
        break;
    default:
        printf("EXPR_?(%d)", e->kind);
    }
}

static void dump_cond(const BCond *c, int depth) {
    indent(depth);
    printf("COND %s  left=%s(sz=%d ptr=%d)  right=%s(sz=%d ptr=%d)\n",
           c->op,
           c->left,  c->left_size,  c->left_is_ptr,
           c->right, c->right_size, c->right_is_ptr);
}

static void dump_stmts(const BStmt *stmts, int n, int depth) {
    for (int i = 0; i < n; i++) {
        const BStmt *s = &stmts[i];
        switch (s->kind) {
        case STMT_STORE:
            indent(depth);
            printf("STORE %s(sz=%d ptr=%d) = ", s->dst_name, s->dst_size, s->dst_is_ptr);
            dump_expr(s->value, depth);
            printf("\n");
            break;
        case STMT_INCR:
            indent(depth);
            printf("INCR %s(sz=%d ptr=%d) %s\n",
                   s->incr_name, s->incr_size, s->incr_is_ptr,
                   s->incr_is_dec ? "--" : "++");
            break;
        case STMT_LOOP:
            indent(depth);
            printf("LOOP depth=%d\n", s->loop.depth);
            dump_cond(&s->loop.cond, depth + 1);
            indent(depth);
            printf("LOOPBODY (%d stmts)\n", s->loop.body_count);
            dump_stmts(s->loop.body, s->loop.body_count, depth + 2);
            indent(depth);
            printf("ENDLOOP\n");
            break;
        case STMT_IF:
            indent(depth);
            printf("IF\n");
            dump_cond(&s->bif.cond, depth + 1);
            indent(depth + 1);
            printf("TRUE (%d stmts)\n", s->bif.true_count);
            dump_stmts(s->bif.true_stmts, s->bif.true_count, depth + 2);
            indent(depth + 1);
            printf("FALSE (%d stmts)\n", s->bif.false_count);
            dump_stmts(s->bif.false_stmts, s->bif.false_count, depth + 2);
            indent(depth);
            printf("ENDIF\n");
            break;
        case STMT_RET:
            indent(depth);
            printf("RET(sz=%d ptr=%d) ", s->ret_size, s->ret_is_ptr);
            dump_expr(s->ret_val, depth);
            printf("\n");
            break;
        case STMT_RET_VOID:
            indent(depth);
            printf("RET_VOID\n");
            break;
        case STMT_ARRSTORE:
            indent(depth);
            printf("ARRSTORE %s[", s->arr_name);
            dump_expr(s->arr_index, depth);
            printf("] = ");
            dump_expr(s->arr_val, depth);
            printf("\n");
            break;
        case STMT_BREAK:
            indent(depth);
            printf("BREAK\n");
            break;
        case STMT_CONTINUE:
            indent(depth);
            printf("CONTINUE\n");
            break;
        default:
            indent(depth);
            printf("STMT_?(%d)\n", s->kind);
        }
    }
}

void bir_dump(const BackendProgram *prog) {
    if (!prog) return;
    printf("===== BackendProgram (%d functions) =====\n\n", prog->func_count);
    for (int i = 0; i < prog->func_count; i++) {
        const BackendFunction *fn = &prog->funcs[i];
        printf("--- Function: %s%s ---\n", fn->name, fn->is_public ? " [public]" : "");
        printf("  return: size=%d  ptr=%d\n", fn->return_size, fn->return_is_ptr);

        printf("  params (%d):\n", fn->param_count);
        for (int j = 0; j < fn->param_count; j++) {
            printf("    %s  size=%d  ptr=%d\n",
                   fn->params[j].name, fn->params[j].size, fn->params[j].is_ptr);
        }

        printf("  locals (%d):\n", fn->local_count);
        for (int j = 0; j < fn->local_count; j++) {
            printf("    %s  size=%d  ptr=%d\n",
                   fn->locals[j].name, fn->locals[j].size, fn->locals[j].is_ptr);
        }

        printf("  body (%d stmts):\n", fn->stmt_count);
        dump_stmts(fn->stmts, fn->stmt_count, 2);
        printf("\n");
    }
}
