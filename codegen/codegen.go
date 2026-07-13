// Package codegen: AST → QBE IR を生成する
package codegen

import (
	"fmt"
	"similarity/ast"
	"strings"
)

// Similarityの型 → QBEの型
var typeMap = map[string]string{
	"int":         "w",
	"float":       "s",
	"bool":        "w",
	"String":      "l",
	"Box_int":     "l",
	"Box_float":   "l",
	"Array_int":   "l",
	"Array_float": "l",
	"Array_bool":  "l",
}

// 型のバイトサイズ
var typeSizeMap = map[string]int{
	"int":   4,
	"float": 4,
	"bool":  4,
	"String": 8,
	"Box_int":     8,
	"Box_float":   8,
	"Array_int":   8,
	"Array_float": 8,
	"Array_bool":  8,
}

type Codegen struct {
	buf          strings.Builder
	counter      int
	vars         map[string]string // 変数名 → ptrレジスタ名
	varTypes     map[string]string // 変数名 → Similarity型
	params       map[string]string // パラメータ名 → QBEレジスタ名
	loopEndLabel string
	loopLabel    string
	asyncCounter int // goroutine ID用
}

func New() *Codegen {
	return &Codegen{
		vars:     make(map[string]string),
		varTypes: make(map[string]string),
		params:   make(map[string]string),
	}
}

func (c *Codegen) fresh(prefix string) string {
	c.counter++
	return fmt.Sprintf("%s%d", prefix, c.counter)
}

func (c *Codegen) emit(format string, args ...interface{}) {
	fmt.Fprintf(&c.buf, format+"\n", args...)
}

func (c *Codegen) Generate(prog *ast.Program) string {
	c.buf.Reset()

	if prog.Explanation != nil {
		c.emit("# Similarity - %s", prog.Explanation.Category)
		for k, v := range prog.Explanation.Args {
			c.emit("# %s: %s", k, v)
		}
		if prog.Explanation.Category == "System" {
			if v := prog.Explanation.Args["value"]; v == "HFT" {
				c.emit("# HFTモード: 自動Wait挿入なし")
			}
		}
		c.emit("")
	}

	for _, stmt := range prog.Statements {
		c.genTopLevel(stmt)
	}

	return c.buf.String()
}

func (c *Codegen) genTopLevel(node ast.Node) {
	switch n := node.(type) {
	case *ast.FuncNode:
		c.genFunc(n)
	case *ast.ImportNode:
		c.emit("# import %s", n.Module)
	case *ast.ExternNode:
		for _, lib := range n.Libs {
			c.emit("# extern lib: %s", lib)
		}
	}
}

func (c *Codegen) genFunc(fn *ast.FuncNode) {
	c.vars = make(map[string]string)
	c.varTypes = make(map[string]string)
	c.params = make(map[string]string)

	var params []string
	for _, p := range fn.Params {
		qt := c.qbeType(p.Type)
		pv := "%" + p.Name
		c.params[p.Name] = pv
		c.varTypes[p.Name] = p.Type
		params = append(params, fmt.Sprintf("%s %s", qt, pv))
	}

	export := ""
	if fn.Public {
		export = "export "
	}

	c.emit("%sfunction w $%s(%s) {", export, fn.Name, strings.Join(params, ", "))
	c.emit("@start")

	for _, stmt := range fn.Body {
		c.genStmt(stmt, "    ")
	}

	hasTopReturn := false
	for _, stmt := range fn.Body {
		if _, ok := stmt.(*ast.ReturnNode); ok {
			hasTopReturn = true
		}
	}
	if !hasTopReturn {
		if fn.Returns != nil {
			if lit, ok := fn.Returns.(*ast.LiteralNode); ok {
				retVal := c.emitLoad(lit.Value, "    ")
				c.emit("    ret %s", retVal)
			} else {
				c.emit("    ret 0")
			}
		} else {
			c.emit("    ret 0")
		}
	}

	c.emit("}")
	c.emit("")
}

func (c *Codegen) genStmt(node ast.Node, indent string) {
	switch n := node.(type) {
	case *ast.VariableNode:
		c.genVariable(n, indent)
	case *ast.IfNode:
		c.genIf(n, indent)
	case *ast.ReturnNode:
		val := c.evalToTemp(n.Value, "w", indent)
		c.emit("%sret %s", indent, val)
	case *ast.MutationNode:
		ptr, ok := c.vars[n.Name]
		if !ok {
			c.emit("%s# Error: %s は未宣言", indent, n.Name)
			return
		}
		qt := c.qbeType(n.Type)
		val := c.evalToTemp(n.Value, qt, indent)
		c.emitStore(qt, val, ptr, indent)
	case *ast.LoopNode:
		c.genLoop(n, indent)
	case *ast.CallNode:
		c.genCall(n, indent)
	case *ast.FatalNode:
		c.emit("%s# Fatal: %s - %s", indent, n.ErrType, n.Msg)
		c.emit("%scall $exit(w 1)", indent)
	case *ast.ErrorNode:
		c.genError(n, indent)
	case *ast.FuncNode:
		c.genFunc(n)

	// ===== share =====
	case *ast.ShareNode:
		// share宣言はtypecheckで検証済み、codegenでは何もしない（注釈のみ）
		c.emit("%s# share(%s) - Async間共有変数", indent, n.Name)

	// ===== Async/Await =====
	case *ast.AsyncNode:
		c.genAsync(n, indent)
	case *ast.AwaitNode:
		c.genAwait(n, indent)

	// ===== GPU =====
	case *ast.GPUNode:
		c.genGPU(n, indent)

	// ===== Mem[risk{}] =====
	case *ast.RawMemNode:
		c.genRawMem(n, indent)

	// ===== ループ制御 =====
	case *ast.BreakNode:
		if c.loopEndLabel != "" {
			c.emit("%sjmp %s", indent, c.loopEndLabel)
		} else {
			c.emit("%s# Error: break outside loop", indent)
		}
	case *ast.ContinueNode:
		if c.loopLabel != "" {
			c.emit("%sjmp %s", indent, c.loopLabel)
		} else {
			c.emit("%s# Error: continue outside loop", indent)
		}

	// ===== cast =====
	case *ast.CastNode:
		c.genCast(n, indent)

	// ===== ポインタ =====
	case *ast.AddressNode:
		c.genAddress(n, indent)
	case *ast.DerefNode:
		c.genDeref(n, indent)

	// ===== 配列アクセス =====
	case *ast.IndexNode:
		c.genIndex(n, indent)
	}
}

// ===== 変数 =====

func (c *Codegen) genVariable(v *ast.VariableNode, indent string) {
	qt := c.qbeType(v.Type)
	size := c.typeSize(v.Type)
	ptrVar := "%" + v.Name + ".ptr"

	if _, exists := c.vars[v.Name]; !exists {
		c.vars[v.Name] = ptrVar
		c.varTypes[v.Name] = v.Type
		c.emit("%s%s =l alloc%d %d", indent, ptrVar, size, size)
	}

	ptr := c.vars[v.Name]
	if v.Value != nil {
		val := c.evalToTemp(v.Value, qt, indent)
		c.emitStore(qt, val, ptr, indent)
	} else {
		c.emitStore(qt, "0", ptr, indent)
	}
}

// ===== ポインタ本実装 =====

// addr{x} → xのスタックアドレスをlongとして返す
func (c *Codegen) genAddress(n *ast.AddressNode, indent string) string {
	if ptr, ok := c.vars[n.Name]; ok {
		// ptrはすでにlongのアドレス
		result := "%" + c.fresh("addr")
		c.emit("%s%s =l copy %s", indent, result, ptr)
		return result
	}
	c.emit("%s# Error: addr: %s は未宣言", indent, n.Name)
	return "0"
}

// addr{x} を式として評価（evalToTemp内から呼ばれる）
func (c *Codegen) evalAddress(n *ast.AddressNode, indent string) string {
	return c.genAddress(n, indent)
}

// deref{ptr} → ptrが指す値をロード
func (c *Codegen) genDeref(n *ast.DerefNode, indent string) string {
	// まず変数のスタックptrアドレスを取得
	ptr, ok := c.vars[n.Name]
	if !ok {
		c.emit("%s# Error: deref: %s は未宣言", indent, n.Name)
		return "0"
	}
	// ptrからアドレス値をload（long）
	addr := "%" + c.fresh("daddr")
	c.emit("%s%s =l loadl %s", indent, addr, ptr)
	// そのアドレスが指す値をload（word）
	result := "%" + c.fresh("deref")
	c.emit("%s%s =w loadw %s", indent, result, addr)
	return result
}

// ===== 配列アクセス本実装 =====
// index{arr(i)} → arr の先頭アドレス + i*elemSize をロード

func (c *Codegen) genIndex(n *ast.IndexNode, indent string) string {
	arrPtr, ok := c.vars[n.Name]
	if !ok {
		c.emit("%s# Error: index: %s は未宣言", indent, n.Name)
		return "0"
	}

	// 配列の先頭アドレスをロード（long）
	base := "%" + c.fresh("base")
	c.emit("%s%s =l loadl %s", indent, base, arrPtr)

	// インデックスを評価
	idxVal := c.evalToTemp(n.Index, "w", indent)

	// elem size: 配列型から要素サイズを決定（デフォルト4）
	elemSize := 4
	if t, ok := c.varTypes[n.Name]; ok {
		switch t {
		case "Array_float":
			elemSize = 4
		case "Array_int":
			elemSize = 4
		case "Array_bool":
			elemSize = 4
		}
	}

	// offset = idx * elemSize
	idxL := "%" + c.fresh("idxl")
	c.emit("%s%s =l extsw %s", indent, idxL, idxVal)
	offset := "%" + c.fresh("off")
	c.emit("%s%s =l mul %s, %d", indent, offset, idxL, elemSize)

	// addr = base + offset
	addr := "%" + c.fresh("iaddr")
	c.emit("%s%s =l add %s, %s", indent, addr, base, offset)

	// load value
	result := "%" + c.fresh("elem")
	c.emit("%s%s =w loadw %s", indent, result, addr)
	return result
}

// ===== cast本実装 =====

func (c *Codegen) genCast(n *ast.CastNode, indent string) string {
	srcVal := c.evalToTemp(n.Value, "w", indent)
	dstQt := c.qbeType(n.Type)
	result := "%" + c.fresh("cast")

	switch n.Type {
	case "float":
		// int → float
		c.emit("%s%s =s swtof %s", indent, result, srcVal)
	case "int":
		// float → int
		c.emit("%s%s =w stosi %s", indent, result, srcVal)
	default:
		c.emit("%s%s =%s copy %s", indent, result, dstQt, srcVal)
	}
	return result
}

// ===== Async/Await本実装 =====
// QBEにはスレッドがないのでpthread呼び出しとして展開する

func (c *Codegen) genAsync(n *ast.AsyncNode, indent string) {
	// Async blockをヘルパー関数として切り出す
	asyncFuncName := fmt.Sprintf("__async_task_%d", c.asyncCounter)
	c.asyncCounter++

	// 呼び出し元: pthread_createでasync関数を起動
	tidVar := "%" + c.fresh("tid")
	c.emit("%s%s =l alloc8 8", indent, tidVar)
	c.emit("%scall $pthread_create(l %s, l 0, l $%s, l 0)", indent, tidVar, asyncFuncName)

	// async関数本体を別関数として出力（後で追記）
	savedBuf := c.buf
	savedVars := c.vars
	savedVarTypes := c.varTypes
	savedParams := c.params

	c.buf = strings.Builder{}
	c.vars = make(map[string]string)
	c.varTypes = make(map[string]string)
	c.params = make(map[string]string)

	c.emit("function l $%s(l %%_arg) {", asyncFuncName)
	c.emit("@start")
	for _, stmt := range n.Body {
		c.genStmt(stmt, "    ")
	}
	c.emit("    ret 0")
	c.emit("}")
	c.emit("")

	asyncBody := c.buf.String()

	c.buf = savedBuf
	c.vars = savedVars
	c.varTypes = savedVarTypes
	c.params = savedParams

	// async関数をバッファの末尾に追加
	c.emit("%s", asyncBody)
}

func (c *Codegen) genAwait(n *ast.AwaitNode, indent string) {
	// pthread_joinでスレッドの完了を待つ
	// tidはAwait[task]のtaskという変数から取得
	tidPtr, ok := c.vars[n.Target]
	if !ok {
		c.emit("%s# Error: Await: %s は未宣言", indent, n.Target)
		return
	}
	tid := "%" + c.fresh("tid")
	c.emit("%s%s =l loadl %s", indent, tid, tidPtr)
	c.emit("%scall $pthread_join(l %s, l 0)", indent, tid)
}

// ===== GPU本実装 =====
// OpenCLのclEnqueueNDRangeKernelを想定した展開

func (c *Codegen) genGPU(n *ast.GPUNode, indent string) {
	gpuFuncName := fmt.Sprintf("__gpu_kernel_%d", c.asyncCounter)
	c.asyncCounter++

	c.emit("%s# GPU kernel: %s", indent, gpuFuncName)
	// GPU kernelをCPU側からOpenCL経由で起動する想定
	// 現時点ではCPUフォールバックとして展開
	c.emit("%s# GPU fallback: CPU実行", indent)
	for _, stmt := range n.Body {
		c.genStmt(stmt, indent)
	}
	_ = gpuFuncName
}

// ===== Mem[risk{}]本実装 =====
// unsafe操作ブロック：境界チェックなし、直接メモリアクセス許可

func (c *Codegen) genRawMem(n *ast.RawMemNode, indent string) {
	c.emit("%s# Mem[risk]: unsafe block begin", indent)
	for _, stmt := range n.Body {
		c.genStmt(stmt, indent)
	}
	c.emit("%s# Mem[risk]: unsafe block end", indent)
}

// ===== ユーティリティ =====

func (c *Codegen) emitLoad(name, indent string) string {
	if pv, ok := c.params[name]; ok {
		return pv
	}
	if ptr, ok := c.vars[name]; ok {
		qt := "w"
		if t, ok := c.varTypes[name]; ok {
			qt = c.qbeType(t)
		}
		tmp := "%" + c.fresh("t")
		c.emitLoadInstr(qt, tmp, ptr, indent)
		return tmp
	}
	return name
}

func (c *Codegen) emitLoadInstr(qt, dst, src, indent string) {
	switch qt {
	case "l":
		c.emit("%s%s =l loadl %s", indent, dst, src)
	case "s":
		c.emit("%s%s =s loads %s", indent, dst, src)
	default:
		c.emit("%s%s =w loadw %s", indent, dst, src)
	}
}

func (c *Codegen) emitStore(qt, val, ptr, indent string) {
	switch qt {
	case "l":
		c.emit("%sstorel %s, %s", indent, val, ptr)
	case "s":
		c.emit("%sstores %s, %s", indent, val, ptr)
	default:
		c.emit("%sstorew %s, %s", indent, val, ptr)
	}
}

func (c *Codegen) evalToTemp(node ast.Node, qt string, indent string) string {
	if node == nil {
		return "0"
	}
	switch n := node.(type) {
	case *ast.LiteralNode:
		if pv, ok := c.params[n.Value]; ok {
			return pv
		}
		if ptr, ok := c.vars[n.Value]; ok {
			vqt := qt
			if t, ok := c.varTypes[n.Value]; ok {
				vqt = c.qbeType(t)
			}
			tmp := "%" + c.fresh("t")
			c.emitLoadInstr(vqt, tmp, ptr, indent)
			return tmp
		}
		return n.Value
	case *ast.ExprNode:
		return c.genExprNode(n, qt, indent)
	case *ast.CallNode:
		return c.genCallExpr(n, qt, indent)
	case *ast.AddressNode:
		return c.evalAddress(n, indent)
	case *ast.DerefNode:
		return c.genDeref(n, indent)
	case *ast.IndexNode:
		return c.genIndex(n, indent)
	case *ast.CastNode:
		return c.genCast(n, indent)
	}
	return "0"
}

func (c *Codegen) genExprNode(expr *ast.ExprNode, qt string, indent string) string {
	if expr.Type != "" {
		qt = c.qbeType(expr.Type)
	}

	result := "%" + c.fresh("r")
	op := c.qbeOp(expr.Op, qt)

	left := c.evalToTemp(expr.Left, qt, indent)
	right := c.evalToTemp(expr.Right, qt, indent)

	c.emit("%s%s =%s %s %s, %s", indent, result, qt, op, left, right)
	return result
}

func (c *Codegen) genIf(n *ast.IfNode, indent string) {
	trueLabel := "@" + c.fresh("true")
	falseLabel := "@" + c.fresh("false")
	endLabel := "@" + c.fresh("end")

	if cond, ok := n.Condition.(*ast.ConditionNode); ok {
		condVar := "%" + c.fresh("cond")
		qbeOp := c.qbeCompare(cond.Op)
		left := c.emitLoad(cond.Left, indent)
		right := c.emitLoad(cond.Right, indent)
		c.emit("%s%s =w %s %s, %s", indent, condVar, qbeOp, left, right)
		c.emit("%sjnz %s, %s, %s", indent, condVar, trueLabel, falseLabel)
	}

	c.emit("%s", trueLabel)
	hasReturn := false
	for _, stmt := range n.True {
		c.genStmt(stmt, indent+"    ")
		if _, ok := stmt.(*ast.ReturnNode); ok {
			hasReturn = true
		}
	}
	if !hasReturn {
		c.emit("%sjmp %s", indent+"    ", endLabel)
	}

	c.emit("%s", falseLabel)
	falseHasReturn := false
	for _, stmt := range n.False {
		c.genStmt(stmt, indent+"    ")
		if _, ok := stmt.(*ast.ReturnNode); ok {
			falseHasReturn = true
		}
	}
	if !falseHasReturn {
		c.emit("%sjmp %s", indent+"    ", endLabel)
	}
	c.emit("%s", endLabel)
}

func (c *Codegen) genLoop(n *ast.LoopNode, indent string) {
	loopLabel := "@" + c.fresh("loop")
	bodyLabel := "@" + c.fresh("body")
	endLabel := "@" + c.fresh("lend")
	prevEnd := c.loopEndLabel
	prevLoop := c.loopLabel
	c.loopEndLabel = endLabel
	c.loopLabel = loopLabel

	if n.Kind == "for" {
		if init, ok := n.Init.(*ast.VariableNode); ok {
			c.genVariable(init, indent)
			c.emit("%sjmp %s", indent, loopLabel)
			c.emit("%s", loopLabel)

			if cond, ok := n.Condition.(*ast.ConditionNode); ok {
				condVar := "%" + c.fresh("cond")
				qbeOp := c.qbeCompare(cond.Op)
				left := c.emitLoad(cond.Left, indent)
				right := c.emitLoad(cond.Right, indent)
				c.emit("%s%s =w %s %s, %s", indent, condVar, qbeOp, left, right)
				c.emit("%sjnz %s, %s, %s", indent, condVar, bodyLabel, endLabel)
			}

			c.emit("%s", bodyLabel)
			for _, stmt := range n.Body {
				c.genStmt(stmt, indent+"    ")
			}

			iCur := c.emitLoad(init.Name, indent+"    ")
			iNew := "%" + c.fresh("step")
			c.emit("%s%s =w add %s, %d", indent+"    ", iNew, iCur, n.Step)
			c.emit("%sstorew %s, %s", indent+"    ", iNew, c.vars[init.Name])
			c.emit("%sjmp %s", indent+"    ", loopLabel)
		}
	} else {
		if init, ok := n.Init.(*ast.VariableNode); ok {
			c.genVariable(init, indent)
			c.emit("%sjmp %s", indent, loopLabel)
			c.emit("%s", loopLabel)

			cnt := c.emitLoad(init.Name, indent)
			condVar := "%" + c.fresh("cond")
			c.emit("%s%s =w csgtw %s, 0", indent, condVar, cnt)
			c.emit("%sjnz %s, %s, %s", indent, condVar, bodyLabel, endLabel)

			c.emit("%s", bodyLabel)
			for _, stmt := range n.Body {
				c.genStmt(stmt, indent+"    ")
			}

			cntCur := c.emitLoad(init.Name, indent+"    ")
			cntNew := "%" + c.fresh("step")
			c.emit("%s%s =w sub %s, 1", indent+"    ", cntNew, cntCur)
			c.emit("%sstorew %s, %s", indent+"    ", cntNew, c.vars[init.Name])
			c.emit("%sjmp %s", indent+"    ", loopLabel)
		}
	}
	c.emit("%s", endLabel)
	c.loopEndLabel = prevEnd
	c.loopLabel = prevLoop
}

func (c *Codegen) genCall(n *ast.CallNode, indent string) {
	var args []string
	for _, arg := range n.Args {
		val := c.evalToTemp(arg, "w", indent)
		args = append(args, "w "+val)
	}
	result := "%" + c.fresh("ret")
	c.emit("%s%s =w call $%s(%s)", indent, result, n.FuncName, strings.Join(args, ", "))
}

func (c *Codegen) genCallExpr(n *ast.CallNode, qt string, indent string) string {
	var args []string
	for _, arg := range n.Args {
		val := c.evalToTemp(arg, "w", indent)
		args = append(args, "w "+val)
	}
	result := "%" + c.fresh("ret")
	c.emit("%s%s =w call $%s(%s)", indent, result, n.FuncName, strings.Join(args, ", "))
	return result
}

func (c *Codegen) genError(n *ast.ErrorNode, indent string) {
	okLabel := "@" + c.fresh("ok")
	errLabel := "@" + c.fresh("err")
	endLabel := "@" + c.fresh("erend")

	errFlag := "%" + c.fresh("eflag")
	c.emit("%s%s =w copy 0", indent, errFlag)
	for _, stmt := range n.Try {
		c.genStmt(stmt, indent)
	}
	c.emit("%sjnz %s, %s, %s", indent, errFlag, errLabel, okLabel)

	c.emit("%s", okLabel)
	for _, stmt := range n.Ok {
		c.genStmt(stmt, indent+"    ")
	}
	c.emit("%sjmp %s", indent+"    ", endLabel)

	c.emit("%s", errLabel)
	if n.Pass {
		c.emit("%sret 1", indent+"    ")
	} else {
		c.emit("%s# Err: %s - %s", indent+"    ", n.ErrType, n.Msg)
	}
	c.emit("%sjmp %s", indent+"    ", endLabel)
	c.emit("%s", endLabel)
}

func (c *Codegen) qbeCompare(op string) string {
	switch op {
	case "equal":
		return "ceqw"
	case "notequal":
		return "cnew"
	case "less":
		return "csltw"
	case "lesseq":
		return "cslew"
	case "greater":
		return "csgtw"
	case "greatereq":
		return "csgew"
	default:
		return "ceqw"
	}
}

func (c *Codegen) qbeOp(op string, qt string) string {
	switch op {
	case "+":
		if qt == "s" {
			return "adds"
		}
		return "add"
	case "-":
		if qt == "s" {
			return "subs"
		}
		return "sub"
	case "*":
		if qt == "s" {
			return "muls"
		}
		return "mul"
	case "/":
		if qt == "s" {
			return "divs"
		}
		return "div"
	default:
		return "add"
	}
}

func (c *Codegen) qbeType(t string) string {
	if qt, ok := typeMap[t]; ok {
		return qt
	}
	return "w"
}

func (c *Codegen) typeSize(t string) int {
	if s, ok := typeSizeMap[t]; ok {
		return s
	}
	return 4
}
