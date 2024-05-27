package cpu_test

import (
	"n64/rcp/cpu"
	"testing"
)

func TestMakePaddedSlice(t *testing.T) {
	for i := range 64 {
		slice := cpu.MakePaddedSlice(i)
		if len(slice) != i {
			t.Errorf("wrong length: expected %v, got %v", i, len(slice))
		}
		if !cpu.IsPadded(slice) {
			t.Errorf("got unpadded slice for len=%v, %v", i, cap(slice))
		}
	}
}
