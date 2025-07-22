// Package fixed provides fixed-point arithmetic types used by the RCP.
package fixed

import "golang.org/x/exp/constraints"

//go:generate go run mkfixed.go UInt14_2 uint16
type UInt14_2 uint16

//go:generate go run mkfixed.go Int11_5 int16
type Int11_5 int16

//go:generate go run mkfixed.go Int6_10 int16
type Int6_10 int16

type Fixed[T any] interface {
	constraints.Integer

	Mul(T) T
	Div(T) T
}

type Point[T Fixed[T]] struct {
	X, Y T
}

// Add returns the vector p+q.
func (p Point[T]) Add(q Point[T]) Point[T] {
	return Point[T]{p.X + q.X, p.Y + q.Y}
}

// Sub returns the vector p-q.
func (p Point[T]) Sub(q Point[T]) Point[T] {
	return Point[T]{p.X - q.X, p.Y - q.Y}
}

// Mul returns the vector p*k.
func (p Point[T]) Mul(k T) Point[T] {
	return Point[T]{p.X.Mul(k), p.Y.Mul(k)}
}

// Div returns the vector p/k.
func (p Point[T]) Div(k T) Point[T] {
	return Point[T]{p.X.Div(k), p.Y.Div(k)}
}

// Rectangle is a fixed-point coordinate rectangle. The Min bound is inclusive
// and the Max bound is exclusive. It is well-formed if Min.X <= Max.X and
// likewise for Y.
//
// It is analogous to the image.Rectangle type in the standard library.
type Rectangle[T Fixed[T]] struct {
	Min, Max Point[T]
}

// Add returns the rectangle r translated by p.
func (r Rectangle[T]) Add(p Point[T]) Rectangle[T] {
	return Rectangle[T]{
		Point[T]{r.Min.X + p.X, r.Min.Y + p.Y},
		Point[T]{r.Max.X + p.X, r.Max.Y + p.Y},
	}
}

// Sub returns the rectangle r translated by -p.
func (r Rectangle[T]) Sub(p Point[T]) Rectangle[T] {
	return Rectangle[T]{
		Point[T]{r.Min.X - p.X, r.Min.Y - p.Y},
		Point[T]{r.Max.X - p.X, r.Max.Y - p.Y},
	}
}

// Intersect returns the largest rectangle contained by both r and s. If the two
// rectangles do not overlap then the zero rectangle will be returned.
func (r Rectangle[T]) Intersect(s Rectangle[T]) Rectangle[T] {
	if r.Min.X < s.Min.X {
		r.Min.X = s.Min.X
	}
	if r.Min.Y < s.Min.Y {
		r.Min.Y = s.Min.Y
	}
	if r.Max.X > s.Max.X {
		r.Max.X = s.Max.X
	}
	if r.Max.Y > s.Max.Y {
		r.Max.Y = s.Max.Y
	}
	// Letting r0 and s0 be the values of r and s at the time that the method
	// is called, this next line is equivalent to:
	//
	// if max(r0.Min.X, s0.Min.X) >= min(r0.Max.X, s0.Max.X) || likewiseForY { etc }
	if r.Empty() {
		return Rectangle[T]{}
	}
	return r
}

// Union returns the smallest rectangle that contains both r and s.
func (r Rectangle[T]) Union(s Rectangle[T]) Rectangle[T] {
	if r.Empty() {
		return s
	}
	if s.Empty() {
		return r
	}
	if r.Min.X > s.Min.X {
		r.Min.X = s.Min.X
	}
	if r.Min.Y > s.Min.Y {
		r.Min.Y = s.Min.Y
	}
	if r.Max.X < s.Max.X {
		r.Max.X = s.Max.X
	}
	if r.Max.Y < s.Max.Y {
		r.Max.Y = s.Max.Y
	}
	return r
}

// Empty returns whether the rectangle contains no points.
func (r Rectangle[T]) Empty() bool {
	return r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y
}

// In returns whether every point in r is in s.
func (r Rectangle[T]) In(s Rectangle[T]) bool {
	if r.Empty() {
		return true
	}
	// Note that r.Max is an exclusive bound for r, so that r.In(s)
	// does not require that r.Max.In(s).
	return s.Min.X <= r.Min.X && r.Max.X <= s.Max.X &&
		s.Min.Y <= r.Min.Y && r.Max.Y <= s.Max.Y
}
