/*
 * main.c — sim_backend エントリーポイント
 *
 * Stage 3: BIRファイルを読んでBackendFunctionを構築し、CFGを生成する。
 *          機械語生成・ELF出力はStage 4以降で実装する。
 *
 * 使い方:
 *   sim_backend <input.bir> <output.elf> [options]
 *
 * options:
 *   --dump          BackendFunctionの内容を表示
 *   --dump-cfg      CFGを表示
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "backend.h"
#include "cfg.h"

int main(int argc, char *argv[]) {
    if (argc < 3) {
        fprintf(stderr, "Usage: sim_backend <input.bir> <output.elf> [--dump] [--dump-cfg]\n");
        return 1;
    }

    const char *bir_path = argv[1];
    const char *out_path = argv[2];

    bool do_dump     = false;
    bool do_dump_cfg = false;
    for (int i = 3; i < argc; i++) {
        if (strcmp(argv[i], "--dump")     == 0) do_dump     = true;
        if (strcmp(argv[i], "--dump-cfg") == 0) do_dump_cfg = true;
    }

    /* BIRファイルを読んでBackendProgramを構築 */
    BackendProgram *prog = bir_parse(bir_path);
    if (!prog) {
        fprintf(stderr, "BIRパース失敗: %s\n", bir_path);
        return 1;
    }

    fprintf(stderr, "BackendProgram: %d functions loaded from %s\n",
            prog->func_count, bir_path);

    /* --dump: BackendFunctionの内容を表示 */
    if (do_dump) {
        bir_dump(prog);
    }

    /* CFGを構築 */
    CFGProgram *cfgprog = cfg_build_program(prog);

    /* --dump-cfg: CFGを表示 */
    if (do_dump_cfg) {
        cfg_program_dump(cfgprog);
    }

    /*
     * Stage 3 完了: CFG構築まで実装。
     * Instruction Selection / VReg / Liveness / RegAlloc / x86-64 / ELF
     * は Stage 4 以降で実装する。
     */
    FILE *out = fopen(out_path, "wb");
    if (!out) {
        fprintf(stderr, "出力ファイルを開けません: %s\n", out_path);
        cfg_program_free(cfgprog);
        bir_free(prog);
        return 1;
    }
    fclose(out);

    fprintf(stderr, "Stage 3 complete: CFG built (%d functions).\n",
            cfgprog->ncfgs);

    cfg_program_free(cfgprog);
    bir_free(prog);
    return 0;
}
