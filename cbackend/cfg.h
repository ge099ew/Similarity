/*
 * cfg.h — CFG (Control Flow Graph) データ構造
 *
 * 入力: BackendFunction（BIRパーサが生成済み）
 * 出力: CFG（CFGBlock のポインタグラフ）
 *
 * 設計原則:
 *   - BackendFunctionの情報（size/is_ptr/loop_depth/cond）をそのまま使う
 *   - 型推論・名前解決は行わない（Analyzerが保証済み）
 *   - BIRのSTMT_LOOP.depth をそのままブロックのloop_depthとして使う
 */

#pragma once
#include <stdbool.h>
#include "backend.h"

/* ===== ブロック種別 ===== */
typedef enum {
    BLOCK_ENTRY,       /* 関数エントリ */
    BLOCK_NORMAL,      /* 通常（通常文の連続） */
    BLOCK_IF,          /* If条件評価 */
    BLOCK_IF_TRUE,     /* Ifの真ブロック */
    BLOCK_IF_FALSE,    /* Ifの偽ブロック */
    BLOCK_MERGE,       /* If合流点 */
    BLOCK_LOOP_HEADER, /* Loop条件評価（ヘッダ） */
    BLOCK_LOOP_BODY,   /* Loopボディ */
    BLOCK_LOOP_EXIT,   /* Loop脱出後 */
    BLOCK_RETURN,      /* Return終端 */
} BlockKind;

/* ===== CFGBlock ===== */
#define MAX_SUCCS  4
#define MAX_PREDS  8
#define MAX_BLOCK_STMTS 1024

typedef struct CFGBlock CFGBlock;

struct CFGBlock {
    int       id;
    BlockKind kind;
    int       loop_depth; /* STMT_LOOP.depth から設定（Analyzerが付与済み） */

    /* 条件（BLOCK_IF / BLOCK_LOOP_HEADER のみ） */
    bool      has_cond;
    BCond     cond;

    /* 後継・前任ブロック */
    CFGBlock *succs[MAX_SUCCS];
    int       nsuccs;
    CFGBlock *preds[MAX_PREDS];
    int       npreds;

    /* このブロックに属する文（STMT_LOOP/STMT_IF は展開されるので含まない） */
    /* ポインタ: BackendFunctionのstmts配列内を指す（コピーしない） */
    const BStmt *stmts[MAX_BLOCK_STMTS];
    int          nstmts;

    /* バックエッジフラグ（Loopで Body→Header に使う） */
    bool is_back_edge[MAX_SUCCS];
};

/* ===== CFG ===== */
#define MAX_BLOCKS 4096

typedef struct {
    const BackendFunction *func;       /* 元のBackendFunction（変更しない） */
    CFGBlock              *blocks[MAX_BLOCKS];
    int                    nblocks;
    CFGBlock              *entry;
    CFGBlock              *exit_block; /* exitという名前はC予約語と衝突するため */
} CFG;

/* ===== API ===== */
CFG  *cfg_build(const BackendFunction *func);
void  cfg_free(CFG *cfg);
void  cfg_dump(const CFG *cfg);

/* Program単位 */
typedef struct {
    CFG **cfgs;
    int   ncfgs;
} CFGProgram;

CFGProgram *cfg_build_program(const BackendProgram *prog);
void        cfg_program_free(CFGProgram *p);
void        cfg_program_dump(const CFGProgram *p);
