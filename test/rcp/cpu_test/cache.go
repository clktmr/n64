package cpu_test

import (
	"n64/rcp/cpu"
	"testing"
	"unsafe"
)

func testMakePaddedSlice[T any](t *testing.T) {
	for i := range 64 {
		slice := cpu.MakePaddedSlice[T](i)
		if len(slice) != i {
			t.Errorf("wrong length: expected %v, got %v", i, len(slice))
		}
		if !cpu.IsPadded(slice) {
			t.Errorf("got unpadded slice for len=%v, %v", i, cap(slice))
		}
	}
}

func TestMakePaddedSlice(t *testing.T) {
	t.Run("byte", testMakePaddedSlice[byte])
	t.Run("uint16", testMakePaddedSlice[uint16])
	t.Run("uint32", testMakePaddedSlice[uint32])
	t.Run("uint64", testMakePaddedSlice[uint64])
}

func testMakePaddedSliceAligned[T any](t *testing.T) {
	for i := range 64 {
		for _, align := range []uintptr{8, 16, 32, 64, 128, 256} {
			slice := cpu.MakePaddedSliceAligned[T](i, align)
			if len(slice) != i {
				t.Errorf("wrong length: expected %v, got %v", i, len(slice))
			}
			if !cpu.IsPadded(slice) {
				t.Errorf("got unpadded slice for len=%v, %v", i, cap(slice))
			}
			if uintptr(unsafe.Pointer(unsafe.SliceData(slice)))%align != 0 {
				t.Errorf("got unaligned slice for len=%v, %v", i, cap(slice))
			}
		}
	}
}

func TestMakePaddedSliceAligned(t *testing.T) {
	t.Run("byte", testMakePaddedSliceAligned[byte])
	t.Run("uint16", testMakePaddedSliceAligned[uint16])
	t.Run("uint32", testMakePaddedSliceAligned[uint32])
	t.Run("uint64", testMakePaddedSliceAligned[uint64])
}
