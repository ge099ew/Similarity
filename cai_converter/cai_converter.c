/*
 * CAI Converter - CAIテキスト形式 → x86_64 ELFバイナリ
 * Similarity言語のバックエンド（踏み台・一回限り）
 * 最終的にアセンブリ製APE形式に置き換える
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <ctype.h>

/* ===== 定数 ===== */
#define MAX_INSTRS    65536
#define MAX_FUNCS     1024
#define MAX_REGS      4096
#define MAX_LABELS    4096
#define MAX_NAME      128
#define MAX_ARGS      16
#define MAX_CODE_SIZE (1024 * 1024)  /* 1MB */

/* ===== 命令種別 ===== */
typedef enum {
    OP_ALLOC, OP_STORE, OP_STOREP, OP_STOREF,
    OP_LOAD,  OP_LOADP, OP_LOADF,
    OP_ADD,   OP_SUB,   OP_MUL,   OP_DIV,
    OP_ADDF,  OP_SUBF,
    OP_CLT,   OP_CLE,   OP_CEQ,   OP_CNE,   OP_CGT,   OP_CGE,
    OP_LABEL, OP_JMP,   OP_JNZ,
    OP_CALL,  OP_RET,   OP_RETV,
    OP_ITOF,  OP_FTOI,
    OP_MOV,
    OP_EXTERN,
    OP_FUNC,  OP_ENDFUNC,
    OP_COMMENT,
} OpKind;

/* ===== 命令 ===== */
typedef struct {
    OpKind  kind;
    char    dst[MAX_NAME];
    char    a[MAX_NAME];
    char    b[MAX_NAME];
    char    args[MAX_ARGS][MAX_NAME];
    int     argc;
    int     is_export;
} Instr;

/* ===== レジスタマップ（仮想→スタックオフセット） ===== */
typedef struct {
    char name[MAX_NAME];
    int  offset;   /* RBPからのオフセット（負値） */
    int  size;
} RegSlot;

/* ===== ラベルマップ ===== */
typedef struct {
    char name[MAX_NAME];
    int  code_offset;   /* コードバッファ内のオフセット */
} LabelDef;

/* ===== 関数情報 ===== */
typedef struct {
    char name[MAX_NAME];
    int  instr_start;
    int  instr_end;
    int  is_export;
} FuncInfo;

/* ===== グローバル状態 ===== */
static Instr    instrs[MAX_INSTRS];
static int      instr_count = 0;

static FuncInfo funcs[MAX_FUNCS];
static int      func_count = 0;

static uint8_t  code[MAX_CODE_SIZE];
static int      code_size = 0;

/* ===== ユーティリティ ===== */
static void trim(char *s) {
    char *p = s;
    while (*p == ' ' || *p == '\t') p++;
    memmove(s, p, strlen(p) + 1);
    int len = strlen(s);
    while (len > 0 && (s[len-1] == ' ' || s[len-1] == '\t' || s[len-1] == '\r' || s[len-1] == '\n'))
        s[--len] = '\0';
}

/* ===== パーサー ===== */
static void parse_line(char *line) {
    trim(line);
    if (line[0] == '#' || line[0] == '\0') {
        instrs[instr_count].kind = OP_COMMENT;
        instr_count++;
        return;
    }

    Instr *ins = &instrs[instr_count];
    memset(ins, 0, sizeof(Instr));

    char *tok = strtok(line, " \t");
    if (!tok) return;

    if (strcmp(tok, "func") == 0 || strcmp(tok, "export") == 0) {
        ins->is_export = (strcmp(tok, "export") == 0);
        if (ins->is_export) tok = strtok(NULL, " \t"); /* skip "func" */
        tok = strtok(NULL, " \t");
        if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
        ins->kind = OP_FUNC;
    } else if (strcmp(tok, "endfunc") == 0) {
        ins->kind = OP_ENDFUNC;
    } else if (strcmp(tok, "alloc") == 0) {
        ins->kind = OP_ALLOC;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) ins->argc = atoi(tok);
    } else if (strcmp(tok, "store") == 0) {
        ins->kind = OP_STORE;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->a, tok, MAX_NAME-1);
    } else if (strcmp(tok, "load") == 0) {
        ins->kind = OP_LOAD;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->a, tok, MAX_NAME-1);
    } else if (strcmp(tok, "add") == 0 || strcmp(tok, "sub") == 0 ||
               strcmp(tok, "mul") == 0 || strcmp(tok, "div") == 0) {
        ins->kind = (strcmp(tok,"add")==0)?OP_ADD:(strcmp(tok,"sub")==0)?OP_SUB:
                    (strcmp(tok,"mul")==0)?OP_MUL:OP_DIV;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->a, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->b, tok, MAX_NAME-1);
    } else if (strcmp(tok, "clt")==0 || strcmp(tok, "cle")==0 ||
               strcmp(tok, "ceq")==0 || strcmp(tok, "cne")==0 ||
               strcmp(tok, "cgt")==0 || strcmp(tok, "cge")==0) {
        ins->kind = (strcmp(tok,"clt")==0)?OP_CLT:(strcmp(tok,"cle")==0)?OP_CLE:
                    (strcmp(tok,"ceq")==0)?OP_CEQ:(strcmp(tok,"cne")==0)?OP_CNE:
                    (strcmp(tok,"cgt")==0)?OP_CGT:OP_CGE;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->a, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->b, tok, MAX_NAME-1);
    } else if (strcmp(tok, "label") == 0) {
        ins->kind = OP_LABEL;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
    } else if (strcmp(tok, "jmp") == 0) {
        ins->kind = OP_JMP;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
    } else if (strcmp(tok, "jnz") == 0) {
        ins->kind = OP_JNZ;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->a, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->b, tok, MAX_NAME-1);
    } else if (strcmp(tok, "call") == 0) {
        ins->kind = OP_CALL;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->a, tok, MAX_NAME-1);
        ins->argc = 0;
        while ((tok = strtok(NULL, " \t")) && ins->argc < MAX_ARGS) {
            strncpy(ins->args[ins->argc++], tok, MAX_NAME-1);
        }
    } else if (strcmp(tok, "ret") == 0) {
        tok = strtok(NULL, " \t");
        if (tok) {
            ins->kind = OP_RET;
            strncpy(ins->dst, tok, MAX_NAME-1);
        } else {
            ins->kind = OP_RETV;
        }
    } else if (strcmp(tok, "itof") == 0) {
        ins->kind = OP_ITOF;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->a, tok, MAX_NAME-1);
    } else if (strcmp(tok, "ftoi") == 0) {
        ins->kind = OP_FTOI;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->a, tok, MAX_NAME-1);
    } else if (strcmp(tok, "mov") == 0) {
        ins->kind = OP_MOV;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->a, tok, MAX_NAME-1);
    } else if (strcmp(tok, "extern") == 0) {
        ins->kind = OP_EXTERN;
        tok = strtok(NULL, " \t"); if (tok) strncpy(ins->dst, tok, MAX_NAME-1);
    } else {
        ins->kind = OP_COMMENT;
    }
    instr_count++;
}

static void parse_file(const char *path) {
    FILE *f = fopen(path, "r");
    if (!f) { perror("fopen"); exit(1); }
    char line[512];
    while (fgets(line, sizeof(line), f)) {
        parse_line(line);
    }
    fclose(f);
}

/* ===== x86_64 コード生成 ===== */
/* 注: Phase1では GCCを呼び出してアセンブリ経由でバイナリ生成 */
/* Phase2で直接バイナリ生成に移行 */

static void emit_asm_header(FILE *f) {
    fprintf(f, ".intel_syntax noprefix\n");
    fprintf(f, ".text\n\n");
    /* スタックが実行不可であることをリンカに伝える */
    fprintf(f, ".section .note.GNU-stack,\"\",@progbits\n");
    fprintf(f, ".text\n\n");
}

/* レジスタ割り当て（仮想→物理、簡易版） */
/* スタックフレームを使う。全仮想レジスタはスタック上のスロット */
typedef struct {
    char     name[MAX_NAME];
    int      stack_off;  /* RBPからの負オフセット */
} VReg;

static VReg   vregs[MAX_REGS];
static int    vreg_count;
static int    stack_size;  /* 現在の関数のスタックサイズ */

static LabelDef labels[MAX_LABELS];
static int      label_count;

static int find_vreg(const char *name) {
    for (int i = 0; i < vreg_count; i++)
        if (strcmp(vregs[i].name, name) == 0) return i;
    return -1;
}

static int alloc_vreg(const char *name, int size) {
    int i = find_vreg(name);
    if (i >= 0) return i;
    stack_size += size;
    vregs[vreg_count].stack_off = -stack_size;
    strncpy(vregs[vreg_count].name, name, MAX_NAME-1);
    return vreg_count++;
}

/* 値をEAXに読み込む */
static void load_to_eax(FILE *f, const char *val) {
    if (val[0] == '%') {
        /* 仮想レジスタ */
        int i = find_vreg(val);
        if (i >= 0)
            fprintf(f, "  mov eax, dword ptr [rbp%+d]\n", vregs[i].stack_off);
        else
            fprintf(f, "  xor eax, eax  # undef reg %s\n", val);
    } else if (isdigit(val[0]) || (val[0] == '-' && isdigit(val[1]))) {
        fprintf(f, "  mov eax, %s\n", val);
    } else if (val[0] == '$') {
        /* 関数ポインタ — 今は使わない */
        fprintf(f, "  lea rax, [rip + %s]\n", val+1);
    } else {
        fprintf(f, "  mov eax, 0  # unknown val %s\n", val);
    }
}

/* 値をRAXに読み込む（64bit） */
static void load_to_rax(FILE *f, const char *val) {
    if (val[0] == '%') {
        int i = find_vreg(val);
        if (i >= 0)
            fprintf(f, "  mov rax, qword ptr [rbp%+d]\n", vregs[i].stack_off);
        else
            fprintf(f, "  xor rax, rax\n");
    } else {
        fprintf(f, "  mov rax, %s\n", val);
    }
}

/* EAXの値を仮想レジスタに保存 */
static void store_eax(FILE *f, const char *dst) {
    int i = find_vreg(dst);
    if (i < 0) i = alloc_vreg(dst, 8);
    fprintf(f, "  mov dword ptr [rbp%+d], eax\n", vregs[i].stack_off);
}

/* RAXの値を仮想レジスタに保存（64bit） */
static void store_rax(FILE *f, const char *dst) {
    int i = find_vreg(dst);
    if (i < 0) i = alloc_vreg(dst, 8);
    fprintf(f, "  mov qword ptr [rbp%+d], rax\n", vregs[i].stack_off);
}

static const char *call_regs[] = {"rdi", "rsi", "rdx", "rcx", "r8", "r9"};

static void gen_func_asm(FILE *f, int fi) {
    FuncInfo *fn = &funcs[fi];
    vreg_count = 0;
    stack_size = 0;
    label_count = 0;

    /* 引数をスタックに仮確保（後でpreludeで調整） */
    /* まず全命令をスキャンしてスタックサイズを確定する必要があるが、
       簡易版では大きめに確保 */
    
    /* 関数名 */
    char fname_buf[MAX_NAME];
    const char *fname = fn->name;
    if (fname[0] == '$') fname++;
    /* main → sim_main に変換（エントリーポイントはラッパーが担当） */
    if (strcmp(fname, "main") == 0) {
        strncpy(fname_buf, "sim_main", MAX_NAME-1);
        fname = fname_buf;
        fn->is_export = 1;
    } else {
        strncpy(fname_buf, fname, MAX_NAME-1);
        fname = fname_buf;
    }
    
    if (fn->is_export) {
        fprintf(f, ".global %s\n", fname);
        fprintf(f, ".global sim_main\n");
    } else {
        fprintf(f, ".local %s\n", fname);
    }
    fprintf(f, "%s:\n", fname);

    /* プロローグ */
    fprintf(f, "  push rbp\n");
    fprintf(f, "  mov rbp, rsp\n");
    fprintf(f, "  sub rsp, 512\n");  /* 簡易版: 固定サイズ確保 */

    /* 引数レジスタをスタックに退避（最大6引数） */
    /* 引数名は arg0, arg1, ... として扱う */
    for (int i = 0; i < 6; i++) {
        char argname[MAX_NAME];
        snprintf(argname, MAX_NAME, "%%arg%d", i);
        int idx = alloc_vreg(argname, 8);
        fprintf(f, "  mov qword ptr [rbp%+d], %s\n",
                vregs[idx].stack_off, call_regs[i]);
    }

    /* 命令生成 */
    for (int i = fn->instr_start; i < fn->instr_end; i++) {
        Instr *ins = &instrs[i];
        switch (ins->kind) {
        case OP_COMMENT:
            break;

        case OP_ALLOC: {
            /* スタック上にスロット確保 */
            int idx = alloc_vreg(ins->dst, ins->argc > 0 ? ins->argc : 8);
            fprintf(f, "  lea rax, [rbp%+d]\n", vregs[idx].stack_off);
            /* .ptrという名前で保存 */
            char ptrname[MAX_NAME];
            snprintf(ptrname, MAX_NAME, "%s_addr", ins->dst);
            int pidx = alloc_vreg(ptrname, 8);
            fprintf(f, "  mov qword ptr [rbp%+d], rax\n", vregs[pidx].stack_off);
            break;
        }

        case OP_STORE: {
            /* store %ptr %val → [ptr] = val */
            /* ptrはアドレス（仮想レジスタ） */
            load_to_eax(f, ins->a);
            fprintf(f, "  mov ecx, eax\n");
            /* dstはptr名 → その_addrを使う */
            char ptrname[MAX_NAME];
            snprintf(ptrname, MAX_NAME, "%s_addr", ins->dst);
            int pidx = find_vreg(ptrname);
            if (pidx >= 0) {
                fprintf(f, "  mov rax, qword ptr [rbp%+d]\n", vregs[pidx].stack_off);
                fprintf(f, "  mov dword ptr [rax], ecx\n");
            } else {
                /* fallback: 直接スロットに書く */
                int idx = find_vreg(ins->dst);
                if (idx >= 0)
                    fprintf(f, "  mov dword ptr [rbp%+d], ecx\n", vregs[idx].stack_off);
            }
            break;
        }

        case OP_LOAD: {
            /* load %dst %ptr → dst = [ptr] */
            char ptrname[MAX_NAME];
            snprintf(ptrname, MAX_NAME, "%s_addr", ins->a);
            int pidx = find_vreg(ptrname);
            if (pidx >= 0) {
                fprintf(f, "  mov rax, qword ptr [rbp%+d]\n", vregs[pidx].stack_off);
                fprintf(f, "  mov eax, dword ptr [rax]\n");
            } else {
                int idx = find_vreg(ins->a);
                if (idx >= 0)
                    fprintf(f, "  mov eax, dword ptr [rbp%+d]\n", vregs[idx].stack_off);
                else
                    fprintf(f, "  xor eax, eax\n");
            }
            store_eax(f, ins->dst);
            break;
        }

        case OP_ADD:
            load_to_eax(f, ins->a);
            fprintf(f, "  mov ecx, eax\n");
            load_to_eax(f, ins->b);
            fprintf(f, "  add eax, ecx\n");
            store_eax(f, ins->dst);
            break;

        case OP_SUB:
            load_to_eax(f, ins->a);
            fprintf(f, "  mov ecx, eax\n");
            load_to_eax(f, ins->b);
            fprintf(f, "  sub ecx, eax\n");
            fprintf(f, "  mov eax, ecx\n");
            store_eax(f, ins->dst);
            break;

        case OP_MUL:
            load_to_eax(f, ins->a);
            fprintf(f, "  mov ecx, eax\n");
            load_to_eax(f, ins->b);
            fprintf(f, "  imul eax, ecx\n");
            store_eax(f, ins->dst);
            break;

        case OP_DIV:
            load_to_eax(f, ins->a);
            fprintf(f, "  mov ecx, eax\n");
            load_to_eax(f, ins->b);
            fprintf(f, "  xchg eax, ecx\n");
            fprintf(f, "  cdq\n");
            fprintf(f, "  idiv ecx\n");
            store_eax(f, ins->dst);
            break;

        case OP_CLT: case OP_CLE: case OP_CEQ:
        case OP_CNE: case OP_CGT: case OP_CGE: {
            load_to_eax(f, ins->a);
            fprintf(f, "  mov ecx, eax\n");
            load_to_eax(f, ins->b);
            fprintf(f, "  cmp ecx, eax\n");
            const char *setcc =
                ins->kind==OP_CLT?"setl":ins->kind==OP_CLE?"setle":
                ins->kind==OP_CEQ?"sete":ins->kind==OP_CNE?"setne":
                ins->kind==OP_CGT?"setg":"setge";
            fprintf(f, "  %s al\n", setcc);
            fprintf(f, "  movzx eax, al\n");
            store_eax(f, ins->dst);
            break;
        }

        case OP_LABEL:
            fprintf(f, ".L_%s_%s:\n", fname, ins->dst);
            break;

        case OP_JMP:
            fprintf(f, "  jmp .L_%s_%s\n", fname, ins->dst);
            break;

        case OP_JNZ:
            load_to_eax(f, ins->dst);
            fprintf(f, "  test eax, eax\n");
            fprintf(f, "  jnz .L_%s_%s\n", fname, ins->a);
            fprintf(f, "  jmp .L_%s_%s\n", fname, ins->b);
            break;

        case OP_CALL: {
            /* 引数をレジスタに設定 */
            for (int j = 0; j < ins->argc && j < 6; j++) {
                load_to_eax(f, ins->args[j]);
                fprintf(f, "  movsx %s, eax\n", call_regs[j]);
            }
            const char *callee = ins->a;
            if (callee[0] == '$') callee++;
            fprintf(f, "  call %s\n", callee);
            if (ins->dst[0] && ins->dst[0] != '_') {
                store_eax(f, ins->dst);
            }
            break;
        }

        case OP_RET:
            load_to_eax(f, ins->dst);
            fprintf(f, "  movsx rax, eax\n");
            fprintf(f, "  leave\n");
            fprintf(f, "  ret\n");
            break;

        case OP_RETV:
            fprintf(f, "  xor eax, eax\n");
            fprintf(f, "  leave\n");
            fprintf(f, "  ret\n");
            break;

        case OP_MOV:
            load_to_rax(f, ins->a);
            store_rax(f, ins->dst);
            break;

        default:
            fprintf(f, "  # unhandled op %d\n", ins->kind);
            break;
        }
    }

    fprintf(f, "\n");
}

/* ===== main関数のラッパー生成 ===== */
static void emit_main_wrapper(FILE *f) {
    fprintf(f, "# main wrapper\n");
    fprintf(f, ".global main\n");
    fprintf(f, "main:\n");
    fprintf(f, "  push rbp\n");
    fprintf(f, "  mov rbp, rsp\n");
    fprintf(f, "  sub rsp, 32\n");
    /* timespec for clock_gettime */
    fprintf(f, "  sub rsp, 32\n");
    fprintf(f, "  lea rdi, [rbp-16]\n");
    fprintf(f, "  mov esi, 1\n");  /* CLOCK_MONOTONIC */
    fprintf(f, "  call clock_gettime\n");
    fprintf(f, "  call sim_main\n");
    fprintf(f, "  mov rbx, rax\n");
    fprintf(f, "  lea rdi, [rbp-32]\n");
    fprintf(f, "  mov esi, 1\n");
    fprintf(f, "  call clock_gettime\n");
    /* 時間計算は省略、結果だけ表示 */
    fprintf(f, "  mov rsi, rbx\n");
    fprintf(f, "  lea rdi, [rip + .Lresult_fmt]\n");
    fprintf(f, "  xor eax, eax\n");
    fprintf(f, "  call printf\n");
    fprintf(f, "  xor eax, eax\n");
    fprintf(f, "  leave\n");
    fprintf(f, "  ret\n");
    fprintf(f, ".Lresult_fmt:\n");
    fprintf(f, "  .string \"Similarity result: %%ld\\n\"\n");
}

/* ===== メイン ===== */
int main(int argc, char *argv[]) {
    if (argc < 3) {
        fprintf(stderr, "Usage: cai_converter <input.cai> <output>\n");
        return 1;
    }

    const char *input  = argv[1];
    const char *output = argv[2];

    /* パース */
    parse_file(input);

    /* 関数情報収集 */
    int cur_func = -1;
    for (int i = 0; i < instr_count; i++) {
        if (instrs[i].kind == OP_FUNC) {
            cur_func = func_count++;
            strncpy(funcs[cur_func].name, instrs[i].dst, MAX_NAME-1);
            funcs[cur_func].is_export = instrs[i].is_export;
            funcs[cur_func].instr_start = i + 1;
        } else if (instrs[i].kind == OP_ENDFUNC && cur_func >= 0) {
            funcs[cur_func].instr_end = i;
            cur_func = -1;
        }
    }

    /* アセンブリ出力 */
    char asm_file[512];
    snprintf(asm_file, sizeof(asm_file), "%s.s", output);
    FILE *f = fopen(asm_file, "w");
    if (!f) { perror("fopen asm"); return 1; }

    emit_asm_header(f);

    for (int i = 0; i < func_count; i++) {
        gen_func_asm(f, i);
    }

    emit_main_wrapper(f);

    fclose(f);

    /* GCCでバイナリ生成 */
    char cmd[1024];
    snprintf(cmd, sizeof(cmd),
        "gcc -no-pie -o %s %s -lc 2>&1", output, asm_file);
    int ret = system(cmd);
    if (ret != 0) {
        fprintf(stderr, "アセンブル/リンクに失敗しました\n");
        return 1;
    }

    printf("Binary → %s ✅\n", output);
    return 0;
}
