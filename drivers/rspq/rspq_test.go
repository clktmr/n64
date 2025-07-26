package rspq_test

import (
	"bytes"
	"testing"

	"github.com/clktmr/n64/drivers/rspq"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/rsp"
	n64testing "github.com/clktmr/n64/testing"
)

func TestMain(m *testing.M) { n64testing.TestMain(m) }

func TestWrite(t *testing.T) {
	rspq.Reset()

	// Write enough commands for multiple buffer swaps
	for i := 0; i < 1000; i++ {
		rspq.Write(rspq.CmdNoop)
		if rspq.Crashed() {
			t.Fatal("rspq crashed")
		}
	}
}

func TestCrash(t *testing.T) {
	rspq.Reset()

	// Cause rspq assertion fail with invalid command
	rspq.Write(rspq.Command(0xde))
	for !rsp.Stopped() {
		// wait
	}

	if !rspq.Crashed() {
		t.Fatal("rspq should have crashed")
	}
}

func TestDMA(t *testing.T) {
	got := cpu.MakePaddedSlice[byte](128)
	expected := cpu.MakePaddedSlice[byte](128)
	rspq.Reset()

	rspq.DMAWrite(got, 256, uint32(len(got)))
	for !rsp.Stopped() {
		// wait
	}

	if rspq.Crashed() {
		t.Fatal("rspq crashed")
	}
	_, err := rsp.DMEM.ReadAt(expected, 256)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("dma data mismatch\n%q\n%q", got, expected)
	}
}
