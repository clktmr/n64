package fixed_test

import (
	"image"
	"testing"

	"github.com/clktmr/n64/rcp/fixed"
	n64testing "github.com/clktmr/n64/testing"
)

func TestMain(m *testing.M) { n64testing.TestMain(m) }

// bLoop benchmarks fn. In this package we want to ensure some calls get always
// inlined, but inlining is disabled in the b.Loop() for loop's body. Putting
// the code to measure in fn allows inlining inside of fn.
func bLoop(b *testing.B, fn func()) {
	for b.Loop() {
		fn()
	}
}

func BenchmarkAdd(b *testing.B) {
	b.Run("image.Rectangle", func(b *testing.B) {
		r := image.Rect(1, 1, 1, 1)
		p := image.Pt(1, 1)
		bLoop(b, func() { r = r.Add(p); r = r.Add(p); r = r.Add(p); r = r.Add(p) })
	})
	b.Run("RectangleU8", func(b *testing.B) {
		r := fixed.RectU8U(1, 1, 1, 1)
		p := fixed.PtU8U(1, 1)
		bLoop(b, func() { r = r.Add(p); r = r.Add(p); r = r.Add(p); r = r.Add(p) })
	})
	b.Run("RectangleU14_2", func(b *testing.B) {
		r := fixed.RectU14_2U(1, 1, 1, 1)
		p := fixed.PtU14_2U(1, 1)
		bLoop(b, func() { r = r.Add(p); r = r.Add(p); r = r.Add(p); r = r.Add(p) })
	})
}

func BenchmarkIntersect(b *testing.B) {
	b.Run("image.Rectangle", func(b *testing.B) {
		r := image.Rect(1, 1, 1, 1)
		bLoop(b, func() { r = r.Intersect(r); r = r.Intersect(r); r = r.Intersect(r); r = r.Intersect(r) })
	})
	b.Run("RectangleU8", func(b *testing.B) {
		r := fixed.RectU8U(1, 1, 1, 1)
		bLoop(b, func() { r = r.Intersect(r); r = r.Intersect(r); r = r.Intersect(r); r = r.Intersect(r) })
	})
	b.Run("RectangleU14_2", func(b *testing.B) {
		r := fixed.RectU14_2U(1, 1, 1, 1)
		bLoop(b, func() { r = r.Intersect(r); r = r.Intersect(r); r = r.Intersect(r); r = r.Intersect(r) })
	})
}

func BenchmarkRect(b *testing.B) {
	var r2 image.Rectangle
	b.Run("RectangleU8", func(b *testing.B) {
		r := fixed.RectU8U(1, 1, 1, 1)
		bLoop(b, func() { r2 = r.Rect() })
	})
	b.Run("RectangleU14_2", func(b *testing.B) {
		r := fixed.RectU14_2U(1, 1, 1, 1)
		bLoop(b, func() { r2 = r.Rect() })
	})
	_ = r2
}
