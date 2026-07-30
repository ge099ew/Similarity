# Changelog

## v0.1.0 — 2026-07-30 (Prototype)

### 初版リリース

#### 言語機能
- lexer / parser / AST
- typecheck（行番号・列番号付きエラー）
  - TC1001: null安全
  - TC2001〜TC2010: 型ミスマッチ・未宣言変数・配列型違反
  - TC3002: risk{}外でのderef
  - TC4001: 整数オーバーフロー
  - TC5001〜TC5002: share()/Async データ競合検出
- 変数（let/unclet/Mutation）
- 制御フロー（If/Loop/break/continue）
- 関数（Function/Function_public/call/return）
- ポインタ（addr/deref）
- 配列アクセス（index）
- cast（int↔float）
- 構造体（struct）
- Mem[risk{}]（unsafeブロック）
- Async/Await/share()（pthread）
- Error/Fatal（エラーハンドリング）
- Import/Extern（モジュール・FFI）
- シュガーシンタックス（.sml）

#### バックエンド（CAI）
- CAI IR（Common Assembly Instructions）設計・実装
- x86_64機械語直接生成（asを完全排除）
- ELF64直接生成（ldを完全排除）
- 静的PIE（ET_DYN + ASLR対応）
- NX（スタック実行禁止、PT_GNU_STACK）
- セクション分離（.text / .rodata / .dynamic）
- セクションヘッダ（readelf/objdump/gdb対応）
- i32演算（add/sub/mul/div + 6比較）
- i64演算（add64/sub64/mul64/div64 + 6比較）
- f32演算（addf/subf/mulf/divf + itof2/ftoi2、SSE2）
- バイト操作（loadb/storeb）
- ポインタ操作（storep/loadp2/addp）
- syscall直接呼び出し（write/read/open/close/exit等）
- レジスタ割り当て（callee-saved: rbx/r12-r15）
- peephole最適化（EAX追跡）
- GCC完全不要

#### 標準ライブラリ
- math（absolute_value/maximum/minimum/pow_int/clamp）
- io（write/read/open/close/strlen/print）
- core（panic/assert）
- string（str_len/str_compare/str_copy/str_contains_char）
- memory（mem_copy/mem_set/mem_zero/mem_compare）
- sys（exit/getpid/sleep/timestamp）
- time（now_ms/sleep）
- random（seed/next/range）
- process（exit/getpid）
- os（mkdir/remove/rename）

#### サポートシステム
- Echo（project.eho、riskブロック自動レポート）
- Cell（project.cel、依存関係管理）

#### ベンチマーク
- fibonacci(40) vs C++ -O0
- sum(0〜1億) vs C++ -O0
- ipow(2,20)x10k vs C++ -O0
- ackermann(3,7) vs C++ -O0
