package ucode

import (
	"encoding/binary"
	"io"

	"github.com/clktmr/n64/rcp/cpu"
)

type UCode struct {
	Name string

	Entry cpu.Addr // initial value of RSP PC register
	Text  []byte   // instructions copied to IMEM
	Data  []byte   // data copied to DMEM
}

func NewUCode(name string, entry cpu.Addr, text []byte, data []byte) *UCode {
	return &UCode{
		Name:  name,
		Entry: entry,
		Text:  cpu.CopyPaddedSlice(text),
		Data:  cpu.CopyPaddedSlice(data),
	}
}

func Load(r io.Reader) (ucode *UCode, err error) {
	ucode = &UCode{}
	load := func(data any) {
		if err != nil {
			return
		}
		err = binary.Read(r, binary.BigEndian, data)
	}
	var size uint32
	load(&size)
	name := make([]byte, size)
	load(&name)
	ucode.Name = string(name)
	load(&ucode.Entry)

	load(&size)
	ucode.Text = cpu.MakePaddedSlice[byte](int(size))
	load(&ucode.Text)

	load(&size)
	ucode.Data = cpu.MakePaddedSlice[byte](int(size))
	load(&ucode.Data)
	return
}

func (ucode *UCode) Store(w io.Writer) (err error) {
	store := func(data any) {
		if err != nil {
			return
		}
		err = binary.Write(w, binary.BigEndian, data)
	}
	store(uint32(len(ucode.Name)))
	store([]byte(ucode.Name))
	store(ucode.Entry)
	store(uint32(len(ucode.Text)))
	store(ucode.Text)
	store(uint32(len(ucode.Data)))
	store(ucode.Data)
	return
}
