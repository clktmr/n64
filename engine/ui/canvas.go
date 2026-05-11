package ui

import (
	"image"
	"image/color"
	idraw "image/draw"
	"time"

	"github.com/clktmr/n64/drivers/controller"
	"github.com/clktmr/n64/drivers/draw"
	"github.com/clktmr/n64/engine/scene"
	"github.com/clktmr/n64/rcp/serial/joybus"
)

// Canvas is a container for a single child [Node]. It marks the root node of an
// UI subtree.
type Canvas struct {
	Node

	focusBg image.Image
}

func NewCanvas(parent scene.Node) *Canvas {
	node := &Canvas{}
	node.Node = Node{parent.AddChild(node)}
	node.focusBg = &image.Uniform{color.NRGBA{0xff, 0xff, 0x00, 0x7f}}
	node.SetZ(1000) // TODO arbitrary value
	return node
}

func (c *Canvas) Autosize(width int, clip bool) (dx, dy int) {
	dx, dy = c.Measure(width)
	b := c.Bounds()
	if clip {
		dx = width
	}
	c.Resize(image.Rectangle{b.Min, b.Min.Add(image.Point{dx, dy})})
	return
}

func (c *Canvas) Resize(r image.Rectangle) {
	c.SetScissor(r)
	c.Layout(c.Bounds(), image.Point{})
}

func (c *Canvas) Measure(width int) (dx, dy int) {
	for child := range c.Children() {
		cdx, cdy := child.Measure(width)
		dx, dy = max(dx, cdx), max(dy, cdy)
	}
	return
}

func (c *Canvas) Layout(r image.Rectangle, sp image.Point) {
	for child := range c.Children() {
		child.Layout(r, sp)
	}
}

func (c *Canvas) Render(dst idraw.Image, r image.Rectangle, sp image.Point) {
	xform := r.Min.Sub(sp)
	draw.Over.Draw(dst, c.focused().Bounds().Add(xform), c.focusBg, sp)
}

func (c *Canvas) Update(_ time.Duration, input *[4]controller.Controller) {
	buttons := input[0].Pressed()

	if buttons&joybus.ButtonCLeft != 0 {
		c.Focus(Left)
	}
	if buttons&joybus.ButtonCRight != 0 {
		c.Focus(Right)
	}
	if buttons&joybus.ButtonCUp != 0 {
		c.Focus(Up)
	}
	if buttons&joybus.ButtonCDown != 0 {
		c.Focus(Down)
	}
}

func (c *Canvas) Focus(d FocusDirection) bool {
	it := c.focused()
	for it != c.Node {
		if it.Focus(d) {
			return true
		}
		it = it.Parent()
	}
	return false
}

func (c *Canvas) Focused() int { return 0 }

// focused returns the currently focused [Node] in this UI subtree.
func (c *Canvas) focused() Node {
	it := c.Node
	for {
		next := it.Child(it.Focused())
		if next == Zero {
			return it
		}
		it = next
	}
}
