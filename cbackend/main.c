/*
 * main.c — sim_backend エントリーポイント
 *
 * Stage 2: BIRファイルを読んでBackendFunctionを構築し、ダンプする。
 *          機械語生成・ELF出力はStage 3以降で実装する。
 *
 * 使い方:
 *   sim_backend <input.bir> <output.elf> [--dump]
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "backend.h"

int main(int argc, char *argv[]) {
    if (argc < 3) {
        fprintf(stderr, "Usage: sim_backend <input.bir> <output.elf> [--dump]\n");
        return 1;
    }

    const char *bir_path = argv[1];
    const char *out_path = argv[2];
    bool do_dump = (argc >= 4 && strcmp(argv[3], "--dump") == 0);

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

    /*
     * Stage 2 完了: BackendFunction構築まで実装。
     * CFG生成・命令選択・レジスタ割り当て・ELF生成はStage 3以降で実装する。
     *
     * 現時点では出力ファイルを空で作成して終了する。
     */
    FILE *out = fopen(out_path, "wb");
    if (!out) {
        fprintf(stderr, "出力ファイルを開けません: %s\n", out_path);
        bir_free(prog);
        return 1;
    }
    fclose(out);

    fprintf(stderr, "Stage 2 complete: BackendFunction built. "
            "CFG/ISel/ELF will be implemented in Stage 3.\n");

    bir_free(prog);
    return 0;
}
