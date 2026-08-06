/*
 * backend.h — Cバックエンド内部データ構造
 *
 * BackendFunctionはAnnotated ASTをCバックエンドが扱える形に変換した
 * 最初の内部表現。
 *
 * 設計原則:
 *   - 型推論しない
 *   - 名前解決しない
 *   - スコープ探索しない
 *   - Analyzerが付与した情報（size / is_ptr / loop_depth）をそのまま使う
 *   - BackendはBIRファイルを読んでBackendFunctionを構築するだけ
 */

#pragma once
#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

/* ===== 文字列の最大長 ===== */
#define MAX_NAME     128
#define MAX_STMTS   4096
#define MAX_PARAMS    32
#define MAX_LOCALS   512
#define MAX_ARGS      16

/* ===== 変数情報（Analyzerが確定済み） ===== */
typedef struct {
    char name[MAX_NAME];
    int  size;      /* バイト数: int=4, String=8, ptr=8, int64=8 */
    bool is_ptr;    /* 64bit幅で扱う（String/ptr/int64） */
} BVariable;

/* ===== 式ノードの種別 ===== */
typedef enum {
    EXPR_LIT_INT   = 1,
    EXPR_LIT_FLOAT = 2,
    EXPR_LIT_STR   = 3,
    EXPR_IDENT     = 4,  /* 変数参照 */
    EXPR_BINOP     = 5,  /* 二項演算 */
    EXPR_CALL      = 6,  /* 関数呼び出し */
    EXPR_CAST      = 7,
    EXPR_ARRLOAD   = 8,  /* 配列要素読み込み */
    EXPR_ADDR      = 9,  /* アドレス取得 */
    EXPR_DEREF     = 10, /* ポインタ参照外し */
} ExprKind;

/* ===== 式ノード（BIRから再構築） ===== */
typedef struct BExpr BExpr;
struct BExpr {
    ExprKind kind;
    int      result_size;    /* Analyzerが付与した結果サイズ */
    bool     result_is_ptr;  /* Analyzerが付与したポインタ属性 */

    /* EXPR_LIT_INT */
    int64_t  int_val;

    /* EXPR_LIT_FLOAT */
    double   float_val;

    /* EXPR_LIT_STR */
    char     str_val[256];

    /* EXPR_IDENT */
    char     ident_name[MAX_NAME];
    int      ident_size;
    bool     ident_is_ptr;

    /* EXPR_BINOP */
    char     op[4];      /* + - * / */
    BExpr   *lhs;
    BExpr   *rhs;

    /* EXPR_CALL */
    char     func_name[MAX_NAME];
    int      call_return_size;
    bool     call_return_is_ptr;
    int      argc;
    BExpr   *args[MAX_ARGS];

    /* EXPR_CAST / EXPR_DEREF */
    BExpr   *inner;

    /* EXPR_ARRLOAD */
    char     arr_name[MAX_NAME];
    int      elem_size;
    bool     elem_is_ptr;
    BExpr   *index;

    /* EXPR_ADDR */
    char     addr_name[MAX_NAME];
};

/* ===== 文ノードの種別 ===== */
typedef enum {
    STMT_STORE    = 1,  /* 変数への代入（Variable宣言/Mutation共通） */
    STMT_INCR     = 2,  /* ++/-- */
    STMT_LOOP     = 3,  /* Loopブロック */
    STMT_IF       = 4,  /* Ifブロック */
    STMT_RET      = 5,  /* return */
    STMT_RET_VOID = 6,  /* return（void） */
    STMT_ARRSTORE = 7,  /* 配列要素書き込み */
    STMT_BREAK    = 8,
    STMT_CONTINUE = 9,
} StmtKind;

/* ===== 条件ノード ===== */
typedef struct {
    char op[16];         /* le / lt / eq / ge / gt / ne */
    char left[MAX_NAME];
    int  left_size;
    bool left_is_ptr;
    char right[MAX_NAME];
    int  right_size;
    bool right_is_ptr;
} BCond;

/* ===== 文ノード（前方宣言） ===== */
typedef struct BStmt BStmt;

/* ===== ループ本体 ===== */
typedef struct {
    int     depth;       /* LoopNode.LoopDepth（Analyzerが付与） */
    BCond   cond;
    BStmt  *body;        /* LOOPBODY内の文リスト */
    int     body_count;
} BLoop;

/* ===== Ifブロック ===== */
typedef struct {
    BCond   cond;
    BStmt  *true_stmts;
    int     true_count;
    BStmt  *false_stmts;
    int     false_count;
} BIf;

/* ===== 文ノード ===== */
struct BStmt {
    StmtKind kind;

    /* STMT_STORE */
    char   dst_name[MAX_NAME];
    int    dst_size;
    bool   dst_is_ptr;
    BExpr *value;

    /* STMT_INCR */
    char   incr_name[MAX_NAME];
    int    incr_size;
    bool   incr_is_ptr;
    bool   incr_is_dec;  /* true: --, false: ++ */

    /* STMT_LOOP */
    BLoop loop;

    /* STMT_IF */
    BIf   bif;

    /* STMT_RET */
    int    ret_size;
    bool   ret_is_ptr;
    BExpr *ret_val;

    /* STMT_ARRSTORE */
    char   arr_name[MAX_NAME];
    int    elem_size;
    bool   elem_is_ptr;
    BExpr *arr_index;
    BExpr *arr_val;
};

/* ===== BackendFunction ===== */
/* Annotated ASTの1関数に対応するBackendの最初の内部表現 */
typedef struct {
    char      name[MAX_NAME];
    bool      is_public;

    /* 引数（Analyzerが付与したParamsから生成） */
    BVariable params[MAX_PARAMS];
    int       param_count;

    /* ローカル変数（FuncNode.LocalVarsから生成） */
    BVariable locals[MAX_LOCALS];
    int       local_count;

    /* 戻り値情報（FuncNode.ReturnAnnから生成） */
    int       return_size;
    bool      return_is_ptr;

    /* 関数Body（文リスト） */
    BStmt    *stmts;
    int       stmt_count;
} BackendFunction;

/* ===== プログラム全体 ===== */
typedef struct {
    BackendFunction *funcs;
    int              func_count;
} BackendProgram;

/* ===== 関数宣言 ===== */
BackendProgram *bir_parse(const char *path);
void            bir_free(BackendProgram *prog);
void            bir_dump(const BackendProgram *prog);  /* デバッグ出力 */
