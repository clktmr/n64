package fixed

import "image"

type UInt20_12 uint32

func UInt20_12U(i int) UInt20_12     { return UInt20_12(i << 12) }
func UInt20_12F(f float32) UInt20_12 { return UInt20_12(f * (1 << 12)) }

func (x UInt20_12) Floor() int                { return int(x >> 12) }
func (x UInt20_12) Ceil() int                 { return int((uint64(x) + (1<<12 - 1)) >> 12) }
func (x UInt20_12) Mul(y UInt20_12) UInt20_12 { return UInt20_12((uint64(x) * uint64(y)) >> 12) }
func (x UInt20_12) Div(y UInt20_12) UInt20_12 { return UInt20_12(uint64(x) << 12 / uint64(y)) }
func (x UInt20_12) String() string            { return asString(int64(x), 12, 7, 4) }

type PointU20_12 struct {
	X, Y UInt20_12
}

func PtU20_12U(x, y int) PointU20_12      { return PointU20_12{UInt20_12U(x), UInt20_12U(y)} }
func PtU20_12F(x, y float32) PointU20_12  { return PointU20_12{UInt20_12F(x), UInt20_12F(y)} }
func PtU20_12P(p image.Point) PointU20_12 { return PointU20_12{UInt20_12U(p.X), UInt20_12U(p.Y)} }

func (p PointU20_12) Add(q PointU20_12) PointU20_12 { return PointU20_12{p.X + q.X, p.Y + q.Y} }
func (p PointU20_12) Sub(q PointU20_12) PointU20_12 { return PointU20_12{p.X - q.X, p.Y - q.Y} }
func (p PointU20_12) Mul(k UInt20_12) PointU20_12   { return PointU20_12{p.X.Mul(k), p.Y.Mul(k)} }
func (p PointU20_12) Div(k UInt20_12) PointU20_12   { return PointU20_12{p.X.Div(k), p.Y.Div(k)} }
func (p PointU20_12) Pt() image.Point               { return image.Point{p.X.Floor(), p.Y.Floor()} }

type RectangleU20_12 struct {
	Min, Max PointU20_12
}

func RectU20_12U(x0, y0, x1, y1 int) RectangleU20_12 {
	return RectangleU20_12{PtU20_12U(x0, y0), PtU20_12U(x1, y1)}
}

func RectU20_12F(x0, y0, x1, y1 float32) RectangleU20_12 {
	return RectangleU20_12{PtU20_12F(x0, y0), PtU20_12F(x1, y1)}
}

func RectU20_12R(r image.Rectangle) RectangleU20_12 {
	return RectangleU20_12{PtU20_12P(r.Min), PtU20_12P(r.Max)}
}

func (r RectangleU20_12) Add(p PointU20_12) RectangleU20_12 {
	return RectangleU20_12{
		PointU20_12{r.Min.X + p.X, r.Min.Y + p.Y},
		PointU20_12{r.Max.X + p.X, r.Max.Y + p.Y},
	}
}

func (r RectangleU20_12) Sub(p PointU20_12) RectangleU20_12 {
	return RectangleU20_12{
		PointU20_12{r.Min.X - p.X, r.Min.Y - p.Y},
		PointU20_12{r.Max.X - p.X, r.Max.Y - p.Y},
	}
}

func (r RectangleU20_12) Intersect(s RectangleU20_12) RectangleU20_12 {
	r.Min.X = max(r.Min.X, s.Min.X)
	r.Min.Y = max(r.Min.Y, s.Min.Y)
	r.Max.X = min(r.Max.X, s.Max.X)
	r.Max.Y = min(r.Max.Y, s.Max.Y)
	if r.Empty() {
		return RectangleU20_12{}
	}
	return r
}

func (r RectangleU20_12) Union(s RectangleU20_12) RectangleU20_12 {
	if r.Empty() {
		return s
	}
	if s.Empty() {
		return r
	}
	r.Min.X = min(r.Min.X, s.Min.X)
	r.Min.Y = min(r.Min.Y, s.Min.Y)
	r.Max.X = max(r.Max.X, s.Max.X)
	r.Max.Y = max(r.Max.Y, s.Max.Y)
	return r
}

func (r RectangleU20_12) Empty() bool {
	return r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y
}

func (r RectangleU20_12) In(s RectangleU20_12) bool {
	if r.Empty() {
		return true
	}
	return s.Min.X <= r.Min.X && r.Max.X <= s.Max.X &&
		s.Min.Y <= r.Min.Y && r.Max.Y <= s.Max.Y
}

func (r RectangleU20_12) Rect() image.Rectangle {
	return image.Rectangle{PointU20_12(r.Min).Pt(), PointU20_12(r.Max).Pt()}
}
