package tree

import "iter"

// Tree stores a n-ary tree with nodes of type [Node]. The nodes don't hold any
// data themself, but can be used as an index to an array. This allows storing
// the tree as an struct of arrays instead of an array of structs.
//
// The tree will allocate memory to grow if necessary, but it'll never free
// memory if it shrinks.
type Tree struct {
	nodes []node
	free  []Node
}

type node struct {
	firstChild, nextSibling Node
}

// Node stores the handle to a node in a [Tree].
// The zero value always represents the root node.
type Node uint16

const maxNodes = 1 << 15

const (
	// Root always refers to the root node of a [Tree].
	Root Node = 0

	// Zero is the zero value of Node. It is identical to the [Root] value.
	// If it refers to the root node or no node depends on context.
	Zero Node = Root

	lastChild Node = maxNodes
)

func NewTree(capacity int) *Tree {
	capacity = min(capacity, maxNodes)
	return &Tree{
		nodes: make([]node, 0, capacity),
		free:  make([]Node, 0, capacity),
	}
}

// IsLeaf returns true if the node has no children.
func (t *Tree) IsLeaf(n Node) bool {
	return t.nodes[n].firstChild == Zero
}

// IsLeaf returns true if the node's parent is the node itself.
func (t *Tree) IsRoot(n Node) bool {
	return n == Root
}

// Parent returns the node's parent. The root node's parent is the root node
// itself.
func (t *Tree) Parent(n Node) Node {
	for t.nodes[n].nextSibling&lastChild == 0 {
		n = t.nodes[n].nextSibling
	}
	return t.nodes[n].nextSibling &^ lastChild
}

// Depth returns the number of edges between the root node and n.
func (t *Tree) Depth(n Node) (depth int) {
	for !t.IsRoot(n) {
		n = t.Parent(n)
		depth++
	}
	return
}

// Child returns the ith's child or the zero value if n has no children. If i is
// less than zero the first child is returned, if it's the children count or
// more the last child is returned.
func (t *Tree) Child(n Node, i int) (c Node) {
	for child := range t.Children(n) {
		if i <= 0 {
			c = child
			break
		}
		i--
	}
	return
}

// NextSibling returns the next sibling or the zero value if n is the last
// child.
func (t *Tree) NextSibling(n Node) Node {
	if t.nodes[n].nextSibling&lastChild != 0 {
		return Zero
	}
	return t.nodes[n].nextSibling
}

// PrevSibling returns the previous sibling or the zero value if n is the first
// child.
func (t *Tree) PrevSibling(n Node) Node {
	if n == Root {
		return Zero
	}
	parent := t.Parent(n)
	if t.nodes[parent].firstChild == n {
		return Zero
	}
	var child Node
	for child = range t.Children(parent) {
		if t.NextSibling(child) == n {
			break
		}
	}
	return child
}

// Add inserts a new node n initialized with value as the first child of parent.
func (t *Tree) Add(parent Node) (n Node) {
	if len(t.free) > 0 {
		n = t.free[len(t.free)-1]
		t.free = t.free[:len(t.free)-1]
		t.nodes[n].firstChild = Zero
	} else if len(t.nodes) < maxNodes {
		t.nodes = append(t.nodes, node{})
		n = Node(len(t.nodes) - 1)
	} else {
		panic("max nodes reached")
	}

	if parent == n { // tree was empty, n is root
		t.nodes[n].nextSibling = Root | lastChild
		return
	}

	oldFirst := t.nodes[parent].firstChild
	t.nodes[parent].firstChild = n
	t.nodes[n].nextSibling = oldFirst
	if oldFirst == Zero {
		t.nodes[n].nextSibling = parent | lastChild
	}

	return
}

// Remove removes node n and all of it's children from the tree. The [Node]
// values become invalid after the call returns.
func (t *Tree) Remove(n Node) {
	for _, n := range t.Postorder(n) {
		t.free = append(t.free, n)
	}
	if n == Root {
		return
	}
	parent := t.Parent(n)
	if t.nodes[parent].firstChild == n {
		t.nodes[parent].firstChild = t.NextSibling(n)
	} else {
		prev := t.PrevSibling(n)
		t.nodes[prev].nextSibling = t.nodes[n].nextSibling
	}
}

// ChildrenCount returns how many children n has.
func (t *Tree) ChildrenCount(n Node) (count int) {
	for range t.Children(n) {
		count++
	}
	return
}

// Children returns an iterator over all children of n.
func (t *Tree) Children(n Node) iter.Seq[Node] {
	return func(yield func(Node) bool) {
		it := t.nodes[n].firstChild
		if it == Zero {
			return
		}
		for yield(it) {
			if t.nodes[it].nextSibling&lastChild != 0 {
				break
			}
			it = t.nodes[it].nextSibling
		}
	}
}

// Preorder traverses the subtree at n in pre-order.
func (t *Tree) Preorder(n Node) iter.Seq2[int, Node] {
	return func(yield func(int, Node) bool) {
		it := n
		depth := 0
		for yield(depth, it) {
			child := t.nodes[it].firstChild
			if child != Zero {
				it = child
				depth++
				continue
			}
			for {
				if it == n {
					return
				}
				next := t.nodes[it].nextSibling
				if next&lastChild == 0 {
					it = next
					break
				}
				it = next &^ lastChild
				depth--
			}
		}
	}
}

// Walk traverses the subtree at n in post-order.
func (t *Tree) Postorder(n Node) iter.Seq2[int, Node] {
	return func(yield func(int, Node) bool) {
		it := n
		depth := 0
		for t.nodes[it].firstChild != Zero {
			it = t.nodes[it].firstChild
			depth++
		}
		for {
			if !yield(depth, it) {
				return
			}
			if it == n {
				return
			}
			next := t.nodes[it].nextSibling
			if next&lastChild == 0 {
				it = next
				for t.nodes[it].firstChild != Zero {
					it = t.nodes[it].firstChild
					depth++
				}
			} else {
				it = next &^ lastChild
				depth--
			}
		}
	}
}
