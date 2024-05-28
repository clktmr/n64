package rsp

import (
	"n64/rcp/cpu"
	"runtime"
)

type UCode struct {
	name string

	entry uint32 // initial value of RSP PC register
	code  []byte // instructions copied to IMEM
	data  []byte // data copied to DMEM
}

func NewUCode(name string, entry uint32, code []byte, data []byte) *UCode {
	paddedCode := cpu.MakePaddedSlice(len(code) * 4)
	copy([]byte(paddedCode), code)
	paddedData := cpu.MakePaddedSlice(len(data) * 4)
	copy([]byte(paddedData), data)

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
}

func (ucode *UCode) Run() {
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