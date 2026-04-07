package fixed

import "image"

type PointU8 struct {
	X, Y uint8
}

func PtU8U(x, y int) PointU8      { return PointU8{uint8(x), uint8(y)} }
func PtU8F(x, y float32) PointU8  { return PointU8{uint8(x), uint8(y)} }
func PtU8P(p image.Point) PointU8 { return PointU8{uint8(p.X), uint8(p.Y)} }

func (p PointU8) Add(q PointU8) PointU8 { return PointU8{p.X + q.X, p.Y + q.Y} }
func (p PointU8) Sub(q PointU8) PointU8 { return PointU8{p.X - q.X, p.Y - q.Y} }
func (p PointU8) Mul(k uint8) PointU8   { return PointU8{p.X * k, p.Y * k} }
func (p PointU8) Div(k uint8) PointU8   { return PointU8{p.X / k, p.Y / k} }
func (p PointU8) Pt() image.Point       { return image.Point{int(p.X), int(p.Y)} }

type RectangleU8 struct {
	Min, Max PointU8
}

func RectU8U(x0, y0, x1, y1 int) RectangleU8 {
	return RectangleU8{PtU8U(x0, y0), PtU8U(x1, y1)}
}

func RectU8F(x0, y0, x1, y1 float32) RectangleU8 {
	return RectangleU8{PtU8F(x0, y0), PtU8F(x1, y1)}
}

func RectU8R(r image.Rectangle) RectangleU8 {
	return RectangleU8{PtU8P(r.Min), PtU8P(r.Max)}
}

func (r RectangleU8) Add(p PointU8) RectangleU8 {
	return RectangleU8{
		PointU8{r.Min.X + p.X, r.Min.Y + p.Y},
		PointU8{r.Max.X + p.X, r.Max.Y + p.Y},
	}
}

func (r RectangleU8) Sub(p PointU8) RectangleU8 {
	return RectangleU8{
		PointU8{r.Min.X - p.X, r.Min.Y - p.Y},
		PointU8{r.Max.X - p.X, r.Max.Y - p.Y},
	}
}

func (r RectangleU8) Intersect(s RectangleU8) RectangleU8 {
	r.Min.X = max(r.Min.X, s.Min.X)
	r.Min.Y = max(r.Min.Y, s.Min.Y)
	r.Max.X = min(r.Max.X, s.Max.X)
	r.Max.Y = min(r.Max.Y, s.Max.Y)
	if r.Empty() {
		return RectangleU8{}
	}
	return r
}

func (r RectangleU8) Union(s RectangleU8) RectangleU8 {
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

func (r RectangleU8) Empty() bool {
	return r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y
}

func (r RectangleU8) In(s RectangleU8) bool {
	if r.Empty() {
		return true
	}
	return s.Min.X <= r.Min.X && r.Max.X <= s.Max.X &&
		s.Min.Y <= r.Min.Y && r.Max.Y <= s.Max.Y
}

func (r RectangleU8) Rect() image.Rectangle {
	return image.Rectangle{PointU8(r.Min).Pt(), PointU8(r.Max).Pt()}
}
