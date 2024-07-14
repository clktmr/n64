// Package joybus contains functions for creating and parsing of joybus messages
// as they are represented in the n64 PIF, which adds a 2-byte header to each
// message.  It doesn't handle the execution of commands on the bus.
package joybus

import (
	"bytes"
	"errors"
	"strings"

	"github.com/sigurn/crc8"
)

var (
	ErrPIFNoResponse      = errors.New("PIF no response flag")
	ErrPIFInvalidResponse = errors.New("PIF invalid response flag")
	ErrHeader             = errors.New("invalid header")
	ErrChecksum           = errors.New("checksum mismatch")
	ErrDataLength         = errors.New("invalid data length")
)

type Allocator interface {
	Alloc(n int) ([]byte, error)
}

// PIF-NUS special control bytes
const (
	CtrlSkip  byte = 0x00
	CtrlReset byte = 0xfd
	CtrlAbort byte = 0xfe
	CtrlNOP   byte = 0xff
)

func ControlByte(alloc Allocator, ctrl byte) error {
	b, err := alloc.Alloc(1)
	if err != nil {
		return err
	}
	b[0] = ctrl
	return nil
}

const headerLen = 3

const (
	// command bits encoded int the first header byte
	flagSkip  = 0x80
	flagReset = 0x40

	// error bits encoded in the second header byte
	flagNoResponse      = 0x80
	flagInvalidResponse = 0x40

	flagMask = 0xc0
)

// joybus commands
const (
	cmdReset           = "\x01\x03\xff"
	cmdInfo            = "\x01\x03\x00"
	cmdControllerState = "\x01\x04\x01"
	cmdReadPak         = "\x03\x21\x02"
	cmdWritePak        = "\x23\x01\x03"
	cmdReadEEPROM      = "\x02\x08\x04"
	cmdWriteEEPROM     = "\x0a\x01\x05"
	cmdRTCInfo         = "\x01\x03\x06"
	cmdReadRTC         = "\x02\x09\x07"
	cmdWriteRTC        = "\x0a\x01\x08"
)

type Command []byte

func newCommand(alloc Allocator, cmd string) (Command, error) {
	c := Command(cmd)
	n := int(2 + c.txSize() + c.rxSize())
	buf, err := alloc.Alloc(n)
	if err != nil {
		return nil, err
	}
	copy(buf, []byte(c))
	return buf, nil
}

func (c Command) Reset() {
	rx := c.rxData()
	for i := range rx {
		rx[i] = 0x00
	}
}

func (c Command) txData() []byte {
	if c.txSize() == 0 {
		return c[1:1]
	}
	return c[headerLen-1 : headerLen-1+c.txSize()]
}

func (c Command) txSize() uint8 {
	return uint8(c[0])
}

func (c Command) rxData() []byte {
	if c.txSize() == 0 {
		return c[1:1]
	}
	return c[headerLen-1+c.txSize() : headerLen-1+c.txSize()+c.rxSize()]
}

func (c Command) rxSize() uint8 {
	if c.txSize() == 0 {
		return 0
	}
	return uint8(c[1]) &^ 0x80
}

type Device uint16

const (
	Controller Device = 0x0500
	VRU        Device = 0x0001
	Mouse      Device = 0x0200
	Keyboard   Device = 0x0002
	LinkCable  Device = 0x0003
	EEPROM4k   Device = 0x0080
	EEPROM16k  Device = 0x00c0
)

type InfoCommand struct{ Command }

func NewInfoCommand(alloc Allocator) (InfoCommand, error) {
	cmd, err := newCommand(alloc, cmdInfo)
	return InfoCommand{cmd}, err
}

func (c InfoCommand) Info() (dev Device, extra byte) {
	validate(c.Command, cmdInfo)
	rx := c.rxData()
	return Device(uint16(rx[0])<<8 | uint16(rx[1])), rx[2]
}

// Reset command has the same data layout as an Info command
func NewResetCommand(alloc Allocator) (InfoCommand, error) {
	cmd, err := newCommand(alloc, cmdReset)
	return InfoCommand{cmd}, err
}

type ButtonMask uint16

const (
	ButtonA ButtonMask = 1 << (15 - iota)
	ButtonB
	ButtonZ
	ButtonStart
	ButtonDUp
	ButtonDDown
	ButtonDLeft
	ButtonDRight
	ButtonReset // L+R+Start pressed simultaneously
	ButtonUnknown
	ButtonL
	ButtonR
	ButtonCUp
	ButtonCDown
	ButtonCLeft
	ButtonCRight
)

var buttonNames = [...]string{
	"A",
	"B",
	"Z",
	"Start",
	"↑",
	"↓",
	"←",
	"→",
	"Reset",
	"Unknown",
	"L",
	"R",
	"C↑",
	"C↓",
	"C←",
	"C→",
}

func (b ButtonMask) String() string {
	var sb strings.Builder
	for i, v := range buttonNames {
		if b&(1<<(15-i)) != 0 {
			if sb.Len() != 0 {
				sb.WriteString(" + ")
			}
			sb.WriteString(v)
		}
	}
	return sb.String()
}

type ControllerStateCommand struct{ Command }

func NewControllerStateCommand(alloc Allocator) (ControllerStateCommand, error) {
	cmd, err := newCommand(alloc, cmdControllerState)
	return ControllerStateCommand{cmd}, err
}

func (c ControllerStateCommand) State() (b ButtonMask, x int8, y int8) {
	validate(c.Command, cmdControllerState)
	rx := c.rxData()
	return ButtonMask(uint16(rx[0])<<8 | uint16(rx[1])), int8(rx[2]), int8(rx[3])
}

type PakCommand struct{ Command }

func (c PakCommand) SetAddress(addr uint16) {
	// calculate checksum in lower 5 bits
	addr &^= 0x1f
	const lut = "\x01\x1a\x0d\x1c\x0e\x07\x19\x16\x0b\x1f\x15"
	for i, v := range lut {
		if addr&(0x1<<(15-i)) != 0 {
			addr ^= uint16(v)
		}
	}
	tx := c.txData()
	tx[1] = byte(addr >> 8)
	tx[2] = byte(addr)
}

var pakCRC8 = crc8.MakeTable(crc8.Params{0x85, 0x00, false, false, 0x00, 0xF4, "CRC-8 N64 Pak"})

type ReadPakCommand struct{ PakCommand }

func NewReadPakCommand(alloc Allocator) (ReadPakCommand, error) {
	cmd, err := newCommand(alloc, cmdReadPak)
	return ReadPakCommand{PakCommand{cmd}}, err
}

func (c ReadPakCommand) Data() (data []byte, err error) {
	err = validate(c.Command, cmdReadPak)
	if err != nil {
		return
	}

	data = c.rxData()
	data = data[:len(data)-1] // exclude checksum byte

	csum := crc8.Init(pakCRC8)
	csum = crc8.Update(csum, data, pakCRC8)
	csum = crc8.Complete(csum, pakCRC8)

	if csum != c.rxData()[len(data)] {
		err = ErrChecksum
	}

	return
}

type WritePakCommand struct {
	PakCommand
	csum byte
}

func NewWritePakCommand(alloc Allocator) (WritePakCommand, error) {
	cmd, err := newCommand(alloc, cmdWritePak)
	return WritePakCommand{PakCommand{cmd}, 0}, err
}

// len(src) must be match the payload size, i.e. 32 bytes.
func (c *WritePakCommand) SetData(src []byte) (err error) {
	err = validate(c.Command, cmdWritePak)
	if err != nil {
		return
	}

	data := c.txData()
	data = data[3:] // exclude addr

	if len(src) != len(data) {
		return ErrDataLength
	}

	copy(data, src)

	c.csum = crc8.Init(pakCRC8)
	c.csum = crc8.Update(c.csum, data, pakCRC8)
	c.csum = crc8.Complete(c.csum, pakCRC8)

	return
}

func (c WritePakCommand) Result() error {
	err := validate(c.Command, cmdWritePak)
	if err != nil {
		return err
	} else if c.rxData()[0] != c.csum {
		return ErrChecksum
	}
	return nil
}

func validate(c Command, header string) error {
	expected := []byte(header)
	got := [headerLen]byte{}
	copy(got[:], c)

	got[0] &^= flagMask

	if !bytes.Equal(got[:], expected) {
		errFlags := got[1] & flagMask
		got[1] &^= errFlags
		if bytes.Equal(got[:], expected) {
			switch errFlags {
			case flagNoResponse:
				return ErrPIFNoResponse
			case flagInvalidResponse:
				return ErrPIFInvalidResponse
			}
		}
		return ErrHeader
	}

	return nil
}
