// Package caigen: Similarity AST → CAI テキスト形式 生成
package caigen

import (
	"fmt"
	"similarity/ast"
	"similarity/stdlib"
	"strconv"
	"strings"
)

// ===== 定数（enum化） =====
const (
	KindInt    = "INT"
	KindFloat  = "FLOAT"
	KindBool   = "BOOL"
	KindString = "STRING_LIT"
	KindIdent  = "IDENT"
)

const (
	OpLess      = "less"
	OpLessEq    = "lesseq"
	OpEqual     = "equal"
	OpNotEqual  = "notequal"
	OpGreater   = "greater"
	OpGreaterEq = "greatereq"
)

const (
	OpAdd = "+"
	OpSub = "-"
	OpMul = "*"
	OpDiv = "/"
)

const (
	CAIAlloc   = "alloc"
	CAIStore   = "store"
	CAILoad    = "load"
	CAIStoreP  = "storep"
	CAILoadP2  = "loadp2"
	CAIAddra   = "addra"  // スタック配列の先頭アドレス取得(lea)
	CAIAddP    = "addp"
	CAILoadB   = "loadb"
	CAIStoreB  = "storeb"
	CAIAdd     = "add"
	CAISub     = "sub"
	CAIMul     = "mul"
	CAIDiv     = "div"
	CAIAdd64   = "add64"
	CAISub64   = "sub64"
	CAIMul64   = "mul64"
	CAIDiv64   = "div64"
	CAIAddF    = "addf"
	CAISubF    = "subf"
	CAIMulF    = "mulf"
	CAIDivF    = "divf"
	CAIItof2   = "itof2"
	CAIFtoi2   = "ftoi2"
	CAIClt     = "clt"
	CAICle     = "cle"
	CAICeq     = "ceq"
	CAICne     = "cne"
	CAICgt     = "cgt"
	CAICge     = "cge"
	CAIClt64   = "clt64"
	CAICle64   = "cle64"
	CAICeq64   = "ceq64"
	CAICne64   = "cne64"
	CAICgt64   = "cgt64"
	CAICge64   = "cge64"
	CAILabel   = "label"
	CAIJmp     = "jmp"
	CAIJnz     = "jnz"
	CAICall    = "call"
	CAIRet     = "ret"
	CAIMov     = "mov"
	CAISyscall = "syscall"
	CAIData    = "data"
)

// ===== 型判定ヘルパー =====

// isPtr64: ポインタ・64bit・文字列型かどうか
func isPtr64(typeName string) bool {
	switch typeName {
	case "String", "ptr", "int64":
		return true
	}
	return false
}

// isFloat: float型かどうか
func isFloat(typeName string) bool {
	return typeName == "float"
}

// ===== 型サイズ管理 =====

var typeSizes = map[string]int{
	"int":    4,
	"float":  4,
	"bool":   4,
	"String": 8,
	"ptr":    8,
	"int64":  8,
}

func sizeOf(typeName string) int {
	if s, ok := typeSizes[typeName]; ok {
		return s
	}
	// Array_int(N) 形式 — ここではelemサイズのみ返す（alloc時はN*elemSize）
	if strings.HasPrefix(typeName, "Array_") {
		elem := strings.TrimPrefix(typeName, "Array_")
		if s, ok := typeSizes[elem]; ok {
			return s
		}
	}
	return 4
}

// ===== StructInfo =====

type StructInfo struct {
	Fields    []ast.StructField
	Size      int
	Alignment int
}

func newStructInfo(fields []ast.StructField) StructInfo {
	size, align := 0, 1
	for _, f := range fields {
		fs := sizeOf(f.Type)
		size += fs
		if fs > align {
			align = fs
		}
	}
	return StructInfo{Fields: fields, Size: size, Alignment: align}
}

// ===== CAIGen =====

type CAIGen struct {
	out           strings.Builder
	tmpIdx        int
	labelIdx      int
	structs       map[string]StructInfo
	varTypes      map[string]string
	breakLabel    string // 現在のループのbreak先
	continueLabel string // 現在のループのcontinue先
}

func New() *CAIGen {
	return &CAIGen{
		structs:  make(map[string]StructInfo),
		varTypes: make(map[string]string),
	}
}

func (c *CAIGen) emit(format string, args ...interface{}) {
	fmt.Fprintf(&c.out, format+"\n", args...)
}

func (c *CAIGen) tmp() string {
	c.tmpIdx++
	return fmt.Sprintf("%%t%d", c.tmpIdx)
}

func (c *CAIGen) label() string {
	c.labelIdx++
	return fmt.Sprintf("lbl%d", c.labelIdx)
}

// ===== store/load命令の型別選択 =====

// emitStore: 型に応じてstore/storepを選択
func (c *CAIGen) emitStore(indent, varName, val, typeName string) {
	if isPtr64(typeName) {
		c.emit("%s%s  %%%s.ptr %s", indent, CAIStoreP, varName, val)
	} else {
		c.emit("%s%s  %%%s.ptr %s", indent, CAIStore, varName, val)
	}
}

// emitLoad: 型に応じてload/loadp2を選択
func (c *CAIGen) emitLoad(indent, dst, varName, typeName string) {
	if isPtr64(typeName) {
		c.emit("%s%s  %s %%%s.ptr", indent, CAILoadP2, dst, varName)
	} else {
		c.emit("%s%s   %s %%%s.ptr", indent, CAILoad, dst, varName)
	}
}

// ===== Generate =====

func (c *CAIGen) Generate(prog *ast.Program) string {
	c.emitHeader(prog)
	c.emitImports(prog)
	c.collectStructs(prog)
	c.emitFunctions(prog)
	return c.out.String()
}

func (c *CAIGen) emitHeader(prog *ast.Program) {
	if prog.Explanation != nil {
		c.emit("# Similarity - %s", prog.Explanation.Category)
		for k, v := range prog.Explanation.Args {
			c.emit("# %s: %s", k, v)
		}
	}
	c.emit("")
}

func (c *CAIGen) emitImports(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *ast.ImportNode:
			if lib, ok := stdlib.AvailableLibsCAI[n.Module]; ok {
				c.emit("# stdlib: %s", n.Module)
				c.out.WriteString(lib)
				c.emit("")
			}
		case *ast.ExternNode:
			for _, fn := range n.Funcs {
				c.emit("extern $%s", fn.Name)
			}
		}
	}
}

func (c *CAIGen) collectStructs(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		v, ok := stmt.(*ast.VariableNode)
		if !ok || v.Type != "__struct__" {
			continue
		}
		def, ok := v.Value.(*ast.StructDefNode)
		if !ok {
			continue
		}
		c.structs[def.Name] = newStructInfo(def.Fields)
	}
}

func (c *CAIGen) emitFunctions(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*ast.FuncNode); ok {
			c.genFunc(fn)
		}
	}
}

// ===== 関数生成 =====

func (c *CAIGen) genFunc(fn *ast.FuncNode) {
	// 関数スコープのvarTypesをリセット
	savedTypes := c.varTypes
	c.varTypes = make(map[string]string)
	// 親スコープの型情報を引き継ぐ
	for k, v := range savedTypes {
		c.varTypes[k] = v
	}

	if fn.Public {
		c.emit("export func $%s", fn.Name)
	} else {
		c.emit("func $%s", fn.Name)
	}

	// 引数をスタックに確保
	for i, param := range fn.Params {
		size := sizeOf(param.Type)
		c.varTypes[param.Name] = param.Type
		c.emit("  %s  %%%s.ptr %d", CAIAlloc, param.Name, size)
		if isPtr64(param.Type) {
			c.emit("  %s  %%%s.ptr %%arg%d", CAIStoreP, param.Name, i)
		} else {
			c.emit("  %s  %%%s.ptr %%arg%d", CAIStore, param.Name, i)
		}
	}

	for _, stmt := range fn.Body {
		c.genStmt(stmt, "  ")
	}

	if fn.Returns != nil {
		val := c.genExpr(fn.Returns, "  ")
		c.emit("  %s    %s", CAIRet, val)
	}

	c.emit("endfunc")
	c.emit("")

	c.varTypes = savedTypes
}

// ===== 文生成 =====

func (c *CAIGen) genStmt(node ast.Node, indent string) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.VariableNode:
		if n.Type == "__struct__" {
			return
		}
		// 配列宣言: Variable[let{Array_int(arr:N)}]
		if strings.HasPrefix(n.Type, "Array_") {
			elemType := strings.TrimPrefix(n.Type, "Array_")
			elemSize := sizeOf(elemType)
			size := 0
			if lit, ok := n.Value.(*ast.LiteralNode); ok {
				size, _ = strconv.Atoi(lit.Value)
			}
			totalSize := size * elemSize
			if totalSize == 0 {
				totalSize = elemSize // フォールバック
			}
			c.varTypes[n.Name] = n.Type
			c.emit("%s%s  %%%s.ptr %d", indent, CAIAlloc, n.Name, totalSize)
			return
		}
		// 文字列リテラルはdataラベルとして.rodataに配置
		if lit, ok := n.Value.(*ast.LiteralNode); ok && lit.Kind == KindString {
			label := fmt.Sprintf("$str_%s_%d", n.Name, c.tmpIdx)
			c.tmpIdx++
			c.emit("%s%s %s \"%s\"", indent, CAIData, label, lit.Value)
			size := sizeOf(n.Type)
			c.varTypes[n.Name] = n.Type
			c.emit("%s%s  %%%s.ptr %d", indent, CAIAlloc, n.Name, size)
			c.emit("%s%s  %%%s.ptr %s", indent, CAIStoreP, n.Name, label)
			return
		}
		size := sizeOf(n.Type)
		c.varTypes[n.Name] = n.Type
		c.emit("%s%s  %%%s.ptr %d", indent, CAIAlloc, n.Name, size)
		if n.Value != nil {
			val := c.genExpr(n.Value, indent)
			c.emitStore(indent, n.Name, val, n.Type)
		}

	case *ast.ArrayStoreNode:
		// Mutation[array{int(arr:i:val)}]
		// addr = base + idx * elemSize
		idx := c.genExprLoad(n.Index, indent)
		val := c.genExprLoad(n.Value, indent)
		base := c.tmp()
		c.emit("%s%s  %s %%%s.ptr", indent, CAIAddra, base, n.Name)
		elemSize := sizeOf(n.ElemType)
		offset := c.tmp()
		c.emit("%s%s    %s %s %d", indent, CAIMul, offset, idx, elemSize)
		addr := c.tmp()
		c.emit("%s%s   %s %s %s", indent, CAIAddP, addr, base, offset)
		c.emit("%sstorei  %s %s", indent, addr, val)

	case *ast.MutationNode:
		if n.Value != nil {
			val := c.genExpr(n.Value, indent)
			typeName := c.varTypes[n.Name]
			c.emitStore(indent, n.Name, val, typeName)
		}

	case *ast.ReturnNode:
		if n.Value != nil {
			val := c.genExpr(n.Value, indent)
			c.emit("%s%s    %s", indent, CAIRet, val)
		} else {
			c.emit("%s%s", indent, CAIRet)
		}

	case *ast.IfNode:
		c.genIf(n, indent)

	case *ast.LoopNode:
		c.genLoop(n, indent)

	case *ast.CallNode:
		t := c.tmp()
		args := c.genArgs(n.Args, indent)
		c.emit("%s%s   %s $%s%s", indent, CAICall, t, n.FuncName, args)

	case *ast.RawMemNode:
		c.emit("%s# risk block begin", indent)
		for _, s := range n.Body {
			c.genStmt(s, indent)
		}
		c.emit("%s# risk block end", indent)

	case *ast.FatalNode:
		c.emit("%s# Fatal: %s - %s", indent, n.ErrType, n.Msg)
		c.emit("%s%s   %%_ $panic", indent, CAICall)

	case *ast.AsyncNode:
		c.emit("%s# Async block (pthread)", indent)
		for _, s := range n.Body {
			c.genStmt(s, indent)
		}

	case *ast.IncrNode:
		// ++{i} → load i, add 1, store i
		// --{i} → load i, sub 1, store i
		typeName := c.varTypes[n.Name]
		dst := c.tmp()
		c.emitLoad(indent, dst, n.Name, typeName)
		result := c.tmp()
		if n.Op == "++" {
			c.emit("%s%s    %s %s 1", indent, CAIAdd, result, dst)
		} else {
			c.emit("%s%s    %s %s 1", indent, CAISub, result, dst)
		}
		c.emitStore(indent, n.Name, result, typeName)

	case *ast.BreakNode:
		if c.breakLabel != "" {
			c.emit("%s%s    %s", indent, CAIJmp, c.breakLabel)
		}

	case *ast.ContinueNode:
		if c.continueLabel != "" {
			c.emit("%s%s    %s", indent, CAIJmp, c.continueLabel)
		}
	}
}

// ===== If生成 =====

func (c *CAIGen) genIf(n *ast.IfNode, indent string) {
	cond := c.genCond(n.Condition, indent)
	lTrue := c.label()
	lFalse := c.label()
	lEnd := c.label()

	c.emit("%s%s    %s %s %s", indent, CAIJnz, cond, lTrue, lFalse)

	c.emit("%s%s  %s", indent, CAILabel, lTrue)
	for _, s := range n.True {
		c.genStmt(s, indent+"  ")
	}
	c.emit("%s%s    %s", indent, CAIJmp, lEnd)

	c.emit("%s%s  %s", indent, CAILabel, lFalse)
	for _, s := range n.False {
		c.genStmt(s, indent+"  ")
	}

	c.emit("%s%s  %s", indent, CAILabel, lEnd)
}

// ===== Loop生成 =====

func (c *CAIGen) genLoop(n *ast.LoopNode, indent string) {
	lStart := c.label()
	lBody := c.label()
	lEnd := c.label()

	// break/continueラベルをスタック的に保存
	savedBreak := c.breakLabel
	savedContinue := c.continueLabel
	c.breakLabel = lEnd
	c.continueLabel = lStart

	if n.Init != nil {
		c.genStmt(n.Init, indent)
	}

	c.emit("%s%s  %s", indent, CAILabel, lStart)

	if n.Condition != nil {
		cond := c.genCond(n.Condition, indent)
		c.emit("%s%s    %s %s %s", indent, CAIJnz, cond, lBody, lEnd)
	}

	c.emit("%s%s  %s", indent, CAILabel, lBody)
	for _, s := range n.Body {
		c.genStmt(s, indent+"  ")
	}

	if n.Step != 0 {
		if n.Init != nil {
			if v, ok := n.Init.(*ast.VariableNode); ok {
				dst := c.tmp()
				t := c.tmp()
				typeName := c.varTypes[v.Name]
				step := n.Step
				if isPtr64(typeName) {
					c.emit("%s%s  %s %%%s.ptr", indent, CAILoadP2, dst, v.Name)
					if step >= 0 {
						c.emit("%s%s  %s %s %d", indent, CAIAdd64, t, dst, step)
					} else {
						c.emit("%s%s  %s %s %d", indent, CAISub64, t, dst, -step)
					}
					c.emit("%s%s  %%%s.ptr %s", indent, CAIStoreP, v.Name, t)
				} else {
					c.emit("%s%s   %s %%%s.ptr", indent, CAILoad, dst, v.Name)
					if step >= 0 {
						c.emit("%s%s    %s %s %d", indent, CAIAdd, t, dst, step)
					} else {
						c.emit("%s%s    %s %s %d", indent, CAISub, t, dst, -step)
					}
					c.emit("%s%s  %%%s.ptr %s", indent, CAIStore, v.Name, t)
				}
			}
		}
	}

	c.emit("%s%s    %s", indent, CAIJmp, lStart)
	c.emit("%s%s  %s", indent, CAILabel, lEnd)

	// break/continueラベルを復元
	c.breakLabel = savedBreak
	c.continueLabel = savedContinue
}

// ===== 条件生成 =====

func (c *CAIGen) genCond(node ast.Node, indent string) string {
	cond, ok := node.(*ast.ConditionNode)
	if !ok {
		return c.genExpr(node, indent)
	}

	// 変数の型を見てi32/i64比較を選択
	typeName := c.varTypes[cond.Left]
	use64 := isPtr64(typeName)

	left := c.loadStrVal(cond.Left, indent)
	right := c.loadStrVal(cond.Right, indent)
	dst := c.tmp()

	var op string
	if use64 {
		switch cond.Op {
		case OpLess:     op = CAIClt64
		case OpLessEq:   op = CAICle64
		case OpEqual:    op = CAICeq64
		case OpNotEqual: op = CAICne64
		case OpGreater:  op = CAICgt64
		case OpGreaterEq: op = CAICge64
		default:         op = CAICeq64
		}
	} else {
		switch cond.Op {
		case OpLess:     op = CAIClt
		case OpLessEq:   op = CAICle
		case OpEqual:    op = CAICeq
		case OpNotEqual: op = CAICne
		case OpGreater:  op = CAICgt
		case OpGreaterEq: op = CAICge
		default:         op = CAICeq
		}
	}
	c.emit("%s%s    %s %s %s", indent, op, dst, left, right)
	return dst
}

func (c *CAIGen) loadStrVal(s string, indent string) string {
	if len(s) == 0 {
		return "0"
	}
	// 数値リテラル（先頭が数字 or マイナス、かつ変数として登録されていない）
	isNum := s[0] >= '0' && s[0] <= '9'
	isNeg := s[0] == '-' && len(s) > 1
	if (isNum || isNeg) && c.varTypes[s] == "" {
		return s
	}
	typeName := c.varTypes[s]
	dst := c.tmp()
	c.emitLoad(indent, dst, s, typeName)
	return dst
}

// ===== 式生成 =====

func (c *CAIGen) genExpr(node ast.Node, indent string) string {
	if node == nil {
		return "0"
	}
	switch n := node.(type) {
	case *ast.LiteralNode:
		if n.Kind == KindIdent {
			typeName := c.varTypes[n.Value]
			dst := c.tmp()
			c.emitLoad(indent, dst, n.Value, typeName)
			return dst
		}
		if n.Kind == KindString {
			// 文字列リテラルをインラインで使う場合（dataラベル生成）
			label := fmt.Sprintf("$strlit_%d", c.tmpIdx)
			c.tmpIdx++
			c.emit("%s%s %s \"%s\"", indent, CAIData, label, n.Value)
			return label
		}
		return n.Value

	case *ast.ExprNode:
		left := c.genExprLoad(n.Left, indent)
		right := c.genExprLoad(n.Right, indent)
		dst := c.tmp()
		// 型に応じてi32/i64/f32演算を選択
		typeName := n.Type
		if typeName == "" {
			// 左辺から型を推論
			if lit, ok := n.Left.(*ast.LiteralNode); ok && lit.Kind == KindIdent {
				typeName = c.varTypes[lit.Value]
			}
		}
		var op string
		if isFloat(typeName) {
			switch n.Op {
			case OpAdd: op = CAIAddF
			case OpSub: op = CAISubF
			case OpMul: op = CAIMulF
			case OpDiv: op = CAIDivF
			default:    op = CAIAddF
			}
		} else if isPtr64(typeName) {
			switch n.Op {
			case OpAdd: op = CAIAdd64
			case OpSub: op = CAISub64
			case OpMul: op = CAIMul64
			case OpDiv: op = CAIDiv64
			default:    op = CAIAdd64
			}
		} else {
			switch n.Op {
			case OpAdd: op = CAIAdd
			case OpSub: op = CAISub
			case OpMul: op = CAIMul
			case OpDiv: op = CAIDiv
			default:    op = CAIAdd
			}
		}
		c.emit("%s%s    %s %s %s", indent, op, dst, left, right)
		return dst

	case *ast.CallNode:
		dst := c.tmp()
		args := c.genArgs(n.Args, indent)
		c.emit("%s%s   %s $%s%s", indent, CAICall, dst, n.FuncName, args)
		return dst

	case *ast.CastNode:
		src := c.genExprLoad(n.Value, indent)
		dst := c.tmp()
		if n.Type == "float" {
			// i32 → f32
			c.emit("%s%s  %s %s", indent, CAIItof2, dst, src)
		} else {
			// f32 → i32
			c.emit("%s%s  %s %s", indent, CAIFtoi2, dst, src)
		}
		return dst

	case *ast.AddressNode:
		// addr{x} → %x.ptrのアドレスをポインタとして返す
		dst := c.tmp()
		c.emit("%s%s    %s %%%s.ptr", indent, CAIMov, dst, n.Name)
		return dst

	case *ast.DerefNode:
		// deref{ptr} → ptrが指す先の値を読む
		// ptrはポインタ変数なので loadp2 で64bitアドレスを読み、
		// さらに load で値を読む
		ptrVal := c.tmp()
		c.emit("%s%s  %s %%%s.ptr", indent, CAILoadP2, ptrVal, n.Name)
		dst := c.tmp()
		c.emit("%s%s   %s %s", indent, CAILoad, dst, ptrVal)
		return dst

	case *ast.IndexNode:
		idx := c.genExprLoad(n.Index, indent)
		dst := c.tmp()
		// 配列アクセス: base + idx*elemSize のアドレスから loadi
		base := c.tmp()
		c.emit("%s%s  %s %%%s.ptr", indent, CAIAddra, base, n.Name)
		// varTypesからelemSizeを取得（Array_int→4など）
		arrType := c.varTypes[n.Name]
		elemType := strings.TrimPrefix(arrType, "Array_")
		elemSize := sizeOf(elemType)
		if elemSize == 0 {
			elemSize = 4
		}
		offset := c.tmp()
		c.emit("%s%s    %s %s %d", indent, CAIMul, offset, idx, elemSize)
		addr := c.tmp()
		c.emit("%s%s   %s %s %s", indent, CAIAddP, addr, base, offset)
		c.emit("%sloadi   %s %s", indent, dst, addr)
		return dst
	}
	return "0"
}

func (c *CAIGen) genExprLoad(node ast.Node, indent string) string {
	if lit, ok := node.(*ast.LiteralNode); ok {
		if lit.Kind == KindInt || lit.Kind == KindFloat {
			return lit.Value
		}
	}
	return c.genExpr(node, indent)
}

func (c *CAIGen) genArgs(args []ast.Node, indent string) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for _, arg := range args {
		v := c.genExprLoad(arg, indent)
		parts = append(parts, v)
	}
	return " " + strings.Join(parts, " ")
}
