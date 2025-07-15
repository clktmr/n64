// Package audio provides analog audio output from a buffer.
//
// While the sample rate is configurable, only signed 16-bit stereo PCM is
// supported by the hardware.
//
// There is no mixing in hardware, i.e. only a single buffer can be played back
// at a time. If mixing multiple channels is required, it's usually done via a
// RSP microcode, e.g. libdragon's rspq_mixer.
package audio

import (
	"embedded/mmio"
	"embedded/rtos"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/clktmr/n64/machine"
	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
)

func regs() *registers { return cpu.MMIO[registers](0x0450_0000) }

type registers struct {
	dramAddr mmio.R32[cpu.Addr] // dma address
	length   mmio.U32           // dma length, read returns remaining bytes
	control  mmio.U32           // write 0x1 to start dma. write only
	status   mmio.R32[statusFlags]
	dacRate  mmio.U32 // equals (videoRate/sampleRate)-1, write-only
	bitRate  mmio.U32
}

type statusFlags uint32

// Read access to status register
const (
	dmaFull    statusFlags = 1 << 31 // transfer pending
	dmaBusy    statusFlags = 1 << 30 // transfer in progress
	dmaEnabled statusFlags = 1 << 25 // reflects control register
)

const (
	dacRateNTSC = 48681818
	dacRatePAL  = 49656530
	dacRateMPAL = 48628322
)

const dmaAlign = 8

const maxBufLen = 1 << 17

var ErrStop = errors.New("playback stopped")

var (
	// pending and write buffer; read buffer is always previous to pending
	pending rcp.IntrInput[[]byte]
	writing int
	bufs    [3][]byte
	bufCap  int
	pinner  cpu.Pinner

	stopMtx sync.Mutex // dma must not be enabled while held
)

func init() {
	Start(48000)
	rcp.SetHandler(rcp.IntrAudio, handler)
	rcp.EnableInterrupts(rcp.IntrAudio)
}

// Start begins playback from [Buffer]. The first parameter sets the samplerate,
// which specifies how many samples per second are played back. Per default it's
// set to 48000 Hz.
func Start(hz int) {
	var clockrate int
	switch machine.VideoType {
	case machine.VideoNTSC:
		clockrate = dacRateNTSC
	case machine.VideoPAL:
		clockrate = dacRatePAL
	case machine.VideoMPAL:
		clockrate = dacRateMPAL

	}

	dacrate := ((2 * clockrate / hz) + 1) / 2
	bitrate := min(dacrate/66, 16)

	// Calculate actual sample rate back from dacrate
	hz = (2 * clockrate) / ((dacrate * 2) - 1)

	const buffersPerSecond = 25
	samplesPerBuffer := (hz / buffersPerSecond) &^ 7
	bufCap = samplesPerBuffer * 2 * 2

	Stop()

	regs().dacRate.Store(uint32(dacrate) - 1)
	regs().bitRate.Store(uint32(bitrate) - 1)

	for i := range bufs {
		pinner.Unpin()
		bufs[i] = newBuffer(bufCap)
		cpu.PinSlice(&pinner, bufs[i])
	}

	stopMtx.Unlock()
}

func Stop() {
	if !stopMtx.TryLock() {
		return // already stopped
	}

	dmaStop.Wait(0) // clear cond
	if regs().status.LoadBits(dmaEnabled) != 0 {
		if !dmaStop.Wait(1 * time.Second) {
			panic("audio stop timeout")
		}
	}
}

// Buffer is the global audio buffer. It implements [io.Writer] and
// [io.ReadFrom]. Data written to Buffer must hold 16-bit stereo samples.
// Playback of audio will not start until enough samples were written for one
// frame of audio, or a call to [Flush].
var Buffer = &Writer{}

// Writer is the type of the global audio buffer. It's not intended for manual
// creation.
type Writer struct{}

// Write implements [io.Writer].
func (b *Writer) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		buf := bufs[writing]

		nn := copy(buf[len(buf):bufCap], p)
		n += nn
		p = p[nn:]
		buf = buf[:len(buf)+nn]
		bufs[writing] = buf

		if len(buf) == bufCap {
			err = b.Flush()
			if err != nil {
				return n, err
			}
		}
	}

	return
}

// ReadFrom implements [io.ReaderFrom].
func (b *Writer) ReadFrom(r io.Reader) (n int64, err error) {
	for {
		buf := bufs[writing]

		nn, err := r.Read(buf[len(buf):bufCap])
		n += int64(nn)
		buf = buf[:len(buf)+nn]
		bufs[writing] = buf

		if err == io.EOF {
			return n, nil
		} else if err != nil {
			return n, err
		}

		if len(buf) == bufCap {
			err = b.Flush()
			if err != nil {
				return n, err
			}
		}
	}
}

// Flush blocks until all bytes written to the buffer were passed to the audio
// DAC for playback.
func (b *Writer) Flush() error {
	for {
		if _, consumed := pending.Read(); consumed {
			break
		}
		if !dmaStart.Wait(1 * time.Second) {
			panic("audio dma timeout")
		}
	}

	if stopMtx.TryLock() {
		cpu.WritebackSlice(bufs[writing])
		pending.Put(bufs[writing])
		stopMtx.Unlock()
	} else {
		return ErrStop
	}

	writing = (writing + 1) % len(bufs)
	bufs[writing] = bufs[writing][:0]

	if regs().status.LoadBits(dmaEnabled) == 0 {
		handler()
	}

	return nil
}

// Len returns the number of bytes that the buffer can hold before it will be
// flushed. This always equals to 1/25 second of playback.
func (b *Writer) Len() int {
	return bufCap
}

var dmaStart, dmaStop rtos.Cond

// handler is executed when the pending DMA starts.
//
//go:nosplit
//go:nowritebarrierrec
func handler() {
	if rtos.HandlerMode() {
		regs().status.Store(0) // clear interrupt
		dmaStart.Signal()
	} else {
		rcp.DisableInterrupts(rcp.IntrAudio)
		defer rcp.EnableInterrupts(rcp.IntrAudio)
	}

	buf, updated := pending.Get()
	if !updated {
		// No data was written, disable playback after dma finished
		if regs().status.LoadBits(dmaBusy) == 0 {
			regs().control.Store(0)
			dmaStop.Signal()
		} else {
			regs().dramAddr.Store(0)
			regs().length.Store(0)
		}
		return
	}

	regs().dramAddr.Store(cpu.PhysicalAddressSlice(buf))
	regs().length.Store(uint32(len(buf) - 1))
	regs().control.Store(1)
}

// newBuffer returns a newly allocated empty buffer with at least capacity n,
// which can be used to play audio.
func newBuffer(n int) []byte {
	buf := cpu.MakePaddedSliceAligned[byte](n+cpu.CacheLineSize, dmaAlign)

	// Workaround DMA hardware bug: End must not be aligned to 0x2000.
	if cpu.PhysicalAddressSlice(buf[len(buf):])&0x1fff == 0 {
		buf = buf[:len(buf)-cpu.CacheLineSize]
	} else {
		buf = buf[cpu.CacheLineSize:]
	}

	return buf[:0]
}
