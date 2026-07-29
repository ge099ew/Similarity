/*
 * CAI Converter v6 - CAIテキスト → x86_64機械語直接生成 → ELF実行ファイル
 * asを完全排除。GCC不要。
 *
 * v4変更点:
 *   - グローバル変数を CaiContext 構造体へ集約
 *   - Program Header 管理を PhdrList で構造化
 *   - align_up() 共通関数化
 *   - chmod を fchmod に変更
 *   - 未解決シンボルは致命エラー
 *
 * v5変更点:
 *   - 静的PIE対応（ET_DYN + ロードベース0x0）
 *
 * v6変更点（PIE完全化）:
 *   - PT_PHDR 追加（プログラムヘッダの正規配置）
 *   - PT_GNU_STACK 追加（NX有効・スタック実行禁止）
 *   - PT_DYNAMIC + .dynamic セクション追加（ET_DYNの正規化）
 *   - セクションヘッダ追加（.text/.rodata/.dynamic/.shstrtab）
 *     → readelf/objdump/gdb でデバッグ可能
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <ctype.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/stat.h>

/* ===== 定数 ===== */
#define MAX_INSTRS   65536
#define MAX_FUNCS    1024
#define MAX_REGS     4096
#define MAX_NAME     128
#define MAX_ARGS     16
#define MAX_LABELS   4096
#define MAX_PATCHES  8192
#define CODE_MAX     (4*1024*1024)
#define RODATA_MAX   (1*1024*1024)
#define MAX_PHDRS    16   /* Program Headerの最大数 */

/* ===== アラインメント共通関数 ===== */
static uint64_t align_up(uint64_t value, uint64_t alignment){
    return (value + alignment - 1) & ~(alignment - 1);
}

/* ===== 命令種別 ===== */
typedef enum {
    OP_ALLOC, OP_STORE, OP_LOAD,
    OP_ADD, OP_SUB, OP_MUL, OP_DIV,
    OP_CLT, OP_CLE, OP_CEQ, OP_CNE, OP_CGT, OP_CGE,
    OP_LABEL, OP_JMP, OP_JNZ,
    OP_CALL, OP_RET, OP_RETV,
    OP_ITOF, OP_FTOI, OP_MOV,
    OP_EXTERN, OP_FUNC, OP_ENDFUNC,
    OP_DATA,
    OP_SYSCALL, /* syscall %dst <nr> <arg0> <arg1> <arg2> */
    OP_LOADB,   /* loadb  %dst %ptr      — 1バイトをゼロ拡張でロード */
    OP_STOREB,  /* storeb %ptr %val      — 1バイトをストア（val の下位8bit） */
    OP_ADDP,    /* addp   %dst %ptr %off — ポインタ(64bit)にオフセット(32bit)を加算 */
    OP_STOREP,  /* storep %ptr %val      — 8バイト(64bit)をストア */
    OP_LOADP2,  /* loadp2 %dst %ptr      — 8バイト(64bit)をロード（loadpはCAI既存名と競合回避） */
    /* ===== i64演算 ===== */
    OP_ADD64,   /* add64 %dst %a %b  — 64bit加算 */
    OP_SUB64,   /* sub64 %dst %a %b  — 64bit減算 */
    OP_MUL64,   /* mul64 %dst %a %b  — 64bit乗算 */
    OP_DIV64,   /* div64 %dst %a %b  — 64bit符号付き除算 */
    OP_CLT64,   /* clt64 %dst %a %b  — 64bit比較 a < b */
    OP_CLE64,   /* cle64 %dst %a %b  — 64bit比較 a <= b */
    OP_CEQ64,   /* ceq64 %dst %a %b  — 64bit比較 a == b */
    OP_CNE64,   /* cne64 %dst %a %b  — 64bit比較 a != b */
    OP_CGT64,   /* cgt64 %dst %a %b  — 64bit比較 a > b */
    OP_CGE64,   /* cge64 %dst %a %b  — 64bit比較 a >= b */
    OP_MOV64,   /* mov64 %dst %src   — 64bitコピー */
    /* ===== f32演算（SSE2） ===== */
    OP_ADDF,    /* addf %dst %a %b   — f32加算 (addss) */
    OP_SUBF,    /* subf %dst %a %b   — f32減算 (subss) */
    OP_MULF,    /* mulf %dst %a %b   — f32乗算 (mulss) */
    OP_DIVF,    /* divf %dst %a %b   — f32除算 (divss) */
    OP_ITOF2,   /* itof2 %dst %src   — i32→f32 (cvtsi2ss) */
    OP_FTOI2,   /* ftoi2 %dst %src   — f32→i32 (cvttss2si, 切り捨て) */
    OP_COMMENT,
} OpKind;

typedef struct {
    OpKind kind;
    char   dst[MAX_NAME];
    char   a[MAX_NAME];
    char   b[MAX_NAME];
    char   args[MAX_ARGS][MAX_NAME];
    int    argc;
    int    is_export;
    char   str_val[512];
} Instr;

typedef struct {
    char name[MAX_NAME];
    int  instr_start, instr_end;
    int  is_export, is_leaf, param_count, stack_size;
} FuncInfo;

/* ===== レジスタ割り当て ===== */
#define NUM_ALLOC_REGS 5
static const int alloc_phys[NUM_ALLOC_REGS] = {3, 12, 13, 14, 15};

typedef struct {
    char name[MAX_NAME];
    int  stack_off;
    int  phys_reg;
    int  use_count;
    int  is_ptr;
    int  is_float; /* f32変数: 物理レジスタ割り当て禁止 */
} VReg;

/* ===== シンボル/パッチ/ラベル ===== */
typedef struct { char name[MAX_NAME]; int off; int defined; int global; } Sym;
typedef struct { int code_off; char sym[MAX_NAME]; } Patch;
typedef struct { char name[MAX_NAME]; int off; } Label;
typedef struct { int code_off; char name[MAX_NAME]; } LabelPatch;

/* ===== Program Header 管理構造体 ===== */
/* ELF PT_LOAD のフラグ定数 */
#define PF_X 0x1
#define PF_W 0x2
#define PF_R 0x4

typedef struct {
    uint32_t p_type;    /* PT_LOAD=1 etc. */
    uint32_t p_flags;   /* PF_R|PF_X など */
    uint64_t p_offset;  /* ファイルオフセット */
    uint64_t p_vaddr;
    uint64_t p_paddr;
    uint64_t p_filesz;
    uint64_t p_memsz;
    uint64_t p_align;
} PhdrEntry;

typedef struct {
    PhdrEntry entries[MAX_PHDRS];
    int       count;
} PhdrList;

static void phdr_add(PhdrList *pl, uint32_t type, uint32_t flags,
                     uint64_t offset, uint64_t vaddr,
                     uint64_t filesz, uint64_t memsz, uint64_t align){
    if(pl->count >= MAX_PHDRS){ fprintf(stderr,"PhdrList overflow\n"); return; }
    PhdrEntry *e = &pl->entries[pl->count++];
    e->p_type   = type;
    e->p_flags  = flags;
    e->p_offset = offset;
    e->p_vaddr  = vaddr;
    e->p_paddr  = vaddr;
    e->p_filesz = filesz;
    e->p_memsz  = memsz;
    e->p_align  = align;
}

/* ===== CaiContext: 全グローバル状態を集約 ===== */
typedef struct {
    /* 命令列 */
    Instr    instrs[MAX_INSTRS];
    int      instr_count;

    /* 関数情報 */
    FuncInfo funcs[MAX_FUNCS];
    int      func_count;

    /* 仮想レジスタ（関数ごとにリセット） */
    VReg     vregs[MAX_REGS];
    int      vreg_count;
    int      stack_used;
    int      reg_used[NUM_ALLOC_REGS];
    char     eax_holds[MAX_NAME];

    /* コードバッファ */
    uint8_t  code[CODE_MAX];
    int      code_size;

    /* rodataバッファ */
    uint8_t  rodata[RODATA_MAX];
    int      rodata_size;

    /* シンボルテーブル */
    Sym      syms[MAX_FUNCS*2];
    int      sym_count;

    /* パッチ（関数間参照） */
    Patch    patches[MAX_PATCHES];
    int      patch_count;

    /* ラベル（関数内） */
    Label    labels[MAX_LABELS];
    int      label_count;
    LabelPatch lpatches[MAX_PATCHES];
    int      lpatch_count;
} CaiContext;

/* コンテキストをゼロ初期化して生成 */
static CaiContext *ctx_new(void){
    CaiContext *c = (CaiContext*)calloc(1, sizeof(CaiContext));
    if(!c){ perror("calloc CaiContext"); exit(1); }
    return c;
}

/* ===== コードバッファ操作（ctx経由） ===== */
static void emit1(CaiContext *c, uint8_t b){ c->code[c->code_size++]=b; }
static void emit2(CaiContext *c, uint8_t a, uint8_t b){ emit1(c,a); emit1(c,b); }
static void emit3(CaiContext *c, uint8_t a, uint8_t b, uint8_t v){ emit2(c,a,b); emit1(c,v); }
static void emit_i32(CaiContext *c, int32_t v){
    c->code[c->code_size++]=v&0xFF;
    c->code[c->code_size++]=(v>>8)&0xFF;
    c->code[c->code_size++]=(v>>16)&0xFF;
    c->code[c->code_size++]=(v>>24)&0xFF;
}
static void patch_i32(CaiContext *c, int off, int32_t v){
    c->code[off]=v&0xFF; c->code[off+1]=(v>>8)&0xFF;
    c->code[off+2]=(v>>16)&0xFF; c->code[off+3]=(v>>24)&0xFF;
}

/* ===== EAX追跡 ===== */
static void reset_eax(CaiContext *c){ c->eax_holds[0]='\0'; }
static int  eax_has(CaiContext *c, const char *n){ return n[0]&&!strcmp(c->eax_holds,n); }
static void set_eax(CaiContext *c, const char *n){ strncpy(c->eax_holds,n,MAX_NAME-1); }

/* ===== ユーティリティ ===== */
static void trim(char *s){
    char *p=s; while(*p==' '||*p=='\t') p++;
    memmove(s,p,strlen(p)+1);
    int len=strlen(s);
    while(len>0&&(s[len-1]==' '||s[len-1]=='\t'||s[len-1]=='\r'||s[len-1]=='\n'))
        s[--len]='\0';
}
static int is_imm(const char *s){
    if(!s||!*s) return 0;
    const char *p=s; if(*p=='-') p++;
    while(*p){ if(!isdigit(*p)) return 0; p++; }
    return 1;
}

/* ===== ModRM / REX / レジスタ定数 ===== */
static uint8_t modrm(int mod,int reg,int rm){ return (uint8_t)((mod<<6)|((reg&7)<<3)|(rm&7)); }
static uint8_t rex(int w,int r,int x,int b){ return (uint8_t)(0x40|(w?8:0)|(r?4:0)|(x?2:0)|(b?1:0)); }

#define EAX 0
#define ECX 1
#define EDX 2
#define EBX 3
#define RSP 4
#define RBP 5
#define ESI 6
#define EDI 7

static const int argregs64[]={7,6,2,1,8,9}; /* rdi,rsi,rdx,rcx,r8,r9 */
static int phys_to_reg(int pi){ return alloc_phys[pi]; }

/* ===== emit ヘルパー（ctx版） ===== */
static void emit_push(CaiContext *c, int r){
    if(r>=8){ emit1(c,0x41); emit1(c,(uint8_t)(0x50+(r-8))); }
    else emit1(c,(uint8_t)(0x50+r));
}
static void emit_pop(CaiContext *c, int r){
    if(r>=8){ emit1(c,0x41); emit1(c,(uint8_t)(0x58+(r-8))); }
    else emit1(c,(uint8_t)(0x58+r));
}
static void emit_mov_r32_imm(CaiContext *c, int r, int32_t imm){
    if(r>=8) emit1(c,rex(0,0,0,1));
    emit1(c,(uint8_t)(0xB8+(r&7))); emit_i32(c,imm);
}
static void emit_store_r32(CaiContext *c, int r, int off){
    if(r>=8) emit1(c,rex(0,1,0,0));
    emit1(c,0x89);
    if(off>=-128&&off<=127){ emit1(c,modrm(1,r&7,RBP)); emit1(c,(uint8_t)(int8_t)off); }
    else { emit1(c,modrm(2,r&7,RBP)); emit_i32(c,off); }
}
static void emit_load_r32(CaiContext *c, int r, int off){
    if(r>=8) emit1(c,rex(0,1,0,0));
    emit1(c,0x8B);
    if(off>=-128&&off<=127){ emit1(c,modrm(1,r&7,RBP)); emit1(c,(uint8_t)(int8_t)off); }
    else { emit1(c,modrm(2,r&7,RBP)); emit_i32(c,off); }
}
static void emit_mov_r32(CaiContext *c, int dst, int src){
    if(dst>=8||src>=8) emit1(c,rex(0,src>=8,0,dst>=8));
    emit1(c,0x89); emit1(c,modrm(3,src&7,dst&7));
}

/* ===== vreg操作 ===== */
static int find_vreg(CaiContext *c, const char *n){
    for(int i=0;i<c->vreg_count;i++) if(!strcmp(c->vregs[i].name,n)) return i;
    return -1;
}
static int alloc_slot(CaiContext *c, const char *n){
    int i=find_vreg(c,n); if(i>=0) return i;
    c->stack_used+=8;
    c->vregs[c->vreg_count].stack_off=-c->stack_used;
    c->vregs[c->vreg_count].phys_reg=-1;
    c->vregs[c->vreg_count].use_count=0;
    c->vregs[c->vreg_count].is_ptr=strstr(n,".ptr")!=NULL;
    strncpy(c->vregs[c->vreg_count].name,n,MAX_NAME-1);
    return c->vreg_count++;
}

static void store_eax_to(CaiContext *c, const char *dst){
    int i=find_vreg(c,dst); if(i<0) i=alloc_slot(c,dst);
    if(c->vregs[i].phys_reg>=0){
        int pr=phys_to_reg(c->vregs[i].phys_reg);
        if(pr!=EAX) emit_mov_r32(c,pr,EAX);
    } else {
        emit_store_r32(c,EAX,c->vregs[i].stack_off);
    }
    set_eax(c,dst);
}

/* 前方宣言 */
static void sym_ref(CaiContext *c, const char *name);
static void load_sym_addr_to_rax(CaiContext *c, const char *sym);

/* emit_rax_to_r64: mov r64_dst, rax
 * 89方向: REX.W + REX.B(if dst>=8) + 89 + ModRM(reg=rax=0, rm=dst)
 * rm=4(rsp/r12)のとき mod=3ならSIBバイト不要（mod=11時はSIBなし）
 */
static void emit_rax_to_r64(CaiContext *c, int pr){
    /* mov r64_dst, rax: 89 /r where reg=rax(0), rm=pr
     * mod=3(register): SIBバイト不要 */
    emit1(c,(uint8_t)(0x48|(pr>=8?1:0)));  /* REX.W + REX.B */
    emit1(c,0x89);
    emit1(c,modrm(3,EAX,pr&7));  /* reg=rax(0)→src, rm=pr→dst */
}

/* emit_rax_to_slot: raxの値をvregのスタックスロットまたは物理レジスタに64bitで保存 */
static void emit_rax_to_slot64(CaiContext *c, int vi){
    if(c->vregs[vi].phys_reg>=0){
        emit_rax_to_r64(c, phys_to_reg(c->vregs[vi].phys_reg));
    } else {
        int off=c->vregs[vi].stack_off;
        emit1(c,0x48); emit1(c,0x89);
        if(off>=-128&&off<=127){ emit1(c,modrm(1,EAX,RBP)); emit1(c,(uint8_t)(int8_t)off); }
        else { emit1(c,modrm(2,EAX,RBP)); emit_i32(c,off); }
    }
}

/* load_ptr_to_rax: ポインタ変数（64bit）をraxにロードする
 * 通常のload_to_eax（32bit）と異なり、64bitアドレスを保持する変数に使う。
 * $シンボル: lea rax,[rip+rel32]
 * %vreg(phys_reg): mov rax, r64（64bit MOV）
 * %vreg(stack):    mov rax, [rbp+off]（64bit MOV）
 * 即値:            mov rax, imm（movsx rax,imm32）
 */
static void load_ptr_to_rax(CaiContext *c, const char *val){
    if(val[0]=='$'){
        load_sym_addr_to_rax(c,val);
    } else if(val[0]=='%'){
        int i=find_vreg(c,val);
        if(i>=0){
            if(c->vregs[i].phys_reg>=0){
                int pr=phys_to_reg(c->vregs[i].phys_reg);
                /* mov rax, r64_src: REX.W+REX.B(if src>=8)+8B+ModRM(reg=rax=0,rm=src)
                 * mod=3時はSIBバイト不要。rm=4(r12)もmod=11ならSIBなし */
                emit1(c,(uint8_t)(0x48|(pr>=8?1:0)));  /* REX.W + REX.B */
                emit1(c,0x8B);
                emit1(c,modrm(3,EAX,pr&7));  /* reg=rax(dst), rm=pr(src) */
            } else {
                /* mov rax, [rbp+off] (64bit) */
                emit1(c,0x48); emit1(c,0x8B);
                int off=c->vregs[i].stack_off;
                if(off>=-128&&off<=127){ emit1(c,modrm(1,EAX,RBP)); emit1(c,(uint8_t)(int8_t)off); }
                else { emit1(c,modrm(2,EAX,RBP)); emit_i32(c,off); }
            }
        } else {
            emit1(c,0x31); emit1(c,0xC0); /* xor eax,eax */
        }
    } else if(is_imm(val)){
        int32_t v=atoi(val);
        emit_mov_r32_imm(c,EAX,v);
        emit3(c,0x48,0x63,0xC0); /* movsx rax,eax */
    } else {
        emit1(c,0x31); emit1(c,0xC0);
    }
}

/* $シンボルのアドレスをraxに64bitでロード（LEA相当）
 * PIE: シンボルVMAはwrite_exe内で確定するため、
 * ここでは mov rax, imm64 のimm64部分をパッチ登録する。
 * 実際には lea rax, [rip+rel32] を使う（PIE安全）。
 */
static void load_sym_addr_to_rax(CaiContext *c, const char *sym){
    const char *name = sym; if(name[0]=='$') name++;
    sym_ref(c, name);
    /* lea rax, [rip + rel32]
     * 48 8D 05 <rel32>
     * rel32 = sym_vma - (patch_vma + 4) → write_exe時に確定
     */
    emit1(c,0x48); emit1(c,0x8D); emit1(c,0x05);
    int po = c->code_size; emit_i32(c,0);
    c->patches[c->patch_count].code_off = po;
    strncpy(c->patches[c->patch_count].sym, name, MAX_NAME-1);
    c->patch_count++;
}

static void load_to_eax(CaiContext *c, const char *val){
    if(eax_has(c,val)) return;
    if(val[0]=='$'){
        /* dataラベル等のシンボルアドレスをraxにロード */
        load_sym_addr_to_rax(c, val);
        /* raxに64bitアドレスが入っている。
         * eax(32bit)ではなくraxを使うため、store_eax_toは使わず
         * 呼び出し側でraxをそのまま使う必要がある。
         * ここではeax_holdsにvalを記録してキャッシュ扱いにする。 */
    } else if(val[0]=='%'){
        int i=find_vreg(c,val);
        if(i>=0){
            if(c->vregs[i].phys_reg>=0){
                int pr=phys_to_reg(c->vregs[i].phys_reg);
                if(pr!=EAX) emit_mov_r32(c,EAX,pr);
            } else {
                emit_load_r32(c,EAX,c->vregs[i].stack_off);
            }
        } else {
            emit1(c,0x31); emit1(c,0xC0);
        }
    } else if(is_imm(val)){
        int32_t v=atoi(val);
        if(v==0){ emit1(c,0x31); emit1(c,0xC0); }
        else emit_mov_r32_imm(c,EAX,v);
    } else {
        emit1(c,0x31); emit1(c,0xC0);
    }
    set_eax(c,val);
}

/* ===== パーサー ===== */
static void parse_line(CaiContext *c, char *line){
    trim(line);
    if(line[0]=='#'||line[0]=='\0'){ c->instrs[c->instr_count++].kind=OP_COMMENT; return; }
    Instr *ins=&c->instrs[c->instr_count]; memset(ins,0,sizeof(Instr));
    char *tok=strtok(line," \t"); if(!tok) return;

    #define NEXT (tok=strtok(NULL," \t"))
    if(!strcmp(tok,"func")||!strcmp(tok,"export")){
        ins->is_export=!strcmp(tok,"export");
        if(ins->is_export) NEXT;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        ins->kind=OP_FUNC;
    } else if(!strcmp(tok,"endfunc")){ ins->kind=OP_ENDFUNC;
    } else if(!strcmp(tok,"alloc")){
        ins->kind=OP_ALLOC; NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) ins->argc=atoi(tok);
    } else if(!strcmp(tok,"store")){
        ins->kind=OP_STORE; NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"load")){
        ins->kind=OP_LOAD; NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"add")||!strcmp(tok,"sub")||!strcmp(tok,"mul")||!strcmp(tok,"div")){
        ins->kind=!strcmp(tok,"add")?OP_ADD:!strcmp(tok,"sub")?OP_SUB:!strcmp(tok,"mul")?OP_MUL:OP_DIV;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->b,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"clt")||!strcmp(tok,"cle")||!strcmp(tok,"ceq")||
               !strcmp(tok,"cne")||!strcmp(tok,"cgt")||!strcmp(tok,"cge")){
        ins->kind=!strcmp(tok,"clt")?OP_CLT:!strcmp(tok,"cle")?OP_CLE:
                  !strcmp(tok,"ceq")?OP_CEQ:!strcmp(tok,"cne")?OP_CNE:
                  !strcmp(tok,"cgt")?OP_CGT:OP_CGE;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->b,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"label")){
        ins->kind=OP_LABEL; NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"jmp")){
        ins->kind=OP_JMP; NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"jnz")){
        ins->kind=OP_JNZ;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->b,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"call")){
        ins->kind=OP_CALL;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
        while((NEXT)&&ins->argc<MAX_ARGS) strncpy(ins->args[ins->argc++],tok,MAX_NAME-1);
    } else if(!strcmp(tok,"ret")){
        NEXT; if(tok){ ins->kind=OP_RET; strncpy(ins->dst,tok,MAX_NAME-1); }
        else ins->kind=OP_RETV;
    } else if(!strcmp(tok,"extern")){
        ins->kind=OP_EXTERN; NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"data")){
        ins->kind=OP_DATA;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        char *rest=strtok(NULL,"");
        if(rest){
            trim(rest);
            if(rest[0]=='"'){
                rest++;
                int slen=0;
                while(*rest&&*rest!='"'&&slen<511){
                    if(*rest=='\\'&&*(rest+1)){
                        rest++;
                        switch(*rest){
                            case 'n': ins->str_val[slen++]='\n'; break;
                            case 't': ins->str_val[slen++]='\t'; break;
                            case '\\': ins->str_val[slen++]='\\'; break;
                            case '"': ins->str_val[slen++]='"'; break;
                            default: ins->str_val[slen++]='\\'; ins->str_val[slen++]=*rest; break;
                        }
                    } else {
                        ins->str_val[slen++]=*rest;
                    }
                    rest++;
                }
                ins->str_val[slen]='\0';
            }
        }
    } else if(!strcmp(tok,"syscall")){
        ins->kind=OP_SYSCALL;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
        ins->argc=0;
        while((NEXT)&&ins->argc<3) strncpy(ins->args[ins->argc++],tok,MAX_NAME-1);
    } else if(!strcmp(tok,"storep")){
        /* storep %ptr %val — [ptr]に64bit値をストア */
        ins->kind=OP_STOREP;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1); /* ptr */
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);   /* val */
    } else if(!strcmp(tok,"loadp2")){
        /* loadp2 %dst %ptr — [ptr]から64bitロード */
        ins->kind=OP_LOADP2;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"addp")){
        /* addp %dst %ptr %off — ポインタ(64bit)+オフセット(32bit) */
        ins->kind=OP_ADDP;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);   /* ptr */
        NEXT; if(tok) strncpy(ins->b,tok,MAX_NAME-1);   /* off */
    } else if(!strcmp(tok,"add64")||!strcmp(tok,"sub64")||
               !strcmp(tok,"mul64")||!strcmp(tok,"div64")){
        ins->kind=!strcmp(tok,"add64")?OP_ADD64:!strcmp(tok,"sub64")?OP_SUB64:
                  !strcmp(tok,"mul64")?OP_MUL64:OP_DIV64;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->b,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"clt64")||!strcmp(tok,"cle64")||!strcmp(tok,"ceq64")||
               !strcmp(tok,"cne64")||!strcmp(tok,"cgt64")||!strcmp(tok,"cge64")){
        ins->kind=!strcmp(tok,"clt64")?OP_CLT64:!strcmp(tok,"cle64")?OP_CLE64:
                  !strcmp(tok,"ceq64")?OP_CEQ64:!strcmp(tok,"cne64")?OP_CNE64:
                  !strcmp(tok,"cgt64")?OP_CGT64:OP_CGE64;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->b,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"mov64")){
        ins->kind=OP_MOV64;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"addf")||!strcmp(tok,"subf")||
               !strcmp(tok,"mulf")||!strcmp(tok,"divf")){
        ins->kind=!strcmp(tok,"addf")?OP_ADDF:!strcmp(tok,"subf")?OP_SUBF:
                  !strcmp(tok,"mulf")?OP_MULF:OP_DIVF;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->b,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"itof2")){
        ins->kind=OP_ITOF2;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"ftoi2")){
        ins->kind=OP_FTOI2;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"loadb")){
        ins->kind=OP_LOADB;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
    } else if(!strcmp(tok,"storeb")){
        ins->kind=OP_STOREB;
        NEXT; if(tok) strncpy(ins->dst,tok,MAX_NAME-1);
        NEXT; if(tok) strncpy(ins->a,tok,MAX_NAME-1);
    } else { ins->kind=OP_COMMENT; }
    #undef NEXT
    c->instr_count++;
}

static void parse_file(CaiContext *c, const char *path){
    FILE *f=fopen(path,"r"); if(!f){perror("fopen");exit(1);}
    char line[512];
    while(fgets(line,sizeof(line),f)) parse_line(c,line);
    fclose(f);
}

/* ===== 関数解析 ===== */
static int is_leaf(CaiContext *c, int s, int e){
    for(int i=s;i<e;i++)
        if(c->instrs[i].kind==OP_CALL||c->instrs[i].kind==OP_SYSCALL) return 0;
    return 1;
}
static int count_params(CaiContext *c, int s, int e){
    int mx=-1;
    for(int i=s;i<e;i++){
        Instr *ins=&c->instrs[i];
        char *ptrs[]={ins->dst,ins->a,ins->b};
        for(int j=0;j<3;j++) if(strncmp(ptrs[j],"%arg",4)==0){ int n=atoi(ptrs[j]+4); if(n>mx) mx=n; }
    }
    return mx+1;
}
static void count_uses(CaiContext *c, int s, int e){
    for(int i=s;i<e;i++){
        Instr *ins=&c->instrs[i];
        for(int j=0;j<c->vreg_count;j++){
            if(!strcmp(ins->dst,c->vregs[j].name)) c->vregs[j].use_count++;
            if(!strcmp(ins->a,c->vregs[j].name))   c->vregs[j].use_count++;
            if(!strcmp(ins->b,c->vregs[j].name))   c->vregs[j].use_count++;
            for(int k=0;k<ins->argc;k++) if(!strcmp(ins->args[k],c->vregs[j].name)) c->vregs[j].use_count++;
        }
    }
}

static void init_regalloc(CaiContext *c, FuncInfo *fn){
    c->vreg_count=0; c->stack_used=0;
    memset(c->reg_used,0,sizeof(c->reg_used));

    for(int i=fn->instr_start;i<fn->instr_end;i++){
        Instr *ins=&c->instrs[i];
        const char *ns[]={ins->dst,ins->a,ins->b};
        for(int j=0;j<3;j++){
            const char *n=ns[j]; if(n[0]!='%') continue;
            if(find_vreg(c,n)<0&&c->vreg_count<MAX_REGS){
                strncpy(c->vregs[c->vreg_count].name,n,MAX_NAME-1);
                c->vregs[c->vreg_count].phys_reg=-1;
                c->vregs[c->vreg_count].use_count=0;
                c->vregs[c->vreg_count].is_ptr=strstr(n,".ptr")!=NULL;
                c->vreg_count++;
            }
        }
        for(int j=0;j<ins->argc;j++){
            const char *n=ins->args[j]; if(n[0]!='%') continue;
            if(find_vreg(c,n)<0&&c->vreg_count<MAX_REGS){
                strncpy(c->vregs[c->vreg_count].name,n,MAX_NAME-1);
                c->vregs[c->vreg_count].phys_reg=-1;
                c->vregs[c->vreg_count].use_count=0;
                c->vregs[c->vreg_count].is_ptr=strstr(n,".ptr")!=NULL;
                c->vreg_count++;
            }
        }
    }
    count_uses(c,fn->instr_start,fn->instr_end);

    /* f32命令のdst/srcにis_float=1を設定（物理レジスタ割り当て禁止） */
    for(int i=fn->instr_start;i<fn->instr_end;i++){
        Instr *ins=&c->instrs[i];
        int is_f=(ins->kind==OP_ADDF||ins->kind==OP_SUBF||
                  ins->kind==OP_MULF||ins->kind==OP_DIVF||
                  ins->kind==OP_ITOF2||ins->kind==OP_FTOI2);
        if(!is_f) continue;
        /* dst と src(a,b) にis_float=1 */
        const char *fns[]={ins->dst,ins->a,ins->b};
        for(int j=0;j<3;j++){
            if(fns[j][0]!='%') continue;
            int vi=find_vreg(c,fns[j]);
            if(vi>=0) c->vregs[vi].is_float=1;
        }
    }

    if(fn->is_leaf){
        for(int i=0;i<c->vreg_count-1;i++)
            for(int j=i+1;j<c->vreg_count;j++)
                if(c->vregs[j].use_count>c->vregs[i].use_count){
                    VReg t=c->vregs[i]; c->vregs[i]=c->vregs[j]; c->vregs[j]=t;
                }
        int ri=0;
        for(int i=0;i<c->vreg_count&&ri<NUM_ALLOC_REGS;i++)
            if(c->vregs[i].use_count>=2&&!c->vregs[i].is_ptr&&!c->vregs[i].is_float&&strncmp(c->vregs[i].name,"%arg",4)){
                c->vregs[i].phys_reg=ri++; c->reg_used[c->vregs[i].phys_reg]=1;
            }
    }

    for(int i=0;i<c->vreg_count;i++){
        if(c->vregs[i].phys_reg>=0) continue;
        c->stack_used+=8; c->vregs[i].stack_off=-c->stack_used;
    }
    for(int i=0;i<6;i++){
        char an[MAX_NAME]; snprintf(an,MAX_NAME,"%%arg%d",i);
        if(find_vreg(c,an)<0&&c->vreg_count<MAX_REGS){
            strncpy(c->vregs[c->vreg_count].name,an,MAX_NAME-1);
            c->stack_used+=8;
            c->vregs[c->vreg_count].stack_off=-c->stack_used;
            c->vregs[c->vreg_count].phys_reg=-1;
            c->vreg_count++;
        }
    }
    c->stack_used=(c->stack_used+15)&~15;
    fn->stack_size=c->stack_used;
}

/* ===== ラベル操作 ===== */
static void label_def(CaiContext *c, const char *name){
    strncpy(c->labels[c->label_count].name,name,MAX_NAME-1);
    c->labels[c->label_count].off=c->code_size;
    c->label_count++;
}
static int label_find(CaiContext *c, const char *name){
    for(int i=0;i<c->label_count;i++) if(!strcmp(c->labels[i].name,name)) return c->labels[i].off;
    return -1;
}
static void lpatch_add(CaiContext *c, int coff, const char *name){
    strncpy(c->lpatches[c->lpatch_count].name,name,MAX_NAME-1);
    c->lpatches[c->lpatch_count].code_off=coff;
    c->lpatch_count++;
}
static void resolve_labels(CaiContext *c){
    for(int i=0;i<c->lpatch_count;i++){
        int off=label_find(c,c->lpatches[i].name);
        if(off<0){ fprintf(stderr,"未解決ラベル: %s\n",c->lpatches[i].name); continue; }
        int32_t rel=off-(c->lpatches[i].code_off+4);
        patch_i32(c,c->lpatches[i].code_off,rel);
    }
}

/* ===== シンボル操作 ===== */
static void sym_define(CaiContext *c, const char *name, int off, int global){
    int i; for(i=0;i<c->sym_count;i++) if(!strcmp(c->syms[i].name,name)) break;
    if(i==c->sym_count){ strncpy(c->syms[i].name,name,MAX_NAME-1); c->sym_count++; }
    c->syms[i].off=off; c->syms[i].defined=1; c->syms[i].global=global;
}
static int sym_find2(CaiContext *c, const char *name){
    for(int i=0;i<c->sym_count;i++) if(!strcmp(c->syms[i].name,name)) return i;
    return -1;
}
static void sym_ref(CaiContext *c, const char *name){
    if(sym_find2(c,name)<0){
        strncpy(c->syms[c->sym_count].name,name,MAX_NAME-1);
        c->syms[c->sym_count].defined=0;
        c->syms[c->sym_count].global=1;
        c->sym_count++;
    }
}

/* ===== 関数コード生成 ===== */
static void gen_func(CaiContext *c, FuncInfo *fn){
    fn->is_leaf=is_leaf(c,fn->instr_start,fn->instr_end);
    fn->param_count=count_params(c,fn->instr_start,fn->instr_end);
    init_regalloc(c,fn);

    char fname[MAX_NAME];
    const char *raw=fn->name; if(raw[0]=='$') raw++;
    strncpy(fname,!strcmp(raw,"main")?"sim_main":raw,MAX_NAME-1);
    if(!strcmp(raw,"main")) fn->is_export=1;

    sym_define(c,fname,c->code_size,fn->is_export);

    /* プロローグ */
    emit1(c,0x55);
    emit2(c,0x48,0x89); emit1(c,modrm(3,RSP,RBP));
    int ss=fn->stack_size+64;
    emit3(c,0x48,0x81,0xEC); emit_i32(c,ss);

    for(int i=0;i<NUM_ALLOC_REGS;i++) if(c->reg_used[i]) emit_push(c,phys_to_reg(i));

    for(int i=0;i<fn->param_count&&i<6;i++){
        char an[MAX_NAME]; snprintf(an,MAX_NAME,"%%arg%d",i);
        int idx=find_vreg(c,an);
        if(idx>=0){
            if(c->vregs[idx].phys_reg>=0)
                emit_mov_r32(c,phys_to_reg(c->vregs[idx].phys_reg),argregs64[i]);
            else {
                /* 引数レジスタ(64bit)をスタックに64bitで保存
                 * mov [rbp+off], r64: REX.W + REX.R(if src>=8) + 89 + ModRM */
                int ar=argregs64[i];
                int off=c->vregs[idx].stack_off;
                emit1(c,(uint8_t)(0x48|(ar>=8?4:0)));
                emit1(c,0x89);
                if(off>=-128&&off<=127){ emit1(c,modrm(1,ar&7,RBP)); emit1(c,(uint8_t)(int8_t)off); }
                else { emit1(c,modrm(2,ar&7,RBP)); emit_i32(c,off); }
            }
        }
    }

    reset_eax(c);
    c->label_count=0; c->lpatch_count=0;

    for(int i=fn->instr_start;i<fn->instr_end;i++){
        Instr *ins=&c->instrs[i];
        switch(ins->kind){
        case OP_COMMENT: break;
        case OP_ALLOC: { int idx=find_vreg(c,ins->dst); if(idx<0) alloc_slot(c,ins->dst); break; }

        case OP_STORE: {
            load_to_eax(c,ins->a);
            int idx=find_vreg(c,ins->dst); if(idx<0) idx=alloc_slot(c,ins->dst);
            if(c->vregs[idx].phys_reg>=0){
                int pr=phys_to_reg(c->vregs[idx].phys_reg);
                if(pr!=EAX) emit_mov_r32(c,pr,EAX);
            } else {
                emit_store_r32(c,EAX,c->vregs[idx].stack_off);
            }
            set_eax(c,ins->dst);
            break;
        }

        case OP_LOAD: {
            int si=find_vreg(c,ins->a);
            if(eax_has(c,ins->a)){
                store_eax_to(c,ins->dst);
            } else if(si>=0){
                if(c->vregs[si].phys_reg>=0){
                    int pr=phys_to_reg(c->vregs[si].phys_reg);
                    if(pr!=EAX) emit_mov_r32(c,EAX,pr);
                } else {
                    emit_load_r32(c,EAX,c->vregs[si].stack_off);
                }
                store_eax_to(c,ins->dst);
            } else {
                emit1(c,0x31); emit1(c,0xC0);
                store_eax_to(c,ins->dst);
            }
            break;
        }

        case OP_ADD: {
            load_to_eax(c,ins->a);
            if(is_imm(ins->b)){
                int v=atoi(ins->b);
                if(v==1){ emit1(c,0xFF); emit1(c,0xC0); }
                else if(v==-1){ emit1(c,0xFF); emit1(c,0xC8); }
                else if(v>=-128&&v<=127){ emit2(c,0x83,0xC0); emit1(c,(uint8_t)(int8_t)v); }
                else { emit1(c,0x05); emit_i32(c,v); }
            } else {
                emit_mov_r32(c,ECX,EAX);
                load_to_eax(c,ins->b);
                emit2(c,0x01,0xC8);
            }
            reset_eax(c); store_eax_to(c,ins->dst);
            break;
        }

        case OP_SUB: {
            if(is_imm(ins->a)&&atoi(ins->a)==0){
                load_to_eax(c,ins->b);
                emit2(c,0xF7,0xD8);
            } else {
                load_to_eax(c,ins->a);
                if(is_imm(ins->b)){
                    int v=atoi(ins->b);
                    if(v==1){ emit1(c,0xFF); emit1(c,0xC8); }
                    else if(v>=-128&&v<=127){ emit2(c,0x83,0xE8); emit1(c,(uint8_t)(int8_t)v); }
                    else { emit1(c,0x2D); emit_i32(c,v); }
                } else {
                    emit_mov_r32(c,ECX,EAX);
                    load_to_eax(c,ins->b);
                    emit2(c,0x29,0xC1);
                    emit2(c,0x89,0xC8);
                }
            }
            reset_eax(c); store_eax_to(c,ins->dst);
            break;
        }

        case OP_MUL: {
            load_to_eax(c,ins->a);
            emit_mov_r32(c,ECX,EAX);
            load_to_eax(c,ins->b);
            emit3(c,0x0F,0xAF,0xC1);
            reset_eax(c); store_eax_to(c,ins->dst);
            break;
        }

        case OP_DIV: {
            load_to_eax(c,ins->a);
            emit_mov_r32(c,ECX,EAX);
            load_to_eax(c,ins->b);
            emit2(c,0x87,0xC1);
            emit1(c,0x99);
            emit2(c,0xF7,0xF9);
            reset_eax(c); store_eax_to(c,ins->dst);
            break;
        }

        case OP_CLT: case OP_CLE: case OP_CEQ:
        case OP_CNE: case OP_CGT: case OP_CGE: {
            load_to_eax(c,ins->a);
            if(is_imm(ins->b)){
                int v=atoi(ins->b);
                if(v>=-128&&v<=127){ emit2(c,0x83,0xF8); emit1(c,(uint8_t)(int8_t)v); }
                else { emit1(c,0x3D); emit_i32(c,v); }
            } else {
                emit_mov_r32(c,ECX,EAX);
                load_to_eax(c,ins->b);
                emit2(c,0x39,0xC1);
            }
            uint8_t cc[]={0x9C,0x9E,0x94,0x95,0x9F,0x9D};
            int ci=ins->kind-OP_CLT;
            emit3(c,0x0F,cc[ci],0xC0);
            emit3(c,0x0F,0xB6,0xC0);
            reset_eax(c); store_eax_to(c,ins->dst);
            break;
        }

        case OP_LABEL:
            reset_eax(c);
            label_def(c,ins->dst);
            break;

        case OP_JMP:
            reset_eax(c);
            { emit1(c,0xE9); int po=c->code_size; emit_i32(c,0); lpatch_add(c,po,ins->dst); }
            break;

        case OP_JNZ: {
            load_to_eax(c,ins->dst);
            emit2(c,0x85,0xC0);
            emit2(c,0x0F,0x85); int pt=c->code_size; emit_i32(c,0); lpatch_add(c,pt,ins->a);
            emit1(c,0xE9);      int pf=c->code_size; emit_i32(c,0); lpatch_add(c,pf,ins->b);
            reset_eax(c);
            break;
        }

        case OP_CALL: {
            for(int j=0;j<NUM_ALLOC_REGS;j++) if(c->reg_used[j]) emit_push(c,phys_to_reg(j));
            for(int j=0;j<ins->argc&&j<6;j++){
                int ar=argregs64[j];
                if(ins->args[j][0]=='$'){
                    /* $シンボル: lea r64,[rip+rel32] で64bitアドレスを引数レジスタに直接設定 */
                    load_sym_addr_to_rax(c,ins->args[j]);
                    /* mov ar, rax (64bit) */
                    emit1(c,(uint8_t)(0x48|(ar>=8?1:0)));
                    emit1(c,0x89);
                    emit1(c,modrm(3,EAX,ar&7));
                } else {
                    /* %vreg または即値: load_ptr_to_raxで64bit対応ロード */
                    load_ptr_to_rax(c,ins->args[j]);
                    /* mov ar64, rax */
                    emit1(c,(uint8_t)(0x48|(ar>=8?1:0)));
                    emit1(c,0x89);
                    emit1(c,modrm(3,EAX,ar&7));
                }
            }
            reset_eax(c);
            const char *callee=ins->a; if(callee[0]=='$') callee++;
            char cn[MAX_NAME]; strncpy(cn,!strcmp(callee,"main")?"sim_main":callee,MAX_NAME-1);
            sym_ref(c,cn);
            emit1(c,0xE8); int po=c->code_size; emit_i32(c,0);
            c->patches[c->patch_count].code_off=po;
            strncpy(c->patches[c->patch_count].sym,cn,MAX_NAME-1);
            c->patch_count++;
            for(int j=NUM_ALLOC_REGS-1;j>=0;j--) if(c->reg_used[j]) emit_pop(c,phys_to_reg(j));
            if(ins->dst[0]&&ins->dst[0]!='_') store_eax_to(c,ins->dst);
            else reset_eax(c);
            break;
        }

        case OP_RET: {
            load_to_eax(c,ins->dst);
            emit3(c,0x48,0x63,0xC0);
            for(int j=NUM_ALLOC_REGS-1;j>=0;j--) if(c->reg_used[j]) emit_pop(c,phys_to_reg(j));
            emit1(c,0xC9); emit1(c,0xC3);
            reset_eax(c);
            break;
        }

        case OP_RETV:
            emit2(c,0x31,0xC0);
            for(int j=NUM_ALLOC_REGS-1;j>=0;j--) if(c->reg_used[j]) emit_pop(c,phys_to_reg(j));
            emit1(c,0xC9); emit1(c,0xC3);
            reset_eax(c);
            break;

        case OP_SYSCALL: {
            /* syscall %dst <nr> <arg0> <arg1> <arg2>
             *
             * x86_64 syscall ABI:
             *   rax = syscall番号
             *   rdi = arg0
             *   rsi = arg1
             *   rdx = arg2
             *   戻り値 = rax
             *
             * callee-savedを退避してからsyscallを発行する。
             * syscallはrax/rcx/r11を破壊するが、
             * callee-savedレジスタ（rbx/r12-r15）は保持される。
             */
            /* callee-saved退避 */
            for(int j=0;j<NUM_ALLOC_REGS;j++) if(c->reg_used[j]) emit_push(c,phys_to_reg(j));

            /* 引数をrdi/rsi/rdxに設定するヘルパー
             * $シンボルの場合はload_sym_addr_to_raxでraxに64bitアドレスが入る。
             * %vreg・即値の場合はeaxに32bit値が入り、movsxで64bitに拡張する。
             */
            /* mov r64_dst, rax: REX.W+REX.B(if dst>=8)+89+ModRM(reg=rax=0,rm=dst)
             * mod=3時はSIBバイト不要 */
            #define MOV_RAX_TO_R64(dreg) do { \
                emit1(c,(uint8_t)(0x48|((dreg)>=8?1:0))); \
                emit1(c,0x89); \
                emit1(c,modrm(3,EAX,(dreg)&7)); \
            } while(0)
            #define LOAD_ARG_TO_REG(argval, dreg) do { \
                load_ptr_to_rax(c,(argval)); \
                if((dreg)!=EAX) MOV_RAX_TO_R64(dreg); \
            } while(0)

            /* arg2 → rdx */
            if(ins->argc>=3){
                LOAD_ARG_TO_REG(ins->args[2], EDX);
            } else {
                emit1(c,0x48); emit1(c,0x31); emit1(c,modrm(3,EDX,EDX));
            }

            /* arg1 → rsi */
            if(ins->argc>=2){
                LOAD_ARG_TO_REG(ins->args[1], ESI);
            } else {
                emit1(c,0x48); emit1(c,0x31); emit1(c,modrm(3,ESI,ESI));
            }

            /* arg0 → rdi */
            if(ins->argc>=1){
                LOAD_ARG_TO_REG(ins->args[0], EDI);
            } else {
                emit1(c,0x48); emit1(c,0x31); emit1(c,modrm(3,EDI,EDI));
            }
            #undef LOAD_ARG_TO_REG

            /* syscall番号 → rax */
            if(is_imm(ins->a)){
                int32_t nr=atoi(ins->a);
                /* mov eax, imm32 → xor+mov */
                if(nr==0){ emit1(c,0x31); emit1(c,0xC0); }
                else { emit_mov_r32_imm(c,EAX,nr); }
                /* movsx rax, eax */
                emit3(c,0x48,0x63,0xC0);
            } else {
                load_to_eax(c,ins->a);
                emit3(c,0x48,0x63,0xC0);
            }

            /* syscall命令 */
            emit2(c,0x0F,0x05);

            /* callee-saved復元 */
            for(int j=NUM_ALLOC_REGS-1;j>=0;j--) if(c->reg_used[j]) emit_pop(c,phys_to_reg(j));

            /* 戻り値（rax）をdstに保存
             * eaxに結果が入っている（raxの下位32bit） */
            reset_eax(c);
            if(ins->dst[0]&&ins->dst[0]!='_') store_eax_to(c,ins->dst);
            break;
        }

        case OP_STOREP: {
            /* storep %ptr %val
             * val(64bit)を[ptr]（スタック上の変数スロット）に書く
             * ptrはスタックオフセットとして扱う（アドレスではなく変数名）
             * 用途: 64bitポインタ値をスタックに保存
             */
            /* val → rax (64bit) */
            load_ptr_to_rax(c,ins->a);
            { int di2=find_vreg(c,ins->dst);
              if(di2<0) di2=alloc_slot(c,ins->dst);
              emit_rax_to_slot64(c,di2); }
            reset_eax(c);
            break;
        }

        case OP_LOADP2: {
            /* loadp2 %dst %ptr
             * スタック上の変数スロットから64bit値をraxにロード
             * 用途: 64bitポインタ値の読み出し
             */
            load_ptr_to_rax(c,ins->a);
            /* dstに64bitで保存 */
            reset_eax(c);
            { int di=find_vreg(c,ins->dst); if(di<0) di=alloc_slot(c,ins->dst);
              emit_rax_to_slot64(c,di); }
            break;
        }

        case OP_ADDP: {
            /* addp %dst %ptr %off
             * 64bitポインタ + 32bitオフセット → 64bitポインタ
             * 手順:
             *   1. ptrを64bitでraxにロード
             *   2. offを32bitでrcxにロード → movsx rcx,ecx で64bit化
             *   3. add rax, rcx
             *   4. dstに保存（64bitアドレスとして）
             */
            /* ptr → rax (64bit) */
            load_ptr_to_rax(c,ins->a);
            /* off → rcx (32bit → 64bit符号拡張) */
            reset_eax(c);
            if(is_imm(ins->b)){
                int32_t v=atoi(ins->b);
                if(v==0){
                    /* add rax,0 は不要 */
                } else {
                    /* add rax, imm32 (64bit): 48 05 <imm32> または 48 83 C0 <imm8> */
                    if(v>=-128&&v<=127){ emit3(c,0x48,0x83,0xC0); emit1(c,(uint8_t)(int8_t)v); }
                    else { emit1(c,0x48); emit1(c,0x05); emit_i32(c,v); }
                }
            } else {
                load_to_eax(c,ins->b);
                /* movsx rcx, eax */
                emit1(c,0x48); emit1(c,0x63); emit1(c,modrm(3,ECX,EAX));
                /* add rax, rcx (64bit): 48 01 C8 */
                emit3(c,0x48,0x01,0xC8);
            }
            reset_eax(c);
            { int di=find_vreg(c,ins->dst); if(di<0) di=alloc_slot(c,ins->dst);
              emit_rax_to_slot64(c,di); }
            break;
        }

        case OP_LOADB: {
            /* loadb %dst %ptr
             * ptrに入っている64bitアドレスの指す先から1バイトをゼロ拡張してロード
             * movzx eax, byte ptr [rax]
             */
            reset_eax(c);
            load_ptr_to_rax(c,ins->a);
            /* movzx eax, byte ptr [rax]: 0F B6 00 */
            emit3(c,0x0F,0xB6,0x00);
            reset_eax(c);
            store_eax_to(c,ins->dst);
            break;
        }

        case OP_STOREB: {
            /* storeb %ptr %val
             * valの下位8bit（al）を[ptr]に書き込む
             * 1. ptrのアドレス(64bit)をraxに取得 → rcxに移動
             * 2. valをeaxにロードしalを使う
             * 3. mov byte ptr [rcx], al
             */
            /* ptr(64bit address) → rcx */
            load_ptr_to_rax(c,ins->dst);
            emit1(c,0x48); emit1(c,0x89); emit1(c,modrm(3,EAX,ECX));
            /* val → eax (下位8bit=al を使う) */
            reset_eax(c);
            load_to_eax(c,ins->a);
            /* mov byte ptr [rcx], al: 88 01 */
            emit2(c,0x88,0x01);
            reset_eax(c);
            break;
        }

        /* ===== i64演算 ===== */
        case OP_ADD64: case OP_SUB64: case OP_MUL64: case OP_DIV64: {
            /* i64演算: REX.W=1 で64bit演算を行う
             * a,bをrax/rcxに64bitでロードして演算し、dstに64bitで保存 */
            load_ptr_to_rax(c,ins->a);
            /* mov rcx, rax */
            emit1(c,0x48); emit1(c,0x89); emit1(c,modrm(3,EAX,ECX));
            load_ptr_to_rax(c,ins->b);
            /* 演算: rax op rcx → rax */
            if(ins->kind==OP_ADD64){
                emit3(c,0x48,0x01,0xC8); /* add rax, rcx */
            } else if(ins->kind==OP_SUB64){
                /* sub rcx, rax; mov rax, rcx */
                emit3(c,0x48,0x29,0xC1); /* sub rcx, rax */
                emit3(c,0x48,0x89,0xC8); /* mov rax, rcx */
            } else if(ins->kind==OP_MUL64){
                emit1(c,0x48); emit3(c,0x0F,0xAF,0xC1); /* imul rax, rcx */
            } else { /* DIV64 */
                /* xchg rax,rcx; cqo; idiv rcx */
                emit3(c,0x48,0x87,0xC1); /* xchg rax, rcx */
                emit2(c,0x48,0x99);      /* cqo */
                emit3(c,0x48,0xF7,0xF9); /* idiv rcx */
            }
            reset_eax(c);
            { int di=find_vreg(c,ins->dst); if(di<0) di=alloc_slot(c,ins->dst);
              emit_rax_to_slot64(c,di); }
            break;
        }

        case OP_CLT64: case OP_CLE64: case OP_CEQ64:
        case OP_CNE64: case OP_CGT64: case OP_CGE64: {
            /* 64bit比較: REX.W=1 cmp + setcc */
            load_ptr_to_rax(c,ins->a);
            emit3(c,0x48,0x89,0xC1); /* mov rcx, rax */
            load_ptr_to_rax(c,ins->b);
            /* cmp rcx, rax (64bit) */
            emit3(c,0x48,0x39,0xC1);
            uint8_t cc[]={0x9C,0x9E,0x94,0x95,0x9F,0x9D};
            int ci=(int)(ins->kind-OP_CLT64);
            emit3(c,0x0F,cc[ci],0xC0); /* setcc al */
            emit3(c,0x0F,0xB6,0xC0);   /* movzx eax, al */
            reset_eax(c); store_eax_to(c,ins->dst);
            break;
        }

        case OP_MOV64: {
            /* 64bitコピー */
            load_ptr_to_rax(c,ins->a);
            reset_eax(c);
            { int di=find_vreg(c,ins->dst); if(di<0) di=alloc_slot(c,ins->dst);
              emit_rax_to_slot64(c,di); }
            break;
        }

        /* ===== f32演算（SSE2） ===== */
        /* f32はXMM0..XMM7レジスタを使う。
         * 現状は全てメモリ経由（スタック上のfloatスロット）で行う。
         * a→xmm0, b→xmm1, 演算, 結果→xmm0→dst
         *
         * float値はスタック上に4バイトとして格納する。
         * movss xmm0, [rbp+off]: F3 0F 10 45/85 <off>
         * movss [rbp+off], xmm0: F3 0F 11 45/85 <off>
         */
        case OP_ADDF: case OP_SUBF: case OP_MULF: case OP_DIVF: {
            /* a → xmm0 */
            { int si=find_vreg(c,ins->a);
              int off=(si>=0)?c->vregs[si].stack_off:0;
              /* movss xmm0, [rbp+off]: F3 0F 10 /r */
              emit1(c,0xF3); emit1(c,0x0F); emit1(c,0x10);
              if(off>=-128&&off<=127){ emit1(c,modrm(1,0,RBP)); emit1(c,(uint8_t)(int8_t)off); }
              else { emit1(c,modrm(2,0,RBP)); emit_i32(c,off); }
            }
            /* b → xmm1 */
            { int si=find_vreg(c,ins->b);
              int off=(si>=0)?c->vregs[si].stack_off:0;
              emit1(c,0xF3); emit1(c,0x0F); emit1(c,0x10);
              /* xmm1: ModRM reg=1 */
              if(off>=-128&&off<=127){ emit1(c,modrm(1,1,RBP)); emit1(c,(uint8_t)(int8_t)off); }
              else { emit1(c,modrm(2,1,RBP)); emit_i32(c,off); }
            }
            /* 演算: xmm0 op xmm1 → xmm0 */
            uint8_t fop=ins->kind==OP_ADDF?0x58:ins->kind==OP_SUBF?0x5C:
                        ins->kind==OP_MULF?0x59:0x5E;
            /* F3 0F <op> xmm0, xmm1: F3 0F <op> C1 */
            emit1(c,0xF3); emit1(c,0x0F); emit1(c,fop);
            emit1(c,modrm(3,0,1)); /* xmm0, xmm1 */
            /* xmm0 → dst（スタック） */
            { int di=find_vreg(c,ins->dst); if(di<0) di=alloc_slot(c,ins->dst);
              int off=c->vregs[di].stack_off;
              /* movss [rbp+off], xmm0: F3 0F 11 45/85 <off> */
              emit1(c,0xF3); emit1(c,0x0F); emit1(c,0x11);
              if(off>=-128&&off<=127){ emit1(c,modrm(1,0,RBP)); emit1(c,(uint8_t)(int8_t)off); }
              else { emit1(c,modrm(2,0,RBP)); emit_i32(c,off); }
            }
            reset_eax(c);
            break;
        }

        case OP_ITOF2: {
            /* i32 → f32: cvtsi2ss xmm0, eax
             * F3 0F 2A /r */
            load_to_eax(c,ins->a);
            /* cvtsi2ss xmm0, eax: F3 0F 2A C0 */
            emit1(c,0xF3); emit1(c,0x0F); emit1(c,0x2A);
            emit1(c,modrm(3,0,EAX));
            /* xmm0 → dst */
            { int di=find_vreg(c,ins->dst); if(di<0) di=alloc_slot(c,ins->dst);
              int off=c->vregs[di].stack_off;
              emit1(c,0xF3); emit1(c,0x0F); emit1(c,0x11);
              if(off>=-128&&off<=127){ emit1(c,modrm(1,0,RBP)); emit1(c,(uint8_t)(int8_t)off); }
              else { emit1(c,modrm(2,0,RBP)); emit_i32(c,off); }
            }
            reset_eax(c);
            break;
        }

        case OP_FTOI2: {
            /* f32 → i32: cvttss2si eax, xmm0 (切り捨て)
             * F3 0F 2C /r */
            { int si=find_vreg(c,ins->a);
              int off=(si>=0)?c->vregs[si].stack_off:0;
              /* movss xmm0, [rbp+off] */
              emit1(c,0xF3); emit1(c,0x0F); emit1(c,0x10);
              if(off>=-128&&off<=127){ emit1(c,modrm(1,0,RBP)); emit1(c,(uint8_t)(int8_t)off); }
              else { emit1(c,modrm(2,0,RBP)); emit_i32(c,off); }
            }
            /* cvttss2si eax, xmm0: F3 0F 2C C0 */
            emit1(c,0xF3); emit1(c,0x0F); emit1(c,0x2C);
            emit1(c,modrm(3,EAX,0));
            reset_eax(c); store_eax_to(c,ins->dst);
            break;
        }

        default: break;
        }
    }

    resolve_labels(c);
}

/* ===== ELFオブジェクトファイル(.o)生成 ===== */
typedef struct {
    uint8_t  e_ident[16]; uint16_t e_type,e_machine; uint32_t e_version;
    uint64_t e_entry,e_phoff,e_shoff; uint32_t e_flags;
    uint16_t e_ehsize,e_phentsize,e_phnum,e_shentsize,e_shnum,e_shstrndx;
} Elf64Ehdr;
typedef struct {
    uint32_t sh_name,sh_type,sh_flags; uint64_t sh_addr,sh_off;
    uint64_t sh_size; uint32_t sh_link,sh_info; uint64_t sh_align,sh_entsize;
} Elf64Shdr;
typedef struct {
    uint32_t st_name; uint8_t st_info,st_other; uint16_t st_shndx;
    uint64_t st_value,st_size;
} Elf64Sym;

#define R_X86_64_PLT32 4

static void write_obj(CaiContext *c, const char *path){
    uint8_t strtab[65536]; int strtab_size=1; strtab[0]=0;
    int sym_stridx[MAX_FUNCS*2];
    for(int i=0;i<c->sym_count;i++){
        sym_stridx[i]=strtab_size;
        int len=strlen(c->syms[i].name);
        memcpy(strtab+strtab_size,c->syms[i].name,len+1);
        strtab_size+=len+1;
    }

    Elf64Sym elf_syms[MAX_FUNCS*2+1];
    int elf_sym_count=0;
    memset(&elf_syms[0],0,sizeof(Elf64Sym)); elf_sym_count=1;
    int first_global=1;
    for(int i=0;i<c->sym_count;i++){
        if(c->syms[i].global) continue;
        Elf64Sym *s=&elf_syms[elf_sym_count++];
        s->st_name=sym_stridx[i];
        s->st_info=(0<<4)|2;
        s->st_shndx=1;
        s->st_value=c->syms[i].defined?(uint64_t)c->syms[i].off:0;
    }
    first_global=elf_sym_count;
    for(int i=0;i<c->sym_count;i++){
        if(!c->syms[i].global) continue;
        Elf64Sym *s=&elf_syms[elf_sym_count++];
        s->st_name=sym_stridx[i];
        s->st_info=(1<<4)|2;
        s->st_shndx=c->syms[i].defined?1:0;
        s->st_value=c->syms[i].defined?(uint64_t)c->syms[i].off:0;
    }

    typedef struct { uint64_t r_offset; uint64_t r_info; int64_t r_addend; } Rela64;
    Rela64 relas[MAX_PATCHES]; int rela_count=0;
    for(int i=0;i<c->patch_count;i++){
        int esi=-1;
        for(int j=1;j<elf_sym_count;j++)
            if(!strcmp((char*)strtab+elf_syms[j].st_name,c->patches[i].sym)){ esi=j; break; }
        if(esi<0){
            Elf64Sym *s=&elf_syms[elf_sym_count];
            int nl=strlen(c->patches[i].sym);
            int nsi=strtab_size;
            memcpy(strtab+strtab_size,c->patches[i].sym,nl+1); strtab_size+=nl+1;
            s->st_name=(uint32_t)nsi;
            s->st_info=(1<<4)|2; s->st_shndx=0; s->st_value=0;
            esi=elf_sym_count++;
        }
        relas[rela_count].r_offset=(uint64_t)c->patches[i].code_off;
        relas[rela_count].r_info=((uint64_t)esi<<32)|R_X86_64_PLT32;
        relas[rela_count].r_addend=-4;
        rela_count++;
    }

    const char shstrtab[]="\0.text\0.rela.text\0.symtab\0.strtab\0.shstrtab\0";
    int sh_text=1, sh_rela=7, sh_sym=18, sh_str=26, sh_shstr=34;

    uint64_t off=sizeof(Elf64Ehdr);
    uint64_t text_off=off; uint64_t text_sz=(uint64_t)c->code_size; off+=text_sz;
    off=align_up(off,8);
    uint64_t rela_off=off; uint64_t rela_sz=(uint64_t)(rela_count*sizeof(Rela64)); off+=rela_sz;
    off=align_up(off,8);
    uint64_t sym_off=off;  uint64_t sym_sz=(uint64_t)(elf_sym_count*sizeof(Elf64Sym)); off+=sym_sz;
    off=align_up(off,8);
    uint64_t str_off=off;  uint64_t str_sz=(uint64_t)strtab_size; off+=str_sz;
    off=align_up(off,8);
    uint64_t shstr_off=off; uint64_t shstr_sz=sizeof(shstrtab); off+=shstr_sz;
    off=align_up(off,8);
    uint64_t shoff=off;

    FILE *f=fopen(path,"wb"); if(!f){perror("fopen obj");return;}
    Elf64Ehdr eh; memset(&eh,0,sizeof(eh));
    memcpy(eh.e_ident,"\x7f" "ELF",4);
    eh.e_ident[4]=2; eh.e_ident[5]=1; eh.e_ident[6]=1;
    eh.e_type=1; eh.e_machine=62; eh.e_version=1;
    eh.e_ehsize=sizeof(Elf64Ehdr);
    eh.e_shentsize=sizeof(Elf64Shdr);
    eh.e_shnum=6; eh.e_shstrndx=5; eh.e_shoff=shoff;
    fwrite(&eh,sizeof(eh),1,f);

    fwrite(c->code,1,c->code_size,f);
    uint8_t zeros[16]={0};
    int pad=(int)(rela_off-(text_off+text_sz)); if(pad>0) fwrite(zeros,1,(size_t)pad,f);
    fwrite(relas,sizeof(Rela64),(size_t)rela_count,f);
    pad=(int)(sym_off-(rela_off+rela_sz)); if(pad>0) fwrite(zeros,1,(size_t)pad,f);
    fwrite(elf_syms,sizeof(Elf64Sym),(size_t)elf_sym_count,f);
    pad=(int)(str_off-(sym_off+sym_sz)); if(pad>0) fwrite(zeros,1,(size_t)pad,f);
    fwrite(strtab,1,(size_t)strtab_size,f);
    pad=(int)(shstr_off-(str_off+str_sz)); if(pad>0) fwrite(zeros,1,(size_t)pad,f);
    fwrite(shstrtab,1,shstr_sz,f);
    pad=(int)(shoff-(shstr_off+shstr_sz)); if(pad>0) fwrite(zeros,1,(size_t)pad,f);

    Elf64Shdr shdrs[6]; memset(shdrs,0,sizeof(shdrs));
    shdrs[1].sh_name=(uint32_t)sh_text; shdrs[1].sh_type=1; shdrs[1].sh_flags=6;
    shdrs[1].sh_off=text_off; shdrs[1].sh_size=text_sz; shdrs[1].sh_align=16;
    shdrs[2].sh_name=(uint32_t)sh_rela; shdrs[2].sh_type=4; shdrs[2].sh_flags=0x40;
    shdrs[2].sh_off=rela_off; shdrs[2].sh_size=rela_sz;
    shdrs[2].sh_link=3; shdrs[2].sh_info=1;
    shdrs[2].sh_align=8; shdrs[2].sh_entsize=sizeof(Rela64);
    shdrs[3].sh_name=(uint32_t)sh_sym; shdrs[3].sh_type=2;
    shdrs[3].sh_off=sym_off; shdrs[3].sh_size=sym_sz;
    shdrs[3].sh_link=4; shdrs[3].sh_info=(uint32_t)first_global;
    shdrs[3].sh_align=8; shdrs[3].sh_entsize=sizeof(Elf64Sym);
    shdrs[4].sh_name=(uint32_t)sh_str; shdrs[4].sh_type=3;
    shdrs[4].sh_off=str_off; shdrs[4].sh_size=str_sz; shdrs[4].sh_align=1;
    shdrs[5].sh_name=(uint32_t)sh_shstr; shdrs[5].sh_type=3;
    shdrs[5].sh_off=shstr_off; shdrs[5].sh_size=shstr_sz; shdrs[5].sh_align=1;
    fwrite(shdrs,sizeof(Elf64Shdr),6,f);
    fclose(f);
}

/* ===== ランタイムバイト列 ===== */
static const uint8_t runtime_bytes[] = {
    0x55,0x48,0x89,0xe5,0x48,0x83,0xec,0x40,0x48,0x89,0x7d,0xf8,0x48,0x89,0x75,0xf0,
    0x48,0x85,0xff,0x79,0x0d,0xc6,0x06,0x2d,0x48,0xff,0xc6,0x48,0x89,0x75,0xf0,0x48,
    0xf7,0xdf,0x48,0x89,0xf8,0x31,0xc9,0x31,0xd2,0x41,0xb8,0x0a,0x00,0x00,0x00,0x49,
    0xf7,0xf0,0x80,0xc2,0x30,0x88,0x54,0x0d,0xe0,0xff,0xc1,0x48,0x85,0xc0,0x75,0xe7,
    0x48,0x89,0x4d,0xd8,0x48,0x8b,0x75,0xf0,0x45,0x31,0xc9,0x49,0x39,0xc9,0x7d,0x18,
    0x49,0x89,0xca,0x4d,0x29,0xca,0x49,0xff,0xca,0x42,0x0f,0xb6,0x44,0x15,0xe0,0x42,
    0x88,0x04,0x0e,0x49,0xff,0xc1,0xeb,0xe3,0x48,0x8b,0x45,0xd8,0xc9,0xc3,
    /* _start at 0x6e */
    0x55,0x48,0x89,0xe5,0x48,0x81,0xec,0x00,0x01,0x00,0x00,0x53,0x41,0x54,0x41,0x55,
    0xb8,0xe4,0x00,0x00,0x00,0xbf,0x01,0x00,0x00,0x00,0x48,0x8d,0x75,0xd0,0x0f,0x05,
    0xe8,0x00,0x00,0x00,0x00,
    0x48,0x63,0xc0,0x49,0x89,0xc4,0xb8,0xe4,0x00,0x00,0x00,0xbf,0x01,0x00,0x00,0x00,
    0x48,0x8d,0x75,0xe0,0x0f,0x05,0x48,0x8b,0x45,0xe0,0x48,0x2b,0x45,0xd0,0x48,0x69,
    0xc0,0xe8,0x03,0x00,0x00,0x48,0x89,0xc3,0x48,0x8b,0x45,0xe8,0x48,0x2b,0x45,0xd8,
    0x48,0x99,0xb9,0x40,0x42,0x0f,0x00,0x48,0xf7,0xf9,0x48,0x01,0xd8,0x49,0x89,0xc5,
    0x48,0x8d,0xb5,0x38,0xff,0xff,0xff,0x31,0xc9,0xb0,0x53,0x88,0x04,0x0e,0xff,0xc1,
    0xb0,0x69,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x6d,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x69,
    0x88,0x04,0x0e,0xff,0xc1,0xb0,0x6c,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x61,0x88,0x04,
    0x0e,0xff,0xc1,0xb0,0x72,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x69,0x88,0x04,0x0e,0xff,
    0xc1,0xb0,0x74,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x79,0x88,0x04,0x0e,0xff,0xc1,0xb0,
    0x20,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x72,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x65,0x88,
    0x04,0x0e,0xff,0xc1,0xb0,0x73,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x75,0x88,0x04,0x0e,
    0xff,0xc1,0xb0,0x6c,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x74,0x88,0x04,0x0e,0xff,0xc1,
    0xb0,0x3a,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x20,0x88,0x04,0x0e,0xff,0xc1,0x4c,0x89,
    0xe7,0x48,0x8d,0x04,0x0e,0x51,0x56,0x48,0x89,0xc6,0xe8,0x8e,0xfe,0xff,0xff,0x5e,
    0x59,0x48,0x01,0xc1,0xb0,0x20,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x20,0x88,0x04,0x0e,
    0xff,0xc1,0xb0,0x74,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x69,0x88,0x04,0x0e,0xff,0xc1,
    0xb0,0x6d,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x65,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x3a,
    0x88,0x04,0x0e,0xff,0xc1,0xb0,0x20,0x88,0x04,0x0e,0xff,0xc1,0x4c,0x89,0xef,0x48,
    0x8d,0x04,0x0e,0x51,0x56,0x48,0x89,0xc6,0xe8,0x40,0xfe,0xff,0xff,0x5e,0x59,0x48,
    0x01,0xc1,0xb0,0x6d,0x88,0x04,0x0e,0xff,0xc1,0xb0,0x73,0x88,0x04,0x0e,0xff,0xc1,
    0xb0,0x0a,0x88,0x04,0x0e,0xff,0xc1,0xb8,0x01,0x00,0x00,0x00,0xbf,0x01,0x00,0x00,
    0x00,0x48,0x89,0xca,0x0f,0x05,0xb8,0x3c,0x00,0x00,0x00,0x48,0x31,0xff,0x0f,0x05
};
#define RUNTIME_ITOA64_OFF      0x00
#define RUNTIME_START_OFF       0x6e
#define RUNTIME_SIMMAIN_CALL_OFF 0x8f

static void emit_itoa64(CaiContext *c){
    sym_define(c,"__itoa64",c->code_size,0);
    for(int i=0;i<RUNTIME_START_OFF;i++) emit1(c,runtime_bytes[i]);
}
static void emit_runtime(CaiContext *c){
    sym_define(c,"_start",c->code_size,1);
    int start_code_off=c->code_size;
    int runtime_size=(int)sizeof(runtime_bytes);
    for(int i=RUNTIME_START_OFF;i<runtime_size;i++) emit1(c,runtime_bytes[i]);
    int call_off=start_code_off+(RUNTIME_SIMMAIN_CALL_OFF-RUNTIME_START_OFF);
    c->patches[c->patch_count].code_off=call_off;
    strncpy(c->patches[c->patch_count].sym,"sim_main",MAX_NAME-1);
    c->patch_count++;
}

static int rodata_add_str(CaiContext *c, const char *s){
    int off=c->rodata_size;
    while(*s) c->rodata[c->rodata_size++]=(uint8_t)*s++;
    c->rodata[c->rodata_size++]=0;
    return off;
}

/* ===== ELF構造体定義 ===== */
#define PAGE      0x1000ULL

/* PT_* 定数 */
#define PT_NULL     0
#define PT_LOAD     1
#define PT_DYNAMIC  2
#define PT_PHDR     6
#define PT_GNU_STACK 0x6474e551

/* DT_* 定数（.dynamic セクション用） */
#define DT_NULL  0
#define DT_DEBUG 21  /* デバッガ用（静的PIEの最小.dynamic） */

typedef struct {
    uint8_t  e_ident[16];
    uint16_t e_type, e_machine;
    uint32_t e_version;
    uint64_t e_entry, e_phoff, e_shoff;
    uint32_t e_flags;
    uint16_t e_ehsize, e_phentsize, e_phnum;
    uint16_t e_shentsize, e_shnum, e_shstrndx;
} ExeEhdr;

typedef struct {
    uint32_t p_type, p_flags;
    uint64_t p_offset, p_vaddr, p_paddr, p_filesz, p_memsz, p_align;
} ExePhdr;

typedef struct {
    uint32_t sh_name, sh_type, sh_flags;
    uint64_t sh_addr, sh_off, sh_size;
    uint32_t sh_link, sh_info;
    uint64_t sh_align, sh_entsize;
} ExeShdr;

typedef struct {
    int64_t d_tag;
    uint64_t d_val;
} Elf64Dyn;

/*
 * write_exe: 完全な静的PIE ELFバイナリを生成する
 *
 * ファイルレイアウト:
 *   0x0000 : ELFヘッダ (64B)
 *   0x0040 : Program Headers (PHDR数 × 56B)
 *   0x1000 : .text (RX)
 *   page境界: .rodata (R) ← rodataがある場合
 *   page境界: .dynamic (RW) ← PT_DYNAMIC正規化用
 *   末尾    : .shstrtab + セクションヘッダ
 *
 * セグメント構成:
 *   PT_PHDR      : プログラムヘッダ自体の位置
 *   PT_LOAD(RX)  : ELFヘッダ + PHDRs + .text
 *   PT_LOAD(R)   : .rodata（ある場合）
 *   PT_LOAD(RW)  : .dynamic
 *   PT_DYNAMIC   : .dynamicセクションを指す（ET_DYN正規化）
 *   PT_GNU_STACK : NX有効（スタック実行禁止）
 */
static void write_exe(CaiContext *c, const char *path){

    /* ===== .dynamic セクション構築 =====
     * 静的PIEの最小構成: DT_DEBUG + DT_NULL
     * DT_DEBUG: gdb/lldb がデバッグ情報を見つけるために使う
     */
    Elf64Dyn dyn_entries[2];
    dyn_entries[0].d_tag = DT_DEBUG; dyn_entries[0].d_val = 0;
    dyn_entries[1].d_tag = DT_NULL;  dyn_entries[1].d_val = 0;
    uint64_t dynamic_size = sizeof(dyn_entries);

    /* ===== .shstrtab 構築 ===== */
    /* セクション名文字列テーブル */
    const char shstrtab_data[] =
        "\0"          /* index 0: null */
        ".text\0"     /* index 1 */
        ".rodata\0"   /* index 7 */
        ".dynamic\0"  /* index 15 */
        ".shstrtab\0" /* index 24 */
        ;
    uint64_t shstrtab_size = sizeof(shstrtab_data);
    /* 各セクション名のインデックス */
    int sh_name_text     = 1;
    int sh_name_rodata   = 7;
    int sh_name_dynamic  = 15;
    int sh_name_shstrtab = 24;

    /* ===== ファイルレイアウト計算 ===== */

    /* PHDRの数を先に決める:
     * PT_PHDR, PT_LOAD(RX), PT_LOAD(R)[条件付き],
     * PT_LOAD(RW/.dynamic), PT_DYNAMIC, PT_GNU_STACK */
    int has_rodata = (c->rodata_size > 0);
    int phdr_count = 5 + (has_rodata ? 1 : 0);

    uint64_t ehdr_size   = sizeof(ExeEhdr);
    uint64_t phdr_size   = (uint64_t)phdr_count * sizeof(ExePhdr);
    uint64_t headers_end = ehdr_size + phdr_size;

    /* .text: pageアラインされた位置から */
    uint64_t text_off    = align_up(headers_end, PAGE);
    uint64_t text_vma    = text_off;  /* PIE: ベース0x0なのでvma=offset */
    uint64_t text_size   = (uint64_t)c->code_size;

    /* .rodata */
    uint64_t rodata_off  = align_up(text_off + text_size, PAGE);
    uint64_t rodata_vma  = rodata_off;
    uint64_t rodata_size = (uint64_t)c->rodata_size;

    /* .dynamic: rodataの後ろ（rodataなしならtextの後ろ） */
    uint64_t dynamic_off = has_rodata
                         ? align_up(rodata_off + rodata_size, PAGE)
                         : align_up(text_off + text_size, PAGE);
    uint64_t dynamic_vma = dynamic_off;

    /* .shstrtab + セクションヘッダ */
    uint64_t shstrtab_off = align_up(dynamic_off + dynamic_size, 8);
    uint64_t shoff         = align_up(shstrtab_off + shstrtab_size, 8);

    /* セクション数: null + .text + .rodata(条件) + .dynamic + .shstrtab */
    int shnum = 4 + (has_rodata ? 1 : 0);

    uint64_t file_size = shoff + (uint64_t)shnum * sizeof(ExeShdr);

    /* ===== バッファ確保 ===== */
    uint8_t *buf = (uint8_t*)calloc(1, file_size);
    if(!buf){ perror("calloc"); exit(1); }

    /* ===== シンボルVMA確定 ===== */
    for(int i=0;i<c->sym_count;i++){
        if(!c->syms[i].defined) continue;
        if(c->syms[i].off & (1<<30)){
            c->syms[i].off = (int)((c->syms[i].off & ~(1<<30)) + rodata_vma);
        } else {
            c->syms[i].off += (int)text_vma;
        }
    }

    /* ===== パッチ適用 ===== */
    int unresolved = 0;
    for(int i=0;i<c->patch_count;i++){
        int si = sym_find2(c, c->patches[i].sym);
        if(si<0 || !c->syms[si].defined){
            fprintf(stderr,"Link Error: Undefined symbol: %s\n", c->patches[i].sym);
            unresolved++;
            continue;
        }
        uint64_t patch_vma = text_vma + (uint64_t)c->patches[i].code_off;
        int32_t rel = (int32_t)((uint64_t)c->syms[si].off - (patch_vma + 4));
        patch_i32(c, c->patches[i].code_off, rel);
    }
    if(unresolved > 0){
        fprintf(stderr,"Link failed: %d unresolved symbol(s).\n", unresolved);
        free(buf);
        exit(1);
    }

    /* ===== エントリポイント ===== */
    int start_si = sym_find2(c, "_start");
    uint64_t entry = start_si >= 0 ? (uint64_t)c->syms[start_si].off : text_vma;

    /* ===== ELFヘッダ ===== */
    ExeEhdr *eh = (ExeEhdr*)buf;
    memcpy(eh->e_ident, "\x7f" "ELF", 4);
    eh->e_ident[4] = 2;  /* ELFCLASS64 */
    eh->e_ident[5] = 1;  /* ELFDATA2LSB */
    eh->e_ident[6] = 1;  /* EV_CURRENT */
    eh->e_type     = 3;  /* ET_DYN: 静的PIE */
    eh->e_machine  = 62; /* EM_X86_64 */
    eh->e_version  = 1;
    eh->e_entry    = entry;
    eh->e_phoff    = ehdr_size;
    eh->e_ehsize   = (uint16_t)ehdr_size;
    eh->e_phentsize= sizeof(ExePhdr);
    eh->e_phnum    = (uint16_t)phdr_count;
    eh->e_shentsize= sizeof(ExeShdr);
    eh->e_shnum    = (uint16_t)shnum;
    eh->e_shstrndx = (uint16_t)(shnum - 1); /* .shstrtabは最後 */
    eh->e_shoff    = shoff;

    /* ===== Program Headers ===== */
    ExePhdr *ph = (ExePhdr*)(buf + ehdr_size);
    int pi = 0;

    /* PT_PHDR: プログラムヘッダテーブル自体を指す */
    ph[pi].p_type   = PT_PHDR;
    ph[pi].p_flags  = PF_R;
    ph[pi].p_offset = ehdr_size;
    ph[pi].p_vaddr  = ehdr_size;
    ph[pi].p_paddr  = ehdr_size;
    ph[pi].p_filesz = phdr_size;
    ph[pi].p_memsz  = phdr_size;
    ph[pi].p_align  = 8;
    pi++;

    /* PT_LOAD #0: ELFヘッダ + PHDRs + .text (R+X) */
    ph[pi].p_type   = PT_LOAD;
    ph[pi].p_flags  = PF_R | PF_X;
    ph[pi].p_offset = 0;
    ph[pi].p_vaddr  = 0;
    ph[pi].p_paddr  = 0;
    ph[pi].p_filesz = text_off + text_size;
    ph[pi].p_memsz  = text_off + text_size;
    ph[pi].p_align  = PAGE;
    pi++;

    /* PT_LOAD #1: .rodata (R) — ある場合のみ */
    if(has_rodata){
        ph[pi].p_type   = PT_LOAD;
        ph[pi].p_flags  = PF_R;
        ph[pi].p_offset = rodata_off;
        ph[pi].p_vaddr  = rodata_vma;
        ph[pi].p_paddr  = rodata_vma;
        ph[pi].p_filesz = rodata_size;
        ph[pi].p_memsz  = rodata_size;
        ph[pi].p_align  = PAGE;
        pi++;
    }

    /* PT_LOAD #2: .dynamic (R+W) */
    ph[pi].p_type   = PT_LOAD;
    ph[pi].p_flags  = PF_R | PF_W;
    ph[pi].p_offset = dynamic_off;
    ph[pi].p_vaddr  = dynamic_vma;
    ph[pi].p_paddr  = dynamic_vma;
    ph[pi].p_filesz = dynamic_size;
    ph[pi].p_memsz  = dynamic_size;
    ph[pi].p_align  = PAGE;
    pi++;

    /* PT_DYNAMIC: .dynamicセクションを指す（ET_DYN正規化） */
    ph[pi].p_type   = PT_DYNAMIC;
    ph[pi].p_flags  = PF_R | PF_W;
    ph[pi].p_offset = dynamic_off;
    ph[pi].p_vaddr  = dynamic_vma;
    ph[pi].p_paddr  = dynamic_vma;
    ph[pi].p_filesz = dynamic_size;
    ph[pi].p_memsz  = dynamic_size;
    ph[pi].p_align  = 8;
    pi++;

    /* PT_GNU_STACK: NX有効（スタック実行禁止）
     * p_flags に PF_X を含めない = スタック実行禁止
     * filesz/memsz = 0 でよい（スタックサイズはOSが決める） */
    ph[pi].p_type   = PT_GNU_STACK;
    ph[pi].p_flags  = PF_R | PF_W;  /* 実行(PF_X)なし = NX有効 */
    ph[pi].p_offset = 0;
    ph[pi].p_vaddr  = 0;
    ph[pi].p_paddr  = 0;
    ph[pi].p_filesz = 0;
    ph[pi].p_memsz  = 0;
    ph[pi].p_align  = 16;
    pi++;

    /* ===== セクションデータをバッファへ ===== */
    memcpy(buf + text_off, c->code, (size_t)text_size);
    if(has_rodata)
        memcpy(buf + rodata_off, c->rodata, (size_t)rodata_size);
    memcpy(buf + dynamic_off, dyn_entries, (size_t)dynamic_size);
    memcpy(buf + shstrtab_off, shstrtab_data, (size_t)shstrtab_size);

    /* ===== セクションヘッダ ===== */
    ExeShdr *sh = (ExeShdr*)(buf + shoff);
    int si2 = 0;

    /* SHT_NULL */
    memset(&sh[si2], 0, sizeof(ExeShdr));
    si2++;

    /* .text: SHT_PROGBITS, SHF_ALLOC|SHF_EXECINSTR */
    sh[si2].sh_name    = (uint32_t)sh_name_text;
    sh[si2].sh_type    = 1;   /* SHT_PROGBITS */
    sh[si2].sh_flags   = 6;   /* SHF_ALLOC | SHF_EXECINSTR */
    sh[si2].sh_addr    = text_vma;
    sh[si2].sh_off     = text_off;
    sh[si2].sh_size    = text_size;
    sh[si2].sh_align   = 16;
    si2++;

    /* .rodata: SHT_PROGBITS, SHF_ALLOC（ある場合） */
    if(has_rodata){
        sh[si2].sh_name  = (uint32_t)sh_name_rodata;
        sh[si2].sh_type  = 1;  /* SHT_PROGBITS */
        sh[si2].sh_flags = 2;  /* SHF_ALLOC */
        sh[si2].sh_addr  = rodata_vma;
        sh[si2].sh_off   = rodata_off;
        sh[si2].sh_size  = rodata_size;
        sh[si2].sh_align = 8;
        si2++;
    }

    /* .dynamic: SHT_DYNAMIC, SHF_ALLOC|SHF_WRITE */
    sh[si2].sh_name    = (uint32_t)sh_name_dynamic;
    sh[si2].sh_type    = 6;   /* SHT_DYNAMIC */
    sh[si2].sh_flags   = 3;   /* SHF_ALLOC | SHF_WRITE */
    sh[si2].sh_addr    = dynamic_vma;
    sh[si2].sh_off     = dynamic_off;
    sh[si2].sh_size    = dynamic_size;
    sh[si2].sh_align   = 8;
    sh[si2].sh_entsize = sizeof(Elf64Dyn);
    si2++;

    /* .shstrtab: SHT_STRTAB */
    sh[si2].sh_name  = (uint32_t)sh_name_shstrtab;
    sh[si2].sh_type  = 3;  /* SHT_STRTAB */
    sh[si2].sh_off   = shstrtab_off;
    sh[si2].sh_size  = shstrtab_size;
    sh[si2].sh_align = 1;
    si2++;

    /* ===== 書き出し ===== */
    FILE *f = fopen(path, "wb");
    if(!f){ perror("fopen exe"); free(buf); exit(1); }
    fwrite(buf, 1, file_size, f);
    fclose(f);
    free(buf);

    /* chmod +x */
    {
        int fd = open(path, O_RDONLY);
        if(fd >= 0){ fchmod(fd, 0755); close(fd); }
    }
}

/* ===== メイン ===== */
int main(int argc, char *argv[]){
    if(argc<3){ fprintf(stderr,"Usage: cai_conv <input.cai> <output>\n"); return 1; }

    CaiContext *c=ctx_new();

    parse_file(c,argv[1]);

    /* 関数収集 */
    int cur=-1;
    for(int i=0;i<c->instr_count;i++){
        if(c->instrs[i].kind==OP_FUNC){
            cur=c->func_count++;
            strncpy(c->funcs[cur].name,c->instrs[i].dst,MAX_NAME-1);
            c->funcs[cur].is_export=c->instrs[i].is_export;
            c->funcs[cur].instr_start=i+1;
        } else if(c->instrs[i].kind==OP_ENDFUNC&&cur>=0){
            c->funcs[cur].instr_end=i; cur=-1;
        }
    }

    /* コード生成 */
    for(int i=0;i<c->func_count;i++) gen_func(c,&c->funcs[i]);

    emit_itoa64(c);
    emit_runtime(c);

    /* data命令処理: .rodataへ文字列定数を配置 */
    for(int i=0;i<c->instr_count;i++){
        if(c->instrs[i].kind==OP_DATA){
            const char *label=c->instrs[i].dst;
            if(label[0]=='$') label++;
            int off=rodata_add_str(c,c->instrs[i].str_val);
            sym_define(c,label,off|(1<<30),1);
        }
    }

    write_exe(c,argv[2]);

    printf("Binary → %s ✅\n",argv[2]);

    free(c);
    return 0;
}
