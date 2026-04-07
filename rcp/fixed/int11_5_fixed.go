package fixed

import "image"

type Int11_5 int16

func Int11_5U(i int) Int11_5     { return Int11_5(i << 5) }
func Int11_5F(f float32) Int11_5 { return Int11_5(f * (1 << 5)) }

func (x Int11_5) Floor() int            { return int(x >> 5) }
func (x Int11_5) Ceil() int             { return int(int32(x) + (1<<5-1)>>5) }
func (x Int11_5) Mul(y Int11_5) Int11_5 { return Int11_5((int32(x) * int32(y)) >> 5) }
func (x Int11_5) Div(y Int11_5) Int11_5 { return Int11_5(int32(x) << 5 / int32(y)) }
func (x Int11_5) String() string        { return asString(int64(x), 5, 4, 2) }

type Point11_5 struct {
	X, Y Int11_5
}

func Pt11_5U(x, y int) Point11_5     { return Point11_5{Int11_5U(x), Int11_5U(y)} }
func Pt11_5F(x, y float32) Point11_5 { return Point11_5{Int11_5F(x), Int11_5F(y)} }

func (p Point11_5) Add(q Point11_5) Point11_5 { return Point11_5{p.X + q.X, p.Y + q.Y} }
func (p Point11_5) Sub(q Point11_5) Point11_5 { return Point11_5{p.X - q.X, p.Y - q.Y} }
func (p Point11_5) Mul(k Int11_5) Point11_5   { return Point11_5{p.X.Mul(k), p.Y.Mul(k)} }
func (p Point11_5) Div(k Int11_5) Point11_5   { return Point11_5{p.X.Div(k), p.Y.Div(k)} }
func (p Point11_5) Pt() image.Point           { return image.Point{p.X.Floor(), p.Y.Floor()} }

type Rectangle11_5 struct {
	Min, Max Point11_5
}

func Rect11_5U(x0, y0, x1, y1 int) Rectangle11_5 {
	return Rectangle11_5{Pt11_5U(x0, y0), Pt11_5U(x1, y1)}
}

func Rect11_5F(x0, y0, x1, y1 float32) Rectangle11_5 {
	return Rectangle11_5{Pt11_5F(x0, y0), Pt11_5F(x1, y1)}
}

func (r Rectangle11_5) Add(p Point11_5) Rectangle11_5 {
	return Rectangle11_5{
		Point11_5{r.Min.X + p.X, r.Min.Y + p.Y},
		Point11_5{r.Max.X + p.X, r.Max.Y + p.Y},
	}
}

func (r Rectangle11_5) Sub(p Point11_5) Rectangle11_5 {
	return Rectangle11_5{
		Point11_5{r.Min.X - p.X, r.Min.Y - p.Y},
		Point11_5{r.Max.X - p.X, r.Max.Y - p.Y},
	}
}

func (r Rectangle11_5) Intersect(s Rectangle11_5) Rectangle11_5 {
	r.Min.X = max(r.Min.X, s.Min.X)
	r.Min.Y = max(r.Min.Y, s.Min.Y)
	r.Max.X = min(r.Max.X, s.Max.X)
	r.Max.Y = min(r.Max.Y, s.Max.Y)
	if r.Empty() {
		return Rectangle11_5{}
	}
	return r
}

func (r Rectangle11_5) Union(s Rectangle11_5) Rectangle11_5 {
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

func (r Rectangle11_5) Empty() bool {
	return r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y
}

func (r Rectangle11_5) In(s Rectangle11_5) bool {
	if r.Empty() {
		return true
	}
	return s.Min.X <= r.Min.X && r.Max.X <= s.Max.X &&
		s.Min.Y <= r.Min.Y && r.Max.Y <= s.Max.Y
}

func (r Rectangle11_5) Rect() image.Rectangle {
	return image.Rectangle{Point11_5(r.Min).Pt(), Point11_5(r.Max).Pt()}
}
