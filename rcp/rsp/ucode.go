package rsp

import (
	"runtime"

	"github.com/clktmr/n64/rcp/cpu"
)

var currentUCode *UCode

type UCode struct {
	name string

	entry uint32 // initial value of RSP PC register
	code  []byte // instructions copied to IMEM
	data  []byte // data copied to DMEM
}

func NewUCode(name string, entry uint32, code []byte, data []byte) *UCode {
	paddedCode := cpu.CopyPaddedSlice(code)
	paddedData := cpu.CopyPaddedSlice(data)

	return &UCode{
		name:  name,
		entry: entry,
		code:  paddedCode,
		data:  paddedData,
	}
}

func (ucode *UCode) Load() {
	_, err := IMEM.WriteAt(ucode.code, 0x0)
	if err != nil {
		panic(err)
	}
	_, err = DMEM.WriteAt(ucode.data, 0x0)
	if err != nil {
		panic(err)
	}

	currentUCode = ucode
}

func (ucode *UCode) Run() {
	if ucode != currentUCode {
		ucode.Load()
	}

	pc.Store(ucode.entry)
	regs.status.Store(clrHalt | clrBroke)

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
