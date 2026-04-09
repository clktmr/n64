package fixed

import (
	"image"

	"github.com/clktmr/n64/debug"
)

type RectangleU10_2 uint64

const mask RectangleU10_2 = (1 << 12) - 1

func RectU10_2R(r image.Rectangle) RectangleU10_2 {
	return RectU10_2(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y)
}

func RectU10_2(x0, y0, x1, y1 int) RectangleU10_2 {
	debug.Assert(image.Rect(x0, y0, x1, y1).In(image.Rect(0, 0, 1024, 1024)), "overflow")
	return RectangleU10_2(x1<<46 | y1<<34 | x0<<14 | y0<<2)
}

func (r RectangleU10_2) Intersect(s RectangleU10_2) (t RectangleU10_2) {
	mask := mask // TODO Does this improve performance?
	t = max(r&(mask<<44), s&(mask<<44))
	t |= max(r&(mask<<32), s&(mask<<32))
	t |= min(r&(mask<<12), s&(mask<<12))
	t |= min(r&mask, s&mask)
	if t.Empty() {
		return RectangleU10_2(0)
	}
	return
}

func (r RectangleU10_2) Rect() image.Rectangle {
	mask := mask // TODO Does this improve performance?
	return image.Rectangle{
		image.Point{int((r >> 14) & mask), int((r >> 2) & mask)},
		image.Point{int((r >> 46) & mask), int((r >> 34) & mask)},
	}
}

func (r RectangleU10_2) Empty() bool {
	mask := mask
	return (r>>44)&mask >= (r>>12)&mask || (r>>32)&mask >= r&mask
}
