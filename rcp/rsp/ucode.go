package rsp

import (
	"n64/rcp/cpu"
	"runtime"
)

var currentUCode *UCode

type UCode struct {
	name string

	entry uint32 // initial value of RSP PC register
	code  []byte // instructions copied to IMEM
	data  []byte // data copied to DMEM
}

func NewUCode(name string, entry uint32, code []byte, data []byte) *UCode {
	paddedCode := cpu.PaddedSlice(code)
	paddedData := cpu.PaddedSlice(data)

	return &UCode{
		name:  name,
		entry: entry,
		code:  paddedCode,
		data:  paddedData,
	}
}

func (ucode *UCode) Load() {
	DMAStore(0x0, ucode.code, IMEM)
	DMAStore(0x0, ucode.data, DMEM)

	currentUCode = ucode
}

func (ucode *UCode) Run() {
	if ucode != currentUCode {
		ucode.Load()
	}

	pc.Store(ucode.entry)
	regs.status.SetBits(clrHalt | clrBroke)

	// Wait until ucode execution has finished
	for {
		status := regs.status.Load()
		// TODO why do we need to wait for dma?
		if status&halted != 0 && (status&(dmaBusy|dmaFull)) == 0 {
			break
		}
		runtime.Gosched()
	}
}
