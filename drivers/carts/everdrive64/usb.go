package everdrive64

import (
	"io"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

const bufferSize = 512

var usbBuf = periph.NewDevice(0x1f80_0400, bufferSize)

type Cart struct {
	buf []byte
}

func Probe() *Cart {
	regs.key.Store(0xaa55) // magic key to unlock registers
	switch regs.version.Load() {
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

func (v *Cart) Write(p []byte) (n int, err error) {
	for err = io.ErrShortWrite; err == io.ErrShortWrite; {
		regs.usbCfgW.Store(writeNop)

		var offset int64 = int64(min(len(p), bufferSize))
		offset, err = usbBuf.Seek(-offset, io.SeekEnd)
		if err != nil {
			return
		}

		var written int
		written, err = usbBuf.Write(p)
		n += written
		p = p[written:]
		usbBuf.Flush()

		regs.usbCfgW.Store(write | usbMode(offset))

		for regs.usbCfgR.Load()&act != 0 {
			// wait
		}
	}

	return
}

// Wraps an io.Writer to provide a new io.Writer, which sends encapsulates each
// write in an UNFLoader packet.
type UNFLoader struct {
	// Can't use an interface here because presumably it causes "malloc
	// during signal" if called via SystemWriter in a syscall.
	// TODO try using generics to make this available for other carts
	w *Cart
}

func NewUNFLoader(w *Cart) *UNFLoader {
	// send a single heartbeat to let UNFLoader know which protocol version
	// we are speaking.
	w.Write([]byte{'D', 'M', 'A', '@', 5, 0, 0, 4, 0, 2, 0, 1, 'C', 'M', 'P', 'H'})
	return &UNFLoader{w: w}
}

func (v *UNFLoader) Write(p []byte) (n int, err error) {
	n = len(p)
	if n >= 1<<24 {
		n = 1 << 24
		err = io.ErrShortWrite
	}
	v.w.Write([]byte{'D', 'M', 'A', '@', 1, byte(n >> 16), byte(n >> 8), byte(n)})

	// Align pi addr to 2 byte to ensure use of DMA.  This might cause the
	// last byte to be discarded.  If that's the case, we prepend it to the
	// footer.
	s, err1 := v.w.Write(p[:n&^1])
	if err1 != nil {
		return s, err1
	}

	footer := []byte{p[len(p)-1], 'C', 'M', 'P', 'H', '0'}
	if n%2 == 0 {
		footer = footer[1 : len(footer)-1]
	}
	v.w.Write(footer)

	return
}
