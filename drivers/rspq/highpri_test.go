package rspq

import (
	"testing"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/rsp"
)

func TestHighpri(t *testing.T) {
	Reset()

	// Start the low-priority queue running in an infinite loop of Noops
	Write(CmdNoop)
	Write(CmdNoop)
	loopTarget := cpu.PhysicalAddressSlice(lowpri.buffers[lowpri.bufIdx])
	Write(CmdJump, uint32(loopTarget))

	if rsp.Stopped() {
		t.Fatal("RSP should be running")
	}

	// Schedule a high-priority command to set sigSyncpoint
	HighpriBegin()
	Write(CmdWriteStatus, sigSyncpoint.SetMask())
	HighpriEnd()

	HighpriSync()

	if rsp.Signals()&sigSyncpoint == 0 {
		t.Fatal("high-priority command did not execute")
	}
}
