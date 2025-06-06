package rspq_test

import (
	"testing"

	"github.com/clktmr/n64/drivers/rspq"
	"github.com/clktmr/n64/rcp/rsp"
)

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
	for !rsp.Halted() {
		// wait
	}

	if !rspq.Crashed() {
		t.Fatal("rspq should have crashed")
	}
}
