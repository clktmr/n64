package ui

import (
	"image"
	"slices"
	"testing"

	n64testing "github.com/clktmr/n64/testing"
)

func TestMain(m *testing.M) { n64testing.TestMain(m) }

type FakeLayouter func(width int) (dx, dy int)

func (i FakeLayouter) Measure(width int) (dx, dy int)      { return i(width) }
func (i FakeLayouter) Layout(image.Rectangle, image.Point) {}

func TestFairShare(t *testing.T) {
	tests := map[string]struct {
		width   int
		mins    []int
		maxs    []int
		widths  []int
		surplus int
	}{
		"fit":        {10, []int{4, 1, 5}, []int{4, 1, 5}, []int{4, 1, 5}, 0},
		"underflow1": {10, []int{1, 1, 1}, []int{1, 1, 1}, []int{1, 1, 1}, 7},
		"underflow2": {10, []int{1, 1, 1}, []int{3, 3, 3}, []int{3, 3, 3}, 1},
		"overflow1":  {10, []int{2, 5, 5}, []int{2, 5, 5}, []int{2, 5, 5}, -2},
		"overflow2":  {10, []int{2, 9, 5}, []int{2, 9, 5}, []int{2, 9, 5}, -6},
		"share1":     {10, []int{1, 1, 1}, []int{11, 11, 11}, []int{3, 3, 4}, 0},
		"share2":     {10, []int{1, 1, 1}, []int{1, 11, 16}, []int{1, 4, 5}, 0},
		"share3":     {10, []int{2, 1, 1}, []int{2, 11, 16}, []int{2, 4, 4}, 0},
		"needsSort":  {10, []int{1, 1, 2}, []int{16, 11, 2}, []int{4, 4, 2}, 0},
		"regression": {100, []int{18, 69}, []int{112, 112}, []int{25, 75}, 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			iter := func(yield func(Layouter) bool) {
				for i := 0; i < len(tc.mins); i++ {
					if !yield(FakeLayouter(func(width int) (dx, dy int) {
						return min(max(tc.mins[i], width), tc.maxs[i]), 0
					})) {
						break
					}
				}
			}
			widths, dx, _ := fairShare(tc.width, len(tc.mins), iter)
			if dx >= 0 && dx != tc.width-tc.surplus {
				t.Errorf("expected dx %v, got %v", tc.width-tc.surplus, dx)
			}
			if !slices.Equal(widths, tc.widths) {
				t.Errorf("expected widths %v, got %v", tc.widths, widths)
			}
		})
	}
}
