/*
 * CAI Converter v3 - CAIテキスト → x86_64機械語直接生成 → .oファイル → リンク
 * asを完全排除。GCCはリンク(ld)のみ使用。
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <ctype.h>

/* ===== 定数 ===== */
#define MAX_INSTRS   65536
#define MAX_FUNCS    1024
#define MAX_REGS     4096
#define MAX_NAME     128
#define MAX_ARGS     16
#define MAX_LABELS   4096
#define MAX_PATCHES  8192
#define CODE_MAX     (4*1024*1024)

/* ===== 命令種別 ===== */
typedef enum {
    OP_ALLOC, OP_STORE, OP_LOAD,
    OP_ADD, OP_SUB, OP_MUL, OP_DIV,
    OP_CLT, OP_CLE, OP_CEQ, OP_CNE, OP_CGT, OP_CGE,
    OP_LABEL, OP_JMP, OP_JNZ,
    OP_CALL, OP_RET, OP_RETV,
    OP_ITOF, OP_FTOI, OP_MOV,
    OP_EXTERN, OP_FUNC, OP_ENDFUNC,
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
} Instr;

typedef struct {
    char name[MAX_NAME];
    int  instr_start, instr_end;
    int  is_export, is_leaf, param_count, stack_size;
} FuncInfo;

/* ===== レジスタ割り当て ===== */
#define NUM_ALLOC_REGS 5
/* callee-saved: rbx=3, r12=12, r13=13, r14=14, r15=15 */
static const int alloc_phys[NUM_ALLOC_REGS] = {3, 12, 13, 14, 15};

typedef struct {
    char name[MAX_NAME];
    int  stack_off;
    int  phys_reg;   /* -1=スタック、>=0=物理レジスタインデックス */
    int  use_count;
    int  is_ptr;
} VReg;

/* ===== グローバル状態 ===== */
static Instr    instrs[MAX_INSTRS];
static int      instr_count = 0;
static FuncInfo funcs[MAX_FUNCS];
static int      func_count = 0;
static VReg     vregs[MAX_REGS];
static int      vreg_count = 0;
static int      stack_used = 0;
static int      reg_used[NUM_ALLOC_REGS];
static char     eax_holds[MAX_NAME];

/* ===== コードバッファ ===== */
static uint8_t  code[CODE_MAX];
static int      code_size = 0;

/* ===== シンボルテーブル ===== */
typedef struct { char name[MAX_NAME]; int off; int defined; int global; } Sym;
static Sym syms[MAX_FUNCS*2];
static int sym_count = 0;

typedef struct { int code_off; char sym[MAX_NAME]; } Patch;
static Patch patches[MAX_PATCHES];
static int   patch_count = 0;

/* ===== ラベルテーブル（関数内） ===== */
typedef struct { char name[MAX_NAME]; int off; } Label;
static Label labels[MAX_LABELS];
static int   label_count = 0;

typedef struct { int code_off; char name[MAX_NAME]; } LabelPatch;
static LabelPatch lpatches[MAX_PATCHES];
static int        lpatch_count = 0;

/* ===== EAX追跡 ===== */
static void reset_eax(){ eax_holds[0]='\0'; }
static int  eax_has(const char *n){ return n[0]&&!strcmp(eax_holds,n); }
static void set_eax(const char *n){ strncpy(eax_holds,n,MAX_NAME-1); }

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

/* ===== コード生成ヘルパー ===== */
static void emit1(uint8_t b){ code[code_size++]=b; }
static void emit2(uint8_t a,uint8_t b){ code[code_size++]=a; code[code_size++]=b; }
static void emit3(uint8_t a,uint8_t b,uint8_t c){ emit2(a,b); emit1(c); }
static void emit_i32(int32_t v){
    code[code_size++]=v&0xFF; code[code_size++]=(v>>8)&0xFF;
    code[code_size++]=(v>>16)&0xFF; code[code_size++]=(v>>24)&0xFF;
}
static void patch_i32(int off, int32_t v){
    code[off]=v&0xFF; code[off+1]=(v>>8)&0xFF;
    code[off+2]=(v>>16)&0xFF; code[off+3]=(v>>24)&0xFF;
}

/* ModRM */
static uint8_t modrm(int mod,int reg,int rm){ return (mod<<6)|((reg&7)<<3)|(rm&7); }
/* REX */
static uint8_t rex(int w,int r,int x,int b){ return 0x40|(w?8:0)|(r?4:0)|(x?2:0)|(b?1:0); }

/* レジスタ番号 */
#define EAX 0
#define ECX 1
#define EDX 2
#define EBX 3
#define RSP 4
#define RBP 5
#define ESI 6
#define EDI 7
/* R8=8..R15=15 */
/* 引数レジスタ（64bit） */
static const int argregs64[]={7,6,2,1,8,9}; /* rdi,rsi,rdx,rcx,r8,r9 */

/* phys_regインデックス→実レジスタ番号 */
static int phys_to_reg(int pi){ return alloc_phys[pi]; }

/* 32bit版レジスタ名 */
static const char *r32name[]={
    "eax","ecx","edx","ebx","esp","ebp","esi","edi",
    "r8d","r9d","r10d","r11d","r12d","r13d","r14d","r15d"
};

/* push reg64 */
static void emit_push(int r){
    if(r>=8){ emit1(0x41); emit1(0x50+(r-8)); } else emit1(0x50+r);
}
/* pop reg64 */
static void emit_pop(int r){
    if(r>=8){ emit1(0x41); emit1(0x58+(r-8)); } else emit1(0x58+r);
}

/* mov r32, imm32 */
static void emit_mov_r32_imm(int r, int32_t imm){
    if(r>=8) emit1(rex(0,0,0,1));
    emit1(0xB8+(r&7)); emit_i32(imm);
}

/* mov [rbp+off], r32 */
static void emit_store_r32(int r, int off){
    if(r>=8) emit1(rex(0,1,0,0));
    emit1(0x89);
    if(off>=-128&&off<=127){ emit1(modrm(1,r&7,RBP)); emit1((int8_t)off); }
    else { emit1(modrm(2,r&7,RBP)); emit_i32(off); }
}

/* mov r32, [rbp+off] */
static void emit_load_r32(int r, int off){
    if(r>=8) emit1(rex(0,1,0,0));
    emit1(0x8B);
    if(off>=-128&&off<=127){ emit1(modrm(1,r&7,RBP)); emit1((int8_t)off); }
    else { emit1(modrm(2,r&7,RBP)); emit_i32(off); }
}

/* mov r32, r32 */
static void emit_mov_r32(int dst, int src){
    if(dst>=8||src>=8) emit1(rex(0,src>=8,0,dst>=8));
    emit1(0x89); emit1(modrm(3,src&7,dst&7));
}

/* ===== vreg操作 ===== */
static int find_vreg(const char *n){
    for(int i=0;i<vreg_count;i++) if(!strcmp(vregs[i].name,n)) return i;
    return -1;
}
static int alloc_slot(const char *n){
    int i=find_vreg(n); if(i>=0) return i;
    stack_used+=8; vregs[vreg_count].stack_off=-stack_used;
    vregs[vreg_count].phys_reg=-1; vregs[vreg_count].use_count=0;
    vregs[vreg_count].is_ptr=strstr(n,".ptr")!=NULL;
    strncpy(vregs[vreg_count].name,n,MAX_NAME-1);
    return vreg_count++;
}

/* EAXをvregに保存 */
static void store_eax_to(const char *dst){
    int i=find_vreg(dst); if(i<0) i=alloc_slot(dst);
    if(vregs[i].phys_reg>=0){
        int pr=phys_to_reg(vregs[i].phys_reg);
        if(pr!=EAX) emit_mov_r32(pr,EAX);
        /* pr==EAX の場合は no-op（EAXがそのまま物理レジスタ） */
    } else {
        emit_store_r32(EAX, vregs[i].stack_off);
    }
    set_eax(dst);
}

/* vregをEAXに読み込む（peephole付き） */
static void load_to_eax(const char *val){
    if(eax_has(val)) return;
    if(val[0]=='%'){
        int i=find_vreg(val);
        if(i>=0){
            if(vregs[i].phys_reg>=0){
                int pr=phys_to_reg(vregs[i].phys_reg);
                if(pr==EAX){
                    /* 既にEAXと同じレジスタ → no-op */
                } else {
                    emit_mov_r32(EAX,pr);
                }
            } else {
                emit_load_r32(EAX, vregs[i].stack_off);
            }
        } else {
            emit1(0x31); emit1(0xC0); /* xor eax,eax */
        }
    } else if(is_imm(val)){
        int32_t v=atoi(val);
        if(v==0){ emit1(0x31); emit1(0xC0); }
        else emit_mov_r32_imm(EAX, v);
    } else {
        emit1(0x31); emit1(0xC0);
    }
    set_eax(val);
}

/* ===== パーサー ===== */
static void parse_line(char *line){
    trim(line);
    if(line[0]=='#'||line[0]=='\0'){ instrs[instr_count++].kind=OP_COMMENT; return; }
    Instr *ins=&instrs[instr_count]; memset(ins,0,sizeof(Instr));
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
    } else { ins->kind=OP_COMMENT; }
    #undef NEXT
    instr_count++;
}

static void parse_file(const char *path){
    FILE *f=fopen(path,"r"); if(!f){perror("fopen");exit(1);}
    char line[512];
    while(fgets(line,sizeof(line),f)) parse_line(line);
    fclose(f);
}

/* ===== 関数解析 ===== */
static int is_leaf(int s,int e){ for(int i=s;i<e;i++) if(instrs[i].kind==OP_CALL) return 0; return 1; }
static int count_params(int s,int e){
    int mx=-1;
    for(int i=s;i<e;i++){
        Instr *ins=&instrs[i];
        char *ptrs[]={ins->dst,ins->a,ins->b};
        for(int j=0;j<3;j++) if(strncmp(ptrs[j],"%arg",4)==0){ int n=atoi(ptrs[j]+4); if(n>mx) mx=n; }
    }
    return mx+1;
}
static void count_uses(int s,int e){
    for(int i=s;i<e;i++){
        Instr *ins=&instrs[i];
        for(int j=0;j<vreg_count;j++){
            if(!strcmp(ins->dst,vregs[j].name)) vregs[j].use_count++;
            if(!strcmp(ins->a,vregs[j].name))   vregs[j].use_count++;
            if(!strcmp(ins->b,vregs[j].name))   vregs[j].use_count++;
            for(int k=0;k<ins->argc;k++) if(!strcmp(ins->args[k],vregs[j].name)) vregs[j].use_count++;
        }
    }
}

static void init_regalloc(FuncInfo *fn){
    vreg_count=0; stack_used=0;
    memset(reg_used,0,sizeof(reg_used));

    /* 全仮想レジスタ収集 */
    for(int i=fn->instr_start;i<fn->instr_end;i++){
        Instr *ins=&instrs[i];
        const char *ns[]={ins->dst,ins->a,ins->b};
        for(int j=0;j<3;j++){
            const char *n=ns[j]; if(n[0]!='%') continue;
            if(find_vreg(n)<0&&vreg_count<MAX_REGS){
                strncpy(vregs[vreg_count].name,n,MAX_NAME-1);
                vregs[vreg_count].phys_reg=-1; vregs[vreg_count].use_count=0;
                vregs[vreg_count].is_ptr=strstr(n,".ptr")!=NULL;
                vreg_count++;
            }
        }
        for(int j=0;j<ins->argc;j++){
            const char *n=ins->args[j]; if(n[0]!='%') continue;
            if(find_vreg(n)<0&&vreg_count<MAX_REGS){
                strncpy(vregs[vreg_count].name,n,MAX_NAME-1);
                vregs[vreg_count].phys_reg=-1; vregs[vreg_count].use_count=0;
                vregs[vreg_count].is_ptr=strstr(n,".ptr")!=NULL;
                vreg_count++;
            }
        }
    }
    count_uses(fn->instr_start,fn->instr_end);

    /* leaf関数のみレジスタ割り当て（use_count>=2、非ポインタ、非引数） */
    if(fn->is_leaf){
        for(int i=0;i<vreg_count-1;i++)
            for(int j=i+1;j<vreg_count;j++)
                if(vregs[j].use_count>vregs[i].use_count){ VReg t=vregs[i];vregs[i]=vregs[j];vregs[j]=t; }
        int ri=0;
        for(int i=0;i<vreg_count&&ri<NUM_ALLOC_REGS;i++)
            if(vregs[i].use_count>=2&&!vregs[i].is_ptr&&strncmp(vregs[i].name,"%arg",4)){
                vregs[i].phys_reg=ri++; reg_used[vregs[i].phys_reg]=1;
            }
    }

    /* スタック割り当て */
    for(int i=0;i<vreg_count;i++){
        if(vregs[i].phys_reg>=0) continue;
        stack_used+=8; vregs[i].stack_off=-stack_used;
    }
    /* 引数スロット追加 */
    for(int i=0;i<6;i++){
        char an[MAX_NAME]; snprintf(an,MAX_NAME,"%%arg%d",i);
        if(find_vreg(an)<0&&vreg_count<MAX_REGS){
            strncpy(vregs[vreg_count].name,an,MAX_NAME-1);
            stack_used+=8; vregs[vreg_count].stack_off=-stack_used;
            vregs[vreg_count].phys_reg=-1; vreg_count++;
        }
    }
    stack_used=(stack_used+15)&~15;
    fn->stack_size=stack_used;
}

/* ===== ラベル操作 ===== */
static void label_def(const char *name){
    strncpy(labels[label_count].name,name,MAX_NAME-1);
    labels[label_count].off=code_size;
    label_count++;
}
static int label_find(const char *name){
    for(int i=0;i<label_count;i++) if(!strcmp(labels[i].name,name)) return labels[i].off;
    return -1;
}
static void lpatch_add(int coff, const char *name){
    strncpy(lpatches[lpatch_count].name,name,MAX_NAME-1);
    lpatches[lpatch_count].code_off=coff;
    lpatch_count++;
}
static void resolve_labels(){
    for(int i=0;i<lpatch_count;i++){
        int off=label_find(lpatches[i].name);
        if(off<0){ fprintf(stderr,"未解決ラベル: %s\n",lpatches[i].name); continue; }
        int32_t rel=off-(lpatches[i].code_off+4);
        patch_i32(lpatches[i].code_off, rel);
    }
}

/* ===== シンボル操作 ===== */
static void sym_define(const char *name, int off, int global){
    int i; for(i=0;i<sym_count;i++) if(!strcmp(syms[i].name,name)) break;
    if(i==sym_count){ strncpy(syms[i].name,name,MAX_NAME-1); sym_count++; }
    syms[i].off=off; syms[i].defined=1; syms[i].global=global;
}
static int sym_find2(const char *name){
    for(int i=0;i<sym_count;i++) if(!strcmp(syms[i].name,name)) return i;
    return -1;
}
static void sym_ref(const char *name){
    if(sym_find2(name)<0){
        strncpy(syms[sym_count].name,name,MAX_NAME-1);
        syms[sym_count].defined=0; syms[sym_count].global=1; sym_count++;
    }
}

/* ===== 関数コード生成 ===== */
static void gen_func(FuncInfo *fn){
    fn->is_leaf=is_leaf(fn->instr_start,fn->instr_end);
    fn->param_count=count_params(fn->instr_start,fn->instr_end);
    init_regalloc(fn);

    /* 関数名（main→sim_main） */
    char fname[MAX_NAME];
    const char *raw=fn->name; if(raw[0]=='$') raw++;
    strncpy(fname, !strcmp(raw,"main")?"sim_main":raw, MAX_NAME-1);
    if(!strcmp(raw,"main")) fn->is_export=1;

    sym_define(fname, code_size, fn->is_export);

    /* プロローグ */
    emit1(0x55);                         /* push rbp */
    emit2(0x48,0x89); emit1(modrm(3,RSP,RBP)); /* mov rbp,rsp */
    /* sub rsp, stack_size+16 */
    int ss=fn->stack_size+64;
    emit3(0x48,0x81,0xEC); emit_i32(ss);

    /* callee-saved退避 */
    for(int i=0;i<NUM_ALLOC_REGS;i++) if(reg_used[i]) emit_push(phys_to_reg(i));

    /* 引数を退避 */
    for(int i=0;i<fn->param_count&&i<6;i++){
        char an[MAX_NAME]; snprintf(an,MAX_NAME,"%%arg%d",i);
        int idx=find_vreg(an);
        if(idx>=0){
            if(vregs[idx].phys_reg>=0)
                emit_mov_r32(phys_to_reg(vregs[idx].phys_reg), argregs64[i]);
            else
                emit_store_r32(argregs64[i], vregs[idx].stack_off);
        }
    }

    reset_eax();
    label_count=0; lpatch_count=0;

    for(int i=fn->instr_start;i<fn->instr_end;i++){
        Instr *ins=&instrs[i];
        switch(ins->kind){
        case OP_COMMENT: break;
        case OP_ALLOC: { int idx=find_vreg(ins->dst); if(idx<0) alloc_slot(ins->dst); break; }

        case OP_STORE: {
            load_to_eax(ins->a);
            int idx=find_vreg(ins->dst); if(idx<0) idx=alloc_slot(ins->dst);
            if(vregs[idx].phys_reg>=0){
                int pr=phys_to_reg(vregs[idx].phys_reg);
                if(pr!=EAX) emit_mov_r32(pr,EAX);
                /* スタックには書かない */
            } else {
                emit_store_r32(EAX, vregs[idx].stack_off);
            }
            set_eax(ins->dst);
            break;
        }

        case OP_LOAD: {
            int si=find_vreg(ins->a);
            if(eax_has(ins->a)){
                store_eax_to(ins->dst);
            } else if(si>=0){
                if(vregs[si].phys_reg>=0){ int pr=phys_to_reg(vregs[si].phys_reg); if(pr!=EAX) emit_mov_r32(EAX,pr); }
                else emit_load_r32(EAX, vregs[si].stack_off);
                store_eax_to(ins->dst);
            } else {
                emit1(0x31); emit1(0xC0);
                store_eax_to(ins->dst);
            }
            break;
        }

        case OP_ADD: {
            load_to_eax(ins->a);
            if(is_imm(ins->b)){
                int v=atoi(ins->b);
                if(v==1){ emit1(0xFF); emit1(0xC0); } /* inc eax */
                else if(v==-1){ emit1(0xFF); emit1(0xC8); } /* dec eax */
                else if(v>=-128&&v<=127){ emit2(0x83,0xC0); emit1((int8_t)v); }
                else { emit1(0x05); emit_i32(v); }
            } else {
                emit_mov_r32(ECX,EAX);
                load_to_eax(ins->b);
                emit2(0x01,0xC8); /* add eax,ecx */
            }
            reset_eax(); store_eax_to(ins->dst);
            break;
        }

        case OP_SUB: {
            if(is_imm(ins->a)&&atoi(ins->a)==0){
                load_to_eax(ins->b);
                emit2(0xF7,0xD8); /* neg eax */
            } else {
                load_to_eax(ins->a);
                if(is_imm(ins->b)){
                    int v=atoi(ins->b);
                    if(v==1){ emit1(0xFF); emit1(0xC8); }
                    else if(v>=-128&&v<=127){ emit2(0x83,0xE8); emit1((int8_t)v); }
                    else { emit1(0x2D); emit_i32(v); }
                } else {
                    emit_mov_r32(ECX,EAX);
                    load_to_eax(ins->b);
                    emit2(0x29,0xC1); /* sub ecx,eax */
                    emit2(0x89,0xC8); /* mov eax,ecx */
                }
            }
            reset_eax(); store_eax_to(ins->dst);
            break;
        }

        case OP_MUL: {
            load_to_eax(ins->a);
            emit_mov_r32(ECX,EAX);
            load_to_eax(ins->b);
            emit3(0x0F,0xAF,0xC1); /* imul eax,ecx */
            reset_eax(); store_eax_to(ins->dst);
            break;
        }

        case OP_DIV: {
            load_to_eax(ins->a);
            emit_mov_r32(ECX,EAX);
            load_to_eax(ins->b);
            emit2(0x87,0xC1); /* xchg eax,ecx */
            emit1(0x99);      /* cdq */
            emit2(0xF7,0xF9); /* idiv ecx */
            reset_eax(); store_eax_to(ins->dst);
            break;
        }

        case OP_CLT: case OP_CLE: case OP_CEQ:
        case OP_CNE: case OP_CGT: case OP_CGE: {
            load_to_eax(ins->a);
            if(is_imm(ins->b)){
                int v=atoi(ins->b);
                if(v>=-128&&v<=127){ emit2(0x83,0xF8); emit1((int8_t)v); }
                else { emit1(0x3D); emit_i32(v); }
            } else {
                emit_mov_r32(ECX,EAX);
                load_to_eax(ins->b);
                emit2(0x39,0xC1); /* cmp ecx,eax */
            }
            uint8_t cc[]={0x9C,0x9E,0x94,0x95,0x9F,0x9D};
            int ci=ins->kind-OP_CLT;
            emit3(0x0F,cc[ci],0xC0); /* setcc al */
            emit3(0x0F,0xB6,0xC0);   /* movzx eax,al */
            reset_eax(); store_eax_to(ins->dst);
            break;
        }

        case OP_LABEL:
            /* ラベル到達時、物理レジスタに入っている値はそのまま有効
               スタックの値は不明（他のパスから来る可能性）*/
            reset_eax();
            label_def(ins->dst);
            break;

        case OP_JMP:
            reset_eax();
            { emit1(0xE9); int po=code_size; emit_i32(0); lpatch_add(po,ins->dst); }
            break;

        case OP_JNZ: {
            load_to_eax(ins->dst);
            emit2(0x85,0xC0); /* test eax,eax */
            /* jnz true */
            emit2(0x0F,0x85); int pt=code_size; emit_i32(0); lpatch_add(pt,ins->a);
            /* jmp false */
            emit1(0xE9); int pf=code_size; emit_i32(0); lpatch_add(pf,ins->b);
            reset_eax();
            break;
        }

        case OP_CALL: {
            /* callee-saved退避 */
            for(int j=0;j<NUM_ALLOC_REGS;j++) if(reg_used[j]) emit_push(phys_to_reg(j));
            /* 引数設定 */
            for(int j=0;j<ins->argc&&j<6;j++){
                load_to_eax(ins->args[j]);
                /* movsx arg_reg64, eax */
                int ar=argregs64[j];
                emit1(ar>=8?0x4C:0x48); emit1(0x63);
                emit1(modrm(3,ar&7,EAX));
            }
            reset_eax();
            /* call rel32 */
            const char *callee=ins->a; if(callee[0]=='$') callee++;
            char cn[MAX_NAME]; strncpy(cn,!strcmp(callee,"main")?"sim_main":callee,MAX_NAME-1);
            sym_ref(cn);
            emit1(0xE8); int po=code_size; emit_i32(0);
            patches[patch_count].code_off=po;
            strncpy(patches[patch_count].sym,cn,MAX_NAME-1);
            patch_count++;
            /* callee-saved復元 */
            for(int j=NUM_ALLOC_REGS-1;j>=0;j--) if(reg_used[j]) emit_pop(phys_to_reg(j));
            if(ins->dst[0]&&ins->dst[0]!='_') store_eax_to(ins->dst);
            else reset_eax();
            break;
        }

        case OP_RET: {
            load_to_eax(ins->dst);
            /* movsx rax, eax */
            emit3(0x48,0x63,0xC0);
            for(int j=NUM_ALLOC_REGS-1;j>=0;j--) if(reg_used[j]) emit_pop(phys_to_reg(j));
            emit1(0xC9); emit1(0xC3); /* leave; ret */
            reset_eax();
            break;
        }

        case OP_RETV:
            emit2(0x31,0xC0); /* xor eax,eax */
            for(int j=NUM_ALLOC_REGS-1;j>=0;j--) if(reg_used[j]) emit_pop(phys_to_reg(j));
            emit1(0xC9); emit1(0xC3);
            reset_eax();
            break;

        default: break;
        }
    }

    resolve_labels();
}

/* ===== ELFオブジェクトファイル(.o)生成 ===== */
/* ld互換のrelocatable ELF64を出力 */
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
typedef struct {
    uint64_t r_offset; uint32_t r_type; int32_t r_addend;
    /* actually r_info=uint64 but we split for clarity */
} Elf64Rela_raw;

/* リロケーション: R_X86_64_PC32 = 2 */
#define R_X86_64_PC32 2
#define R_X86_64_PLT32 4

static void write_obj(const char *path){
    /* セクション: null, .text, .rela.text, .symtab, .strtab, .shstrtab */
    /* 1. シンボル文字列テーブルを構築 */
    uint8_t strtab[65536]; int strtab_size=1; strtab[0]=0;
    int sym_stridx[MAX_FUNCS*2];
    for(int i=0;i<sym_count;i++){
        sym_stridx[i]=strtab_size;
        int len=strlen(syms[i].name);
        memcpy(strtab+strtab_size, syms[i].name, len+1);
        strtab_size+=len+1;
    }

    /* 2. シンボルテーブル構築 */
    /* 最初にlocal（STB_LOCAL）、次にglobal（STB_GLOBAL） */
    Elf64Sym elf_syms[MAX_FUNCS*2+1];
    int elf_sym_count=0;
    memset(&elf_syms[0],0,sizeof(Elf64Sym)); elf_sym_count=1; /* null sym */
    int first_global=1;
    /* local syms */
    for(int i=0;i<sym_count;i++){
        if(syms[i].global) continue;
        Elf64Sym *s=&elf_syms[elf_sym_count++];
        s->st_name=sym_stridx[i];
        s->st_info=(0<<4)|2; /* STB_LOCAL|STT_FUNC */
        s->st_shndx=1; /* .text */
        s->st_value=syms[i].defined?syms[i].off:0;
        s->st_size=0;
    }
    first_global=elf_sym_count;
    /* global syms */
    for(int i=0;i<sym_count;i++){
        if(!syms[i].global) continue;
        Elf64Sym *s=&elf_syms[elf_sym_count++];
        s->st_name=sym_stridx[i];
        s->st_info=(1<<4)|2; /* STB_GLOBAL|STT_FUNC */
        s->st_shndx=syms[i].defined?1:0; /* 1=.text, 0=UND */
        s->st_value=syms[i].defined?syms[i].off:0;
        s->st_size=0;
    }

    /* グローバルシンボルのelf_sym内インデックスを取得する関数 */
    /* パッチ適用のためにシンボル名→elfシンボルインデックスが必要 */
    /* 3. リロケーションテーブル構築 */
    typedef struct { uint64_t r_offset; uint64_t r_info; int64_t r_addend; } Rela64;
    Rela64 relas[MAX_PATCHES]; int rela_count=0;
    for(int i=0;i<patch_count;i++){
        /* シンボルをelfシンボルテーブルで探す */
        int esi=-1;
        for(int j=1;j<elf_sym_count;j++)
            if(!strcmp(strtab+elf_syms[j].st_name, patches[i].sym)){ esi=j; break; }
        if(esi<0){
            /* 未登録グローバルシンボルを追加 */
            Elf64Sym *s=&elf_syms[elf_sym_count];
            /* strtabに追加 */
            int nl=strlen(patches[i].sym);
            int nsi=strtab_size;
            memcpy(strtab+strtab_size,patches[i].sym,nl+1); strtab_size+=nl+1;
            s->st_name=nsi;
            s->st_info=(1<<4)|2; s->st_shndx=0; s->st_value=0;
            esi=elf_sym_count++;
        }
        relas[rela_count].r_offset=patches[i].code_off;
        relas[rela_count].r_info=((uint64_t)esi<<32)|R_X86_64_PLT32;
        relas[rela_count].r_addend=-4;
        rela_count++;
    }

    /* 4. セクション文字列テーブル */
    const char shstrtab[]="\0.text\0.rela.text\0.symtab\0.strtab\0.shstrtab\0";
    int sh_text=1, sh_rela=7, sh_sym=18, sh_str=26, sh_shstr=34;

    /* 5. レイアウト計算 */
    uint64_t off=sizeof(Elf64Ehdr);
    /* no program headers in .o */
    uint64_t text_off=off; uint64_t text_sz=code_size; off+=text_sz;
    off=(off+7)&~7;
    uint64_t rela_off=off; uint64_t rela_sz=rela_count*sizeof(Rela64); off+=rela_sz;
    off=(off+7)&~7;
    uint64_t sym_off=off;  uint64_t sym_sz=elf_sym_count*sizeof(Elf64Sym); off+=sym_sz;
    off=(off+7)&~7;
    uint64_t str_off=off;  uint64_t str_sz=strtab_size; off+=str_sz;
    off=(off+7)&~7;
    uint64_t shstr_off=off; uint64_t shstr_sz=sizeof(shstrtab); off+=shstr_sz;
    off=(off+7)&~7;
    uint64_t shoff=off;

    /* 6. ELFヘッダ */
    FILE *f=fopen(path,"wb"); if(!f){perror("fopen obj");return;}
    Elf64Ehdr eh; memset(&eh,0,sizeof(eh));
    memcpy(eh.e_ident,"\x7f" "ELF",4);
    eh.e_ident[4]=2;eh.e_ident[5]=1;eh.e_ident[6]=1;
    eh.e_type=1; /* ET_REL */
    eh.e_machine=62; /* EM_X86_64 */
    eh.e_version=1;
    eh.e_ehsize=sizeof(Elf64Ehdr);
    eh.e_shentsize=sizeof(Elf64Shdr);
    eh.e_shnum=6; /* null,.text,.rela.text,.symtab,.strtab,.shstrtab */
    eh.e_shstrndx=5;
    eh.e_shoff=shoff;
    fwrite(&eh,sizeof(eh),1,f);

    /* 7. セクションデータ */
    fwrite(code,1,code_size,f);
    /* padding */
    uint8_t zeros[16]={0};
    int pad=(int)(rela_off-(text_off+text_sz)); if(pad>0) fwrite(zeros,1,pad,f);
    fwrite(relas,sizeof(Rela64),rela_count,f);
    pad=(int)(sym_off-(rela_off+rela_sz)); if(pad>0) fwrite(zeros,1,pad,f);
    fwrite(elf_syms,sizeof(Elf64Sym),elf_sym_count,f);
    pad=(int)(str_off-(sym_off+sym_sz)); if(pad>0) fwrite(zeros,1,pad,f);
    fwrite(strtab,1,strtab_size,f);
    pad=(int)(shstr_off-(str_off+str_sz)); if(pad>0) fwrite(zeros,1,pad,f);
    fwrite(shstrtab,1,shstr_sz,f);
    pad=(int)(shoff-(shstr_off+shstr_sz)); if(pad>0) fwrite(zeros,1,pad,f);

    /* 8. セクションヘッダ */
    Elf64Shdr shdrs[6]; memset(shdrs,0,sizeof(shdrs));
    /* null */
    /* .text */
    shdrs[1].sh_name=sh_text; shdrs[1].sh_type=1; /* SHT_PROGBITS */
    shdrs[1].sh_flags=6; /* SHF_ALLOC|SHF_EXECINSTR */
    shdrs[1].sh_off=text_off; shdrs[1].sh_size=text_sz; shdrs[1].sh_align=16;
    /* .rela.text */
    shdrs[2].sh_name=sh_rela; shdrs[2].sh_type=4; /* SHT_RELA */
    shdrs[2].sh_flags=0x40; /* SHF_INFO_LINK */
    shdrs[2].sh_off=rela_off; shdrs[2].sh_size=rela_sz;
    shdrs[2].sh_link=3; shdrs[2].sh_info=1; /* link=.symtab, info=.text idx */
    shdrs[2].sh_align=8; shdrs[2].sh_entsize=sizeof(Rela64);
    /* .symtab */
    shdrs[3].sh_name=sh_sym; shdrs[3].sh_type=2; /* SHT_SYMTAB */
    shdrs[3].sh_off=sym_off; shdrs[3].sh_size=sym_sz;
    shdrs[3].sh_link=4; shdrs[3].sh_info=first_global;
    shdrs[3].sh_align=8; shdrs[3].sh_entsize=sizeof(Elf64Sym);
    /* .strtab */
    shdrs[4].sh_name=sh_str; shdrs[4].sh_type=3; /* SHT_STRTAB */
    shdrs[4].sh_off=str_off; shdrs[4].sh_size=str_sz; shdrs[4].sh_align=1;
    /* .shstrtab */
    shdrs[5].sh_name=sh_shstr; shdrs[5].sh_type=3;
    shdrs[5].sh_off=shstr_off; shdrs[5].sh_size=shstr_sz; shdrs[5].sh_align=1;
    fwrite(shdrs,sizeof(Elf64Shdr),6,f);
    fclose(f);
}

/* ===== タイマーヘルパーC ===== */
static void write_helper(const char *path){
    FILE *f=fopen(path,"w"); if(!f){perror("helper");return;}
    fprintf(f,
        "#include <stdio.h>\n#include <time.h>\n"
        "extern long sim_main();\n"
        "int main(){\n"
        "  struct timespec s,e;\n"
        "  clock_gettime(1,&s);\n"
        "  long r=sim_main();\n"
        "  clock_gettime(1,&e);\n"
        "  long ms=(e.tv_sec-s.tv_sec)*1000+(e.tv_nsec-s.tv_nsec)/1000000;\n"
        "  printf(\"Similarity result: %%ld  time: %%ldms\\n\",r,ms);\n"
        "  return 0;\n}\n");
    fclose(f);
}

/* ===== メイン ===== */
int main(int argc,char *argv[]){
    if(argc<3){ fprintf(stderr,"Usage: cai_conv <input.cai> <output>\n"); return 1; }
    parse_file(argv[1]);

    /* 関数収集 */
    int cur=-1;
    for(int i=0;i<instr_count;i++){
        if(instrs[i].kind==OP_FUNC){
            cur=func_count++;
            strncpy(funcs[cur].name,instrs[i].dst,MAX_NAME-1);
            funcs[cur].is_export=instrs[i].is_export;
            funcs[cur].instr_start=i+1;
        } else if(instrs[i].kind==OP_ENDFUNC&&cur>=0){
            funcs[cur].instr_end=i; cur=-1;
        }
    }

    /* コード生成 */
    for(int i=0;i<func_count;i++) gen_func(&funcs[i]);

    /* .oファイル出力 */
    char obj_file[4096];
    snprintf(obj_file,sizeof(obj_file),"%s.o",argv[2]);
    write_obj(obj_file);

    /* タイマーヘルパー */
    char helper_c[4096];
    snprintf(helper_c,sizeof(helper_c),"%s_helper.c",argv[2]);
    write_helper(helper_c);

    /* GCC でリンクのみ */
    char cmd[16384];
    snprintf(cmd,sizeof(cmd),"gcc -no-pie -o %s %s %s -lc 2>&1",argv[2],obj_file,helper_c);
    int ret=system(cmd);
    remove(helper_c);
    if(ret!=0){ fprintf(stderr,"リンクエラー\n"); return 1; }
    printf("Binary → %s ✅\n",argv[2]);
    return 0;
}
