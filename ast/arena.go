package ast

// Arena: ASTノードを一括管理するメモリプール
type Arena struct {
	nodes []Node
}

func NewArena() *Arena {
	// 最初に1000ノード分確保
	return &Arena{nodes: make([]Node, 0, 1000)}
}

func (a *Arena) Add(n Node) Node {
	a.nodes = append(a.nodes, n)
	return n
}
