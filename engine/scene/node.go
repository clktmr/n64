package scene

import (
	"image"
	"iter"

	"github.com/clktmr/n64/engine/tree"
)

const scissorFlag int = (1 << 62)

// These are set by the node's implementation
var (
	impl      []Renderer
	z         []uint16
	bounds    []image.Rectangle
	transform []image.Point
)

var scenetree = tree.NewTree(32)

var Zero = Node(tree.Zero)

type Node tree.Node

func (n Node) IsLeaf() bool      { return scenetree.IsLeaf(tree.Node(n)) }
func (n Node) IsRoot() bool      { return scenetree.IsRoot(tree.Node(n)) }
func (n Node) Parent() Node      { return Node(scenetree.Parent(tree.Node(n))) }
func (n Node) Child(i int) Node  { return Node(scenetree.Child(tree.Node(n), i)) }
func (n Node) NextSibling() Node { return Node(scenetree.NextSibling(tree.Node(n))) }
func (n Node) PrevSibling() Node { return Node(scenetree.PrevSibling(tree.Node(n))) }
func (n Node) AddChild(node Renderer) Node {
	child := Node(scenetree.Add(tree.Node(n)))
	if int(child) >= len(z) {
		impl = append(impl, node)
		z = append(z, 0)
		bounds = append(bounds, image.Rectangle{})
		transform = append(transform, image.Point{})
	} else {
		impl[child] = node
		z[child] = 0
		bounds[child] = image.Rectangle{}
		transform[child] = image.Point{}
	}
	return child
}
func (n Node) Remove() {
	for _, n := range scenetree.Postorder(tree.Node(n)) {
		impl[n] = nil
	}
	scenetree.Remove(tree.Node(n))
}
func (n Node) ChildrenCount() int { return scenetree.ChildrenCount(tree.Node(n)) }
func (n Node) Children() iter.Seq[Node] {
	fn := scenetree.Children(tree.Node(n))
	return func(yield func(Node) bool) {
		fn(func(nn tree.Node) bool { return yield(Node(nn)) })
	}
}
func (n Node) Preorder() iter.Seq2[int, Node] {
	fn := scenetree.Preorder(tree.Node(n))
	return func(yield func(int, Node) bool) {
		fn(func(depth int, nn tree.Node) bool { return yield(depth, Node(nn)) })
	}
}
func (n Node) Postorder() iter.Seq2[int, Node] {
	fn := scenetree.Postorder(tree.Node(n))
	return func(yield func(int, Node) bool) {
		fn(func(depth int, nn tree.Node) bool { return yield(depth, Node(nn)) })
	}
}

func (n Node) Bounds() image.Rectangle {
	b := bounds[n]
	b.Min.X &^= scissorFlag
	return b
}
func (n Node) SetBounds(r image.Rectangle)  { bounds[n] = r }
func (n Node) SetScissor(r image.Rectangle) { r.Min.X |= scissorFlag; bounds[n] = r }
func (n Node) IsScissor() bool              { return bounds[n].Min.X&scissorFlag != 0 }
func (n Node) Transform() image.Point       { return transform[n] }
func (n Node) SetTransform(t image.Point)   { transform[n] = t }
func (n Node) Z() uint16                    { return z[n] }
func (n Node) SetZ(depth uint16)            { z[n] = depth }
func (n Node) Value() Renderer              { return impl[n] }
