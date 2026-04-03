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

func BenchmarkAddImage(b *testing.B) {
	var r image.Rectangle
	r.Min.X = 2
	r.Min.Y = 1

	bLoop(b, func() {
		r = r.Add(r.Min)
		r = r.Add(r.Min)
		r = r.Add(r.Min)
		r = r.Add(r.Min)
	})
}

func BenchmarkAdd(b *testing.B) {
	b.Run("UInt8", benchmarkAdd[fixed.UInt8])
	b.Run("UInt14_2", benchmarkAdd[fixed.UInt14_2])
	b.Run("UInt64", benchmarkAdd[fixed.UInt64])
}

func benchmarkAdd[T fixed.Fixed[T]](b *testing.B) {
	var r fixed.Rectangle[T]
	r.Min.X = 2
	r.Min.Y = 1

	bLoop(b, func() {
		r = r.Add(r.Min)
		r = r.Add(r.Min)
		r = r.Add(r.Min)
		r = r.Add(r.Min)
	})
}

func BenchmarkIntersectImage(b *testing.B) {
	var r1, r2 image.Rectangle

	bLoop(b, func() {
		r1 = r1.Intersect(r2)
		r1 = r1.Intersect(r2)
		r1 = r1.Intersect(r2)
		r1 = r1.Intersect(r2)
	})
}

func BenchmarkIntersect(b *testing.B) {
	b.Run("UInt8", benchmarkIntersect[fixed.UInt8])
	b.Run("UInt14_2", benchmarkIntersect[fixed.UInt14_2])
	b.Run("UInt64", benchmarkIntersect[fixed.UInt64])
}

func benchmarkIntersect[T fixed.Fixed[T]](b *testing.B) {
	var r1, r2 fixed.Rectangle[T]

	bLoop(b, func() {
		r1 = r1.Intersect(r2)
		r1 = r1.Intersect(r2)
		r1 = r1.Intersect(r2)
		r1 = r1.Intersect(r2)
	})
}

func BenchmarkRect(b *testing.B) {
	var r1 fixed.Rectangle[fixed.UInt8]

	bLoop(b, func() {
		fixed.Rect(r1)
		fixed.Rect(r1)
		fixed.Rect(r1)
		fixed.Rect(r1)
	})
}
