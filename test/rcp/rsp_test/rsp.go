package rsp_test

import (
	"bytes"
	"n64/rcp/cpu"
	"n64/rcp/rsp"
	"testing"
)

func TestDMA(t *testing.T) {
	rsp.Init()
	testdata := cpu.MakePaddedSlice(80)
	for i := range len(testdata) {
		testdata[i] = byte(i)
	}
	rsp.DMAStore(0x100, testdata, rsp.DMEM)

	result := rsp.DMALoad(0x100, len(testdata), rsp.DMEM)
	if !bytes.Equal(testdata, result) {
		t.Error("exptected to read same data back that was written")
	}

	shift := 0x20
	result = rsp.DMALoad(0x100+uintptr(shift), len(testdata)-shift, rsp.DMEM)
	if !bytes.Equal(testdata[shift:len(testdata)], result) {
		t.Error("exptected to read part of same data back that was written")
	}
}
