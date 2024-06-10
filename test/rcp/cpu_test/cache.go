package cpu_test

import (
	"testing"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

func assertPadded[T cpu.Paddable](t *testing.T, slice []T, length int, align uintptr) {
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(slice)))
	if len(slice) != length {
		t.Fatalf("wrong length: expected %v, got %v", length, len(slice))
	}
	if !cpu.IsPadded(slice) {
		t.Fatalf("got unpadded slice for len=%v: addr=0x%x, cap=%v", length, addr, cap(slice))
	}
	if addr%align != 0 {
		t.Fatalf("got unaligned slice for len=%v, %v", length, cap(slice))
	}
}

func testMakePaddedSlice[T cpu.Paddable](t *testing.T) {
	for i := range 64 {
		slice := cpu.MakePaddedSlice[T](i)
		assertPadded(t, slice, i, 1)
	}
}

func TestMakePaddedSlice(t *testing.T) {
	t.Run("byte", testMakePaddedSlice[uint8])
	t.Run("uint16", testMakePaddedSlice[uint16])
	t.Run("uint32", testMakePaddedSlice[uint32])
	t.Run("uint64", testMakePaddedSlice[uint64])
}

func testMakePaddedSliceAligned[T cpu.Paddable](t *testing.T) {
	for i := range 64 {
		for _, align := range []uintptr{2, 4, 8, 16, 32, 64, 128, 256} {
			slice := cpu.MakePaddedSliceAligned[T](i, align)
			assertPadded(t, slice, i, 1)
		}
	}
}

func TestMakePaddedSliceAligned(t *testing.T) {
	t.Run("byte", testMakePaddedSliceAligned[uint8])
	t.Run("uint16", testMakePaddedSliceAligned[uint16])
	t.Run("uint32", testMakePaddedSliceAligned[uint32])
	t.Run("uint64", testMakePaddedSliceAligned[uint64])
}
