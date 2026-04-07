package fixed

import "image"

type Int6_10 int16

func Int6_10U(i int) Int6_10     { return Int6_10(i << 10) }
func Int6_10F(f float32) Int6_10 { return Int6_10(f * (1 << 10)) }

func (x Int6_10) Floor() int            { return int(x >> 10) }
func (x Int6_10) Ceil() int             { return int(int32(x) + (1<<10-1)>>10) }
func (x Int6_10) Mul(y Int6_10) Int6_10 { return Int6_10((int32(x) * int32(y)) >> 10) }
func (x Int6_10) Div(y Int6_10) Int6_10 { return Int6_10(int32(x) << 10 / int32(y)) }
func (x Int6_10) String() string        { return asString(int64(x), 10, 2, 4) }

type Point6_10 struct {
	X, Y Int6_10
}

func Pt6_10U(x, y int) Point6_10      { return Point6_10{Int6_10U(x), Int6_10U(y)} }
func Pt6_10F(x, y float32) Point6_10  { return Point6_10{Int6_10F(x), Int6_10F(y)} }
func Pt6_10P(p image.Point) Point6_10 { return Point6_10{Int6_10U(p.X), Int6_10U(p.Y)} }

func (p Point6_10) Add(q Point6_10) Point6_10 { return Point6_10{p.X + q.X, p.Y + q.Y} }
func (p Point6_10) Sub(q Point6_10) Point6_10 { return Point6_10{p.X - q.X, p.Y - q.Y} }
func (p Point6_10) Mul(k Int6_10) Point6_10   { return Point6_10{p.X.Mul(k), p.Y.Mul(k)} }
func (p Point6_10) Div(k Int6_10) Point6_10   { return Point6_10{p.X.Div(k), p.Y.Div(k)} }
func (p Point6_10) Pt() image.Point           { return image.Point{p.X.Floor(), p.Y.Floor()} }

type Rectangle6_10 struct {
	Min, Max Point6_10
}

func Rect6_10U(x0, y0, x1, y1 int) Rectangle6_10 {
	return Rectangle6_10{Pt6_10U(x0, y0), Pt6_10U(x1, y1)}
}

func Rect6_10F(x0, y0, x1, y1 float32) Rectangle6_10 {
	return Rectangle6_10{Pt6_10F(x0, y0), Pt6_10F(x1, y1)}
}

func Rect6_10R(r image.Rectangle) Rectangle6_10 {
	return Rectangle6_10{Pt6_10P(r.Min), Pt6_10P(r.Max)}
}

func (r Rectangle6_10) Add(p Point6_10) Rectangle6_10 {
	return Rectangle6_10{
		Point6_10{r.Min.X + p.X, r.Min.Y + p.Y},
		Point6_10{r.Max.X + p.X, r.Max.Y + p.Y},
	}
}

func (r Rectangle6_10) Sub(p Point6_10) Rectangle6_10 {
	return Rectangle6_10{
		Point6_10{r.Min.X - p.X, r.Min.Y - p.Y},
		Point6_10{r.Max.X - p.X, r.Max.Y - p.Y},
	}
}

func (r Rectangle6_10) Intersect(s Rectangle6_10) Rectangle6_10 {
	r.Min.X = max(r.Min.X, s.Min.X)
	r.Min.Y = max(r.Min.Y, s.Min.Y)
	r.Max.X = min(r.Max.X, s.Max.X)
	r.Max.Y = min(r.Max.Y, s.Max.Y)
	if r.Empty() {
		return Rectangle6_10{}
	}
	return r
}

func (r Rectangle6_10) Union(s Rectangle6_10) Rectangle6_10 {
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

func (r Rectangle6_10) Empty() bool {
	return r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y
}

func (r Rectangle6_10) In(s Rectangle6_10) bool {
	if r.Empty() {
		return true
	}
	return s.Min.X <= r.Min.X && r.Max.X <= s.Max.X &&
		s.Min.Y <= r.Min.Y && r.Max.Y <= s.Max.Y
}

func (r Rectangle6_10) Rect() image.Rectangle {
	return image.Rectangle{Point6_10(r.Min).Pt(), Point6_10(r.Max).Pt()}
}
