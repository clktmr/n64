package fixed

import "image"

type Int1_15 int16

func Int1_15U(i int) Int1_15     { return Int1_15(i << 15) }
func Int1_15F(f float32) Int1_15 { return Int1_15(f * (1 << 15)) }

func (x Int1_15) Floor() int            { return int(x >> 15) }
func (x Int1_15) Ceil() int             { return int((int32(x) + (1<<15 - 1)) >> 15) }
func (x Int1_15) Mul(y Int1_15) Int1_15 { return Int1_15((int32(x) * int32(y)) >> 15) }
func (x Int1_15) Div(y Int1_15) Int1_15 { return Int1_15(int32(x) << 15 / int32(y)) }
func (x Int1_15) String() string        { return asString(int64(x), 15, 1, 5) }

type Point1_15 struct {
	X, Y Int1_15
}

func Pt1_15U(x, y int) Point1_15      { return Point1_15{Int1_15U(x), Int1_15U(y)} }
func Pt1_15F(x, y float32) Point1_15  { return Point1_15{Int1_15F(x), Int1_15F(y)} }
func Pt1_15P(p image.Point) Point1_15 { return Point1_15{Int1_15U(p.X), Int1_15U(p.Y)} }

func (p Point1_15) Add(q Point1_15) Point1_15 { return Point1_15{p.X + q.X, p.Y + q.Y} }
func (p Point1_15) Sub(q Point1_15) Point1_15 { return Point1_15{p.X - q.X, p.Y - q.Y} }
func (p Point1_15) Mul(k Int1_15) Point1_15   { return Point1_15{p.X.Mul(k), p.Y.Mul(k)} }
func (p Point1_15) Div(k Int1_15) Point1_15   { return Point1_15{p.X.Div(k), p.Y.Div(k)} }
func (p Point1_15) Pt() image.Point           { return image.Point{p.X.Floor(), p.Y.Floor()} }

type Rectangle1_15 struct {
	Min, Max Point1_15
}

func Rect1_15U(x0, y0, x1, y1 int) Rectangle1_15 {
	return Rectangle1_15{Pt1_15U(x0, y0), Pt1_15U(x1, y1)}
}

func Rect1_15F(x0, y0, x1, y1 float32) Rectangle1_15 {
	return Rectangle1_15{Pt1_15F(x0, y0), Pt1_15F(x1, y1)}
}

func Rect1_15R(r image.Rectangle) Rectangle1_15 {
	return Rectangle1_15{Pt1_15P(r.Min), Pt1_15P(r.Max)}
}

func (r Rectangle1_15) Add(p Point1_15) Rectangle1_15 {
	return Rectangle1_15{
		Point1_15{r.Min.X + p.X, r.Min.Y + p.Y},
		Point1_15{r.Max.X + p.X, r.Max.Y + p.Y},
	}
}

func (r Rectangle1_15) Sub(p Point1_15) Rectangle1_15 {
	return Rectangle1_15{
		Point1_15{r.Min.X - p.X, r.Min.Y - p.Y},
		Point1_15{r.Max.X - p.X, r.Max.Y - p.Y},
	}
}

func (r Rectangle1_15) Intersect(s Rectangle1_15) Rectangle1_15 {
	r.Min.X = max(r.Min.X, s.Min.X)
	r.Min.Y = max(r.Min.Y, s.Min.Y)
	r.Max.X = min(r.Max.X, s.Max.X)
	r.Max.Y = min(r.Max.Y, s.Max.Y)
	if r.Empty() {
		return Rectangle1_15{}
	}
	return r
}

func (r Rectangle1_15) Union(s Rectangle1_15) Rectangle1_15 {
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

func (r Rectangle1_15) Empty() bool {
	return r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y
}

func (r Rectangle1_15) In(s Rectangle1_15) bool {
	if r.Empty() {
		return true
	}
	return s.Min.X <= r.Min.X && r.Max.X <= s.Max.X &&
		s.Min.Y <= r.Min.Y && r.Max.Y <= s.Max.Y
}

func (r Rectangle1_15) Rect() image.Rectangle {
	return image.Rectangle{Point1_15(r.Min).Pt(), Point1_15(r.Max).Pt()}
}
