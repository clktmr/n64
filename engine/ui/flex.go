package ui

import (
	"image"
	idraw "image/draw"
	"iter"
	"slices"

	"github.com/clktmr/n64/drivers/draw"
)

type (
	JustifyContent int
	AlignItems     int
	Direction      int
)

const (
	DirectionColumn Direction = iota
	DirectionRow
)

const (
	JustifyStart JustifyContent = iota
	JustifyCenter
	JustifyEnd
	JustifyBetween
	JustifyAround
)

const (
	AlignStart AlignItems = iota
	AlignCenter
	AlignEnd
	AlignStretch
)

type Clearance struct{ Top, Bottom, Left, Right int }

func (c Clearance) Dx() int { return c.Left + c.Right }
func (c Clearance) Dy() int { return c.Top + c.Bottom }
func (c Clearance) Add(d Clearance) Clearance {
	return Clearance{
		Top:    c.Top + d.Top,
		Bottom: c.Bottom + d.Bottom,
		Left:   c.Left + d.Left,
		Right:  c.Right + d.Right,
	}
}

func inset(r image.Rectangle, c Clearance) image.Rectangle {
	r.Min.X += c.Left
	r.Max.X -= c.Right
	r.Min.Y += c.Top
	r.Max.Y -= c.Bottom

	if r.Min.X > r.Max.X {
		r.Min.X = (r.Min.X + r.Max.X) / 2
		r.Max.X = r.Min.X
	}
	if r.Min.Y > r.Max.Y {
		r.Min.Y = (r.Min.Y + r.Max.Y) / 2
		r.Max.Y = r.Min.Y
	}

	return r
}

// FlexBox lays out it's children vertically or horizontally.
//
// Children are justified and aligned according to the Justify and Align struct
// fields. If the content box is too small to fit all children, children will be
// clipped. The content box is defined by insetting the bounds by padding and
// margin. A background can be drawn with the size of the content box plus
// margin. The background image can also be used to draw a border.
//
// This UI primitive is designed to be simple yet powerful. There are no
// configuration values where options get interpreted differently. The layout
// function always does the same. Still the available options should be enough
// to cover most usecases.
type FlexBox struct {
	Node

	Padding   Clearance
	Margin    Clearance
	Direction Direction
	Justify   JustifyContent
	Align     AlignItems

	focused int

	Background image.Image
}

func NewFlexBox(parent Node) *FlexBox {
	node := &FlexBox{}
	node.Node = parent.AddChild(node)
	return node
}

func (p *FlexBox) Render(dst idraw.Image, r image.Rectangle, sp image.Point) {
	if p.Background != nil {
		xform := r.Min.Sub(sp).Sub(p.Bounds().Min)
		bgRect := inset(p.Bounds(), p.Margin).Add(xform)
		drawRect := bgRect.Intersect(r)
		if !drawRect.Empty() {
			draw.Over.Draw(dst, drawRect, p.Background, drawRect.Min.Sub(bgRect.Min))
		}
	}
}

func (p *FlexBox) Focus(d FocusDirection) bool {
	if d.Horizontal() && p.Direction == DirectionColumn {
		return false
	}
	if d.Vertical() && p.Direction == DirectionRow {
		return false
	}

	switch d {
	case Down, Right:
		p.focused++
		if p.focused >= p.ChildrenCount() {
			p.focused = p.ChildrenCount() - 1
			return false
		}
	case Up, Left:
		p.focused--
		if p.focused < 0 {
			p.focused = 0
			return false
		}
	}

	return true
}
func (p *FlexBox) Focused() int { return p.focused }

func (p *FlexBox) Measure(width int) (dx, dy int) {
	clearance := p.Padding.Dx() + p.Margin.Dx()
	width = max(width-clearance, 0)

	if p.Direction == DirectionRow {
		dx, dy = p.measureRow(width)
	} else {
		dx, dy = p.measureCol(width)
	}

	dx += clearance
	dy += p.Padding.Dy() + p.Margin.Dy()
	return
}

func (p *FlexBox) measureRow(width int) (dx, dy int) {
	dxs, dx, dy := fairShare(width, p.ChildrenCount(), p.Children())
	if dx != -1 {
		return
	}

	dx, dy = 0, 0
	i := 0
	for child := range p.Children() {
		cdx, cdy := child.Measure(dxs[i])
		dx += cdx
		dy = max(dy, cdy)
		i++
	}
	return
}

func (p *FlexBox) measureCol(width int) (dx, dy int) {
retry:
	for child := range p.Children() {
		cdx, cdy := child.Measure(width)
		dy += cdy
		dx = max(dx, cdx)
	}
	// If at least one child didn't fit into width, remeasure with all
	// childs with the required minimum width.
	if dx > width {
		width = dx
		dx, dy = 0, 0
		goto retry
	}
	return
}

func (p *FlexBox) Layout(r image.Rectangle, _ image.Point) {
	if p.Direction == DirectionRow {
		p.layoutRow(r)
	} else {
		p.layoutCol(r)
	}

	if resizer, ok := p.Background.(Resizer); ok {
		resizer.SetSize(
			r.Dx()-p.Margin.Left-p.Margin.Right,
			r.Dy()-p.Margin.Top-p.Margin.Bottom,
		)
	}
	p.SetBounds(r)
}

func (p *FlexBox) layoutRow(r image.Rectangle) {
	childLayouts := make([]struct {
		r  image.Rectangle
		sp image.Point
	}, p.ChildrenCount())

	content := inset(r, p.Padding.Add(p.Margin))
	dot := content.Min
	i := 0

	// Measure children and place them justified at JustifyStart
	dxs, _, _ := fairShare(content.Dx(), p.ChildrenCount(), p.Children())
	for child := range p.Children() {
		cdx, cdy := child.Measure(dxs[i])
		lr := dot.Add(image.Pt(cdx, cdy))
		childLayouts[i].r = image.Rectangle{dot, lr}

		if p.Align == AlignStretch {
			childLayouts[i].r.Min.Y = content.Min.Y
			childLayouts[i].r.Max.Y = content.Max.Y
		} else {
			surplus := content.Max.Y - lr.Y
			shift := 0
			switch p.Align {
			case AlignCenter:
				shift = surplus / 2
			case AlignEnd:
				shift = surplus
			}
			childLayouts[i].r.Min.Y += shift
			childLayouts[i].r.Max.Y += shift
		}

		dot.X = lr.X
		i++
	}

	// Justify children
	surplus := content.Max.X - dot.X
	shift, gap := p.justify(surplus)
	for i := range childLayouts {
		childLayouts[i].r.Min.X += shift
		childLayouts[i].r.Max.X += shift
		shift += gap
	}

	// Clip to content and layout children
	i = 0
	for child := range p.Children() {
		l := childLayouts[i]
		l.r = l.r.Intersect(content)
		l.sp = l.r.Min.Sub(childLayouts[i].r.Min)
		child.Layout(l.r, l.sp)
		i++
	}
}

func (p *FlexBox) layoutCol(r image.Rectangle) {
	childLayouts := make([]struct {
		r  image.Rectangle
		sp image.Point
	}, p.ChildrenCount())

	content := inset(r, p.Padding.Add(p.Margin))
	dot := content.Min
	i := 0

	// Measure children and place them justified at JustifyStart
	for child := range p.Children() {
		cdx, cdy := child.Measure(content.Dx())
		lr := dot.Add(image.Pt(cdx, cdy))
		childLayouts[i].r = image.Rectangle{dot, lr}

		if p.Align == AlignStretch {
			childLayouts[i].r.Min.X = content.Min.X
			childLayouts[i].r.Max.X = content.Max.X
		} else {
			surplus := content.Max.X - lr.X
			shift := 0
			switch p.Align {
			case AlignCenter:
				shift = surplus / 2
			case AlignEnd:
				shift = surplus
			}
			childLayouts[i].r.Min.X += shift
			childLayouts[i].r.Max.X += shift
		}

		dot.Y = lr.Y
		i++
	}

	// Justify children
	surplus := content.Max.Y - dot.Y
	shift, gap := p.justify(surplus)
	for i := range childLayouts {
		childLayouts[i].r.Min.Y += shift
		childLayouts[i].r.Max.Y += shift
		shift += gap
	}

	// Clip to content and layout children
	i = 0
	for child := range p.Children() {
		l := childLayouts[i]
		l.r = l.r.Intersect(content)
		l.sp = l.r.Min.Sub(childLayouts[i].r.Min)
		child.Layout(l.r, l.sp)
		i++
	}
}

func (p *FlexBox) justify(surplus int) (shift, gap int) {
	switch p.Justify {
	case JustifyCenter:
		shift = surplus / 2
	case JustifyEnd:
		shift = surplus
	case JustifyBetween:
		gap = max(surplus/(p.ChildrenCount()-1), 0)
		shift = min(0, surplus/2)
	case JustifyAround:
		gap = max(surplus/p.ChildrenCount(), 0)
		shift = min(gap/2, surplus/2)
	}
	return
}

// fairShare splits width among count [Layouter]s such, that each layouter gets
// it's minimum dx if width is too small to fit all children. Otherwise the
// surplus is divided among the children by allowing each to grow to their fair
// share or to their maximum dx.
//
// The dx and dy specify the sum of all dxs and the maximum of all dys. They are
// only valid if there is no surplus and are set to -1 otherwise.
func fairShare(width, count int, children iter.Seq[Node]) (dxs []int, dx, dy int) {
	dxs = make([]int, count)
	indices := make([]int, count)

	surplus := width
	mins := make([]int, count)
	i := 0
	for child := range children {
		cdx, cdy := child.Measure(0)
		dy = max(dy, cdy)
		mins[i] = cdx
		surplus -= mins[i]

		indices[i] = i
		i++
	}
	if surplus <= 0 {
		dxs = mins
		dx = width - surplus
		return
	}
	dx, dy = -1, -1

	// We have a surplus to divide among the children. First determine their
	// growths potential and sort them by that.
	growths := make([]int, count)
	i = 0
	for child := range children {
		cdx, _ := child.Measure(width)
		growths[i] = cdx - mins[i]
		i++
	}

	slices.SortFunc(indices, func(a, b int) int {
		if growths[a] < growths[b] {
			return -1
		} else if growths[a] > growths[b] {
			return 1
		}
		return 0
	})

	// Iterate over the children in order of their growth potential and let
	// each child grow as much as possible, capped by their fair share.
	for i, idx := range indices {
		share := surplus / (len(mins) - i)
		dxs[idx] = mins[idx] + min(share, growths[idx])
		surplus -= min(share, growths[idx])
	}
	return
}
