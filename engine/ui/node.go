package ui

import (
	"image"
	"iter"

	"github.com/clktmr/n64/engine/scene"
)

// Node is a UI node in the scenetree. All of it's children must be UI nodes as
// well.
//
// Node implements the [Layouter] interface.
type Node struct{ scene.Node }

var Zero = Node{scene.Zero}

func (n Node) Parent() Node {
	if _, ok := n.Node.Parent().Value().(Layouter); !ok {
		return Zero
	}
	return Node{n.Node.Parent()}
}
func (n Node) NextSibling() Node { return Node{n.Node.NextSibling()} }
func (n Node) PrevSibling() Node { return Node{n.Node.PrevSibling()} }
func (n Node) Child(i int) Node  { return Node{n.Node.Child(i)} }
func (n Node) AddChild(node interface {
	scene.Renderer
	Layouter
	Focuser
}) Node {
	return Node{n.Node.AddChild(node)}
}
func (n Node) Children() iter.Seq[Node] {
	fn := n.Node.Children()
	return func(yield func(Node) bool) {
		fn(func(nn scene.Node) bool {
			return yield(Node{nn})
		})
	}
}

func (n Node) Measure(width int) (dx, dy int)           { return n.Value().(Layouter).Measure(width) }
func (n Node) Layout(r image.Rectangle, sp image.Point) { n.Value().(Layouter).Layout(r, sp) }
func (n Node) Focus(d FocusDirection) bool              { return n.Value().(Focuser).Focus(d) }
func (n Node) Focused() int                             { return n.Value().(Focuser).Focused() }
