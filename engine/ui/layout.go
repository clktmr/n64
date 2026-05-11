package ui

import (
	"image"
	_ "image/draw"
)

// Layouter reports it's dimensions and scaling constraints and can be placed
// according to these.
type Layouter interface {
	// Measure returns the nodes dimensions for a desired width, assuming
	// that no content is clipped.
	Measure(width int) (dx, dy int)

	// Layout informs the node where to draw in the next rendering pass. The
	// parameters have the same meaning as in [draw.Drawer].
	// The rectangle {Min: r.Min-sp, Max: r.Max} must have the same or
	// smaller dimensions than returned from [Measure] call.
	Layout(r image.Rectangle, sp image.Point)
}

type FocusDirection int

const (
	Up FocusDirection = iota
	Down
	Left
	Right
)

func (d FocusDirection) Vertical() bool   { return d == Up || d == Down }
func (d FocusDirection) Horizontal() bool { return d == Left || d == Right }

type Focuser interface {
	// Focus shifts the focus to the next [Node] in the specified direction.
	// It returns true if the requested change in focus was applied.
	Focus(d FocusDirection) (changed bool)

	// Focused returns the index currently focused child.
	Focused() (index int)
}

// Resizer implements an image.Image with flexible dimensions.
type Resizer interface {
	SetSize(width, height int)
}
