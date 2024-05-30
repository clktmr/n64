package rsp_test

import (
	"bytes"
	"n64/rcp"
	"n64/rcp/cpu"
	"n64/rcp/rsp"
	"testing"
	"time"
	"unsafe"
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

func TestRun(t *testing.T) {
	// Simple program that will swap the first two words in DMEM
	code := []byte{
		0x3c, 0x09, 0xa4, 0x00, //lui   t1,0xa400
		0x8d, 0x29, 0x00, 0x00, //lw    t1,0(t1)
		0x3c, 0x0a, 0xa4, 0x00, //lui   t2,0xa400
		0x8d, 0x4a, 0x00, 0x04, //lw    t2,4(t2)
		0x3c, 0x01, 0xa4, 0x00, //lui   at,0xa400
		0xac, 0x2a, 0x00, 0x00, //sw    t2,0(at)
		0x3c, 0x01, 0xa4, 0x00, //lui   at,0xa400
		0xac, 0x29, 0x00, 0x04, //sw    t1,4(at)
		0x00, 0x00, 0x00, 0x0d, //break
	}
	data := []byte{
		0xde, 0xad, 0xbe, 0xef,
		0xbe, 0xef, 0xf0, 0x0d,
	}
	ucode := rsp.NewUCode("testcode", uint32(rsp.IMEM&0xffffffff), code, data)
	ucode.Load()

	result0 := (*uint32)(unsafe.Pointer(rsp.DMEM))
	result1 := (*uint32)(unsafe.Pointer(rsp.DMEM + 4))

	if *result0 != 0xdeadbeef || *result1 != 0xbeeff00d {
		t.Fatal("failed to load ucode data")
	}

	ucode.Run()

	if *result0 != 0xbeeff00d || *result1 != 0xdeadbeef {
		t.Fatalf("unexpected result after ucode execution: 0x%x 0x%x", *result0, *result1)
	}
}

func TestInterrupt(t *testing.T) {
	t.Cleanup(func() {
		rcp.DisableInterrupts(rcp.SignalProcessor)
		rsp.InterruptOnBreak(false)
	})

	rcp.EnableInterrupts(rcp.SignalProcessor)
	rsp.InterruptOnBreak(true)

	code := []byte{
		0x00, 0x00, 0x00, 0x0d, //break
	}
	data := []byte{}
	ucode := rsp.NewUCode("testcode", uint32(rsp.IMEM&0xffffffff), code, data)
	ucode.Load()

	rsp.IntBreak.Clear()
	ucode.Run()

	if triggered := rsp.IntBreak.Sleep(10 * time.Millisecond); !triggered {
		t.Fatal("timeout")
	}
}
