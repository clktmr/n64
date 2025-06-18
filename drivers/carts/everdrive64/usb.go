package everdrive64

import (
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

const bufferSize = 512

var usbBuf = periph.NewDevice(0x1f80_0400, bufferSize)

type Cart struct {
	buf []byte
}

// Probe returns the [Cart] if an EverDrive64 was detected.
func Probe() *Cart {
	regs().key.Store(0xaa55) // magic key to unlock registers
	switch regs().version.Load() {
	case 0xed64_0008: // EverDrive64 X3
		fallthrough
	case 0x0000_0001: // EverDrive64 X7 without sdcard inserted
		fallthrough
	case 0xed64_0013: // EverDrive64 X7
		cart := &Cart{
			buf: cpu.MakePaddedSlice[byte](bufferSize),
		}
		return cart
	}
	return nil
}

// Write writes data from p to the USB port as raw bytes.
func (v *Cart) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		regs().usbCfgW.Store(writeNop)

		offset := int64(min(len(p), bufferSize))

		var nn int
		nn, err = usbBuf.WriteAt(p[:min(len(p), usbBuf.Size())],
			int64(usbBuf.Size())-offset)
		if err != nil {
			return
		}
		p = p[nn:]

		regs().usbCfgW.Store(write | usbMode(offset))

		for regs().usbCfgR.Load()&act != 0 {
			// wait
		}

		n += nn
	}

	return
}

// Wraps an io.Writer to provide a new io.Writer, which encapsulates each write
// in an UNFLoader packet.
type UNFLoader struct {
	// Can't use an interface here because presumably it causes "malloc
	// during signal" if called via SystemWriter in a syscall.
	w *Cart
}

// Returns a new [UNFLoader]. Use this if you intend to use the USB port for
// logging.
func NewUNFLoader(w *Cart) *UNFLoader {
	// send a single heartbeat to let UNFLoader know which protocol version
	// we are speaking.
	w.Write([]byte{'D', 'M', 'A', '@', 5, 0, 0, 4, 0, 2, 0, 1, 'C', 'M', 'P', 'H'})
	return &UNFLoader{w: w}
}

// Write writes data from p to the underlying writer in UNFLoader packets.
func (v *UNFLoader) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		nn := min(len(p), (1<<24)-1)
		_, err = v.w.Write([]byte{'D', 'M', 'A', '@', 1, byte(nn >> 16), byte(nn >> 8), byte(nn)})
		if err != nil {
			return
		}

		// Align pi addr to 2 byte to ensure use of DMA. This might cause the
		// last byte to be discarded. If that's the case, we prepend it to the
		// footer.
		_, err = v.w.Write(p[:nn&^1])
		if err != nil {
			return
		}

		footer := []byte{p[nn-1], 'C', 'M', 'P', 'H', '0'}
		if nn%2 == 0 {
			footer = footer[1 : len(footer)-1]
		}
		_, err = v.w.Write(footer)
		if err != nil {
			return
		}

		p = p[nn:]
		n += nn
	}

	return
}
