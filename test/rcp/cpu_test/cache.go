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

func assertPadAdded[T cpu.Paddable](t *testing.T, slice, pslice []T, head, tail int) {
	if cpu.IsPadded(pslice) == false {
		t.Errorf("got unpadded slice")
	}
	if len(pslice)+head+tail != len(slice) {
		t.Errorf("length don't match")
	}
}

func testPaddedSubslice[T cpu.Paddable](t *testing.T) {
	var tt T
	cls := cpu.CacheLineSize / int(unsafe.Sizeof(tt))
	for i := range 64 {
		slice := cpu.MakePaddedSlice[T](i)
		pslice, head, tail := cpu.PaddedSlice(slice)
		if len(slice) != len(pslice) || head != 0 || tail != 0 {
			t.Fatalf("%v: unnecessary padding added: before=%v, after=%v, head=%v, tail=%v",
				i, len(slice), len(pslice), head, tail)
		}

		if i < 1 {
			continue
		}

		tslice := slice[1:]
		pslice, head, tail = cpu.PaddedSlice(slice[1:])
		assertPadAdded(t, tslice, pslice, head, tail)
		if len(pslice) > 0 && (head != cls-1 || tail != 0) {
			t.Fatalf("%v: wrong padding: head=%v, tail=%v", i, head, tail)
		}

		tslice = slice[:len(slice)-1]
		pslice, head, tail = cpu.PaddedSlice(slice[:len(slice)-1])
		assertPadAdded(t, tslice, pslice, head, tail)
		if head != 0 || tail != 0 {
			t.Fatalf("wrong padding: head=%v, tail=%v", head, tail)
		}

		tslice = slice[:cap(slice)]
		pslice, head, tail = cpu.PaddedSlice(slice[:cap(slice)])
		assertPadAdded(t, tslice, pslice, head, tail)
		if head != 0 || tail != cap(slice)%cls {
			t.Fatalf("%v: wrong padding: head=%v, tail=%v", i, head, tail)
		}
	}
}

func TestPaddedSubslice(t *testing.T) {
	t.Run("byte", testPaddedSubslice[uint8])
	t.Run("uint16", testPaddedSubslice[uint16])
	t.Run("uint32", testPaddedSubslice[uint32])
	t.Run("uint64", testPaddedSubslice[uint64])
}
