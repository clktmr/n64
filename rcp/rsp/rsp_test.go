package rsp_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
	"time"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/rsp"
	"github.com/clktmr/n64/rcp/rsp/ucode"
	n64testing "github.com/clktmr/n64/testing"
)

func TestMain(m *testing.M) { n64testing.TestMain(m) }

func TestDMA(t *testing.T) {
	testdata := cpu.MakePaddedSlice[byte](80)
	for i := range len(testdata) {
		testdata[i] = byte(i)
	}
	_, err := rsp.DMEM.WriteAt(testdata, 0x100)
	if err != nil {
		t.Fatal(err)
	}

	result := cpu.MakePaddedSlice[byte](len(testdata))
	_, err = rsp.DMEM.ReadAt(result, 0x100)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(testdata, result) {
		t.Error("exptected to read same data back that was written")
	}

	shift := int64(0x20)
	_, err = rsp.DMEM.ReadAt(result, 0x100+shift)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(testdata[shift:], result[:len(result)-int(shift)]) {
		t.Error("exptected to read part of same data back that was written")
	}
}

func TestRun(t *testing.T) {
	// Simple program that will swap the first two dwords in DMEM
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
	uc := ucode.NewUCode("testcode", cpu.Addr(rsp.IMEM&0xffffffff), code, data)
	rsp.Load(uc)

	var results = cpu.MakePaddedSlice[uint32](2)
	sr := io.NewSectionReader(rsp.DMEM, 0, 8)
	err := binary.Read(sr, binary.BigEndian, &results)
	if err != nil {
		t.Fatal(err)
	}
	if results[0] != 0xdeadbeef || results[1] != 0xbeeff00d {
		t.Fatal("failed to load ucode data")
	}

	rsp.Resume()

	sr.Seek(0, io.SeekStart)
	err = binary.Read(sr, binary.BigEndian, &results)
	if err != nil {
		t.Fatal(err)
	}
	if results[0] != 0xbeeff00d || results[1] != 0xdeadbeef {
		t.Fatalf("unexpected result after ucode execution: %x", results)
	}
}

func TestInterrupt(t *testing.T) {
	t.Cleanup(func() {
		rsp.SetInterrupt(false)
	})

	rsp.SetInterrupt(true)

	code := []byte{
		0x00, 0x00, 0x00, 0x0d, //break
	}
	data := []byte{}
	uc := ucode.NewUCode("testcode", cpu.Addr(rsp.IMEM&0xffffffff), code, data)
	rsp.Load(uc)

	rsp.Resume()

	if triggered := rsp.IntBreak.Wait(10 * time.Millisecond); !triggered {
		t.Fatal("timeout")
	}
}
