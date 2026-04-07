package fixed

import "image"

type UInt14_2 uint16

func UInt14_2U(i int) UInt14_2     { return UInt14_2(i << 2) }
func UInt14_2F(f float32) UInt14_2 { return UInt14_2(f * (1 << 2)) }

func (x UInt14_2) Floor() int              { return int(x >> 2) }
func (x UInt14_2) Ceil() int               { return int(uint32(x) + (1<<2-1)>>2) }
func (x UInt14_2) Mul(y UInt14_2) UInt14_2 { return UInt14_2((uint32(x) * uint32(y)) >> 2) }
func (x UInt14_2) Div(y UInt14_2) UInt14_2 { return UInt14_2(uint32(x) << 2 / uint32(y)) }
func (x UInt14_2) String() string          { return asString(int64(x), 2, 5, 1) }

type PointU14_2 struct {
	X, Y UInt14_2
}

func PtU14_2U(x, y int) PointU14_2      { return PointU14_2{UInt14_2U(x), UInt14_2U(y)} }
func PtU14_2F(x, y float32) PointU14_2  { return PointU14_2{UInt14_2F(x), UInt14_2F(y)} }
func PtU14_2P(p image.Point) PointU14_2 { return PointU14_2{UInt14_2U(p.X), UInt14_2U(p.Y)} }

func (p PointU14_2) Add(q PointU14_2) PointU14_2 { return PointU14_2{p.X + q.X, p.Y + q.Y} }
func (p PointU14_2) Sub(q PointU14_2) PointU14_2 { return PointU14_2{p.X - q.X, p.Y - q.Y} }
func (p PointU14_2) Mul(k UInt14_2) PointU14_2   { return PointU14_2{p.X.Mul(k), p.Y.Mul(k)} }
func (p PointU14_2) Div(k UInt14_2) PointU14_2   { return PointU14_2{p.X.Div(k), p.Y.Div(k)} }
func (p PointU14_2) Pt() image.Point             { return image.Point{p.X.Floor(), p.Y.Floor()} }

type RectangleU14_2 struct {
	Min, Max PointU14_2
}

func RectU14_2U(x0, y0, x1, y1 int) RectangleU14_2 {
	return RectangleU14_2{PtU14_2U(x0, y0), PtU14_2U(x1, y1)}
}

func RectU14_2F(x0, y0, x1, y1 float32) RectangleU14_2 {
	return RectangleU14_2{PtU14_2F(x0, y0), PtU14_2F(x1, y1)}
}

func RectU14_2R(r image.Rectangle) RectangleU14_2 {
	return RectangleU14_2{PtU14_2P(r.Min), PtU14_2P(r.Max)}
}

func (r RectangleU14_2) Add(p PointU14_2) RectangleU14_2 {
	return RectangleU14_2{
		PointU14_2{r.Min.X + p.X, r.Min.Y + p.Y},
		PointU14_2{r.Max.X + p.X, r.Max.Y + p.Y},
	}
}

func (r RectangleU14_2) Sub(p PointU14_2) RectangleU14_2 {
	return RectangleU14_2{
		PointU14_2{r.Min.X - p.X, r.Min.Y - p.Y},
		PointU14_2{r.Max.X - p.X, r.Max.Y - p.Y},
	}
}

func (r RectangleU14_2) Intersect(s RectangleU14_2) RectangleU14_2 {
	r.Min.X = max(r.Min.X, s.Min.X)
	r.Min.Y = max(r.Min.Y, s.Min.Y)
	r.Max.X = min(r.Max.X, s.Max.X)
	r.Max.Y = min(r.Max.Y, s.Max.Y)
	if r.Empty() {
		return RectangleU14_2{}
	}
	return r
}

func (r RectangleU14_2) Union(s RectangleU14_2) RectangleU14_2 {
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

func (r RectangleU14_2) Empty() bool {
	return r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y
}

func (r RectangleU14_2) In(s RectangleU14_2) bool {
	if r.Empty() {
		return true
	}
	return s.Min.X <= r.Min.X && r.Max.X <= s.Max.X &&
		s.Min.Y <= r.Min.Y && r.Max.Y <= s.Max.Y
}

func (r RectangleU14_2) Rect() image.Rectangle {
	return image.Rectangle{PointU14_2(r.Min).Pt(), PointU14_2(r.Max).Pt()}
}
