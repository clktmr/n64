package mixer

import (
	"embed"
	"errors"
	"io"
	"structs"
	"sync"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/drivers/cartfs"
	"github.com/clktmr/n64/drivers/rspq"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/rsp"
	"github.com/clktmr/n64/rcp/rsp/ucode"
)

var (
	// rsp_mixer microcode from libdragon's examples
	// Version: 3feaaadf0 (RSPQ_DEBUG enabled)
	//
	//go:embed rsp_mixer.ucode
	_rspMixerFiles embed.FS
	rspMixerFiles  cartfs.FS = cartfs.Embed(_rspMixerFiles)
	rspMixerId     uint32
)

const (
	cmdExec             rspq.Command = 0x0
	cmdVADPCMDecompress rspq.Command = 0x1
)

const MaxChannels = 32

var (
	mtx sync.Mutex

	sampleRate = 48000
	volume     = float32(1.0)
	settings   = cpu.NewPadded[Settings, cpu.Align16]()
	inputs     = [MaxChannels]struct {
		buf []byte
		src *Source
	}{}
)

type Settings struct {
	_ structs.HostLayout

	lvol, rvol [MaxChannels]int1_15
	channels   [MaxChannels]Channel
}

type Channel struct {
	_ structs.HostLayout

	pos      uint20_12    // Current position within the waveform (in bytes)
	step     uint20_12    // Step between samples (in bytes) to playback at the correct frequency
	len      uint20_12    // Length of the waveform (in bytes)
	loop_len uint20_12    // Length of the loop in the waveform (in bytes)
	ptr      cpu.Addr     // Pointer to the waveform
	flags    channelFlags // Misc flags (see CH_FLAGS_*)
}

type channelFlags uint32

const (
	chBpsShift    channelFlags = (3 << 0) // BPS shift value
	ch16bit       channelFlags = (1 << 2) // Set if the channel is 16 bit
	chStereo      channelFlags = (1 << 3) // Set if the channel is stereo (left)
	chStereoSub   channelFlags = (1 << 4) // The channel is the second half of a stereo (right)
	chStereoAlloc channelFlags = (1 << 5) // The channel has a buffer sized for stereo
)

const bps = 1 // 2 bytes per samples

func Init() {
	inputs = [MaxChannels]struct {
		buf []byte
		src *Source
	}{}

	r, err := rspMixerFiles.Open("rsp_mixer.ucode")
	if err != nil {
		panic(err)
	}
	uc, err := ucode.Load(r)
	if err != nil {
		panic(err)
	}
	rspMixerId = rspq.Register(uc)
}

func SetSampleRate(hz int) {
	sampleRate = hz
}

func exec(volume float32, channels int, dst []byte) {
	debug.Assert(cpu.PhysicalAddressSlice(dst)&0xf == 0, "buffer alignment")
	debug.Assert(cpu.PhysicalAddress(settings.Value())&0xf == 0, "settings alignment")

	settings.Writeback()
	settings.Invalidate()

	rspq.Write(cmdExec|rspq.Command(rspMixerId>>24),
		uint32(uint16(volume*0xffff)),
		uint32((len(dst)>>2)<<16|channels),
		uint32(cpu.PhysicalAddressSlice(dst)),
		uint32(cpu.PhysicalAddress(settings.Value())))
}

func Play(channel int, src *Source) {
	mtx.Lock()
	defer mtx.Unlock()

	s := settings.Value()
	ch := &s.channels[channel]
	s.lvol[channel] = int1_15F(1.0)
	s.rvol[channel] = int1_15F(1.0)
	ch.pos = 0
	ch.step = uint20_12F(float32(src.SampleRate)/float32(sampleRate)) << bps
	ch.flags = channelFlags(bps) | ch16bit

	inputs[channel].src = src
}

func Stop(channel int) {
	inputs[channel].src = nil
}

var Output = &Reader{}

type Reader struct{}

const loopOverread = 64

func resampledSize(inputHz, outputHz, outputLen int) int {
	return (outputLen*inputHz + outputHz - 1) / outputHz
}

func (b *Reader) Read(p []byte) (n int, err error) {
	numChannels := 0
	for i, input := range inputs {
		ch := &settings.Value().channels[i]

		if input.src == nil {
			ch.len = 0
			continue
		}

		inputLen := resampledSize(input.src.SampleRate, sampleRate, len(p)>>1)

		if l, ok := input.src.ReadSeeker.(*looper); ok {
			// Check if we must loop in the rsp
			if l.Size() <= int64(inputLen) {
				input.buf = inputsBuf.Alloc(int(l.Size() + loopOverread))
				_, err := io.ReadFull(input.src, input.buf) // TODO read only once
				if err != nil {
					panic(err)
				}
				ch.len = uint20_12U(uint(l.Size()))
				ch.loop_len = ch.len
				ch.ptr = cpu.PhysicalAddressSlice(input.buf)
				cpu.WritebackSlice(input.buf)
				numChannels = i
				continue
			}
		}

		input.buf = inputsBuf.Alloc(inputLen + cpu.CacheLineSize)

		// seek back unread bytes and align down to cacheline
		seek := -int64(ch.len.Floor() - ch.pos.Floor()&^(cpu.CacheLineSize-1))
		_, err := input.src.Seek(seek, io.SeekCurrent)
		if err != nil {
			panic(err)
		}
		// only keep position relative to current cacheline
		ch.pos &= (cpu.CacheLineSize-1)<<12 | (1<<12 - 1)

		nn, err := io.ReadFull(input.src, input.buf)
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			inputs[i].src = nil
			inputs[i].buf = nil
		} else if err != nil {
			panic(err)
		}

		ch.len = uint20_12U(uint(nn))
		ch.loop_len = 0
		ch.ptr = cpu.PhysicalAddressSlice(input.buf)
		cpu.WritebackSlice(input.buf)

		numChannels = i
	}

	cpu.InvalidateSlice(p)
	exec(volume, numChannels+1, p)
	for !rsp.Stopped() {
		// wait
	}
	if rspq.Crashed() {
		panic("rsp crash")
	}
	inputsBuf.Free()

	return len(p), nil
}

type Source struct {
	io.ReadSeeker
	SampleRate int
}

type buffer struct {
	buf []byte
	pos int
}

var inputsBuf buffer // TODO pinner

func (v *buffer) Alloc(n int) (b []byte) {
	if v.pos+n > len(v.buf) || !cpu.IsPadded(v.buf[v.pos:v.pos+n]) {
		v.buf = cpu.MakePaddedSliceAligned[byte](v.pos+n, 16)
	}

	b = v.buf[v.pos : v.pos+n] // TODO alignUp to fill cacheline
	v.pos = (v.pos + n + cpu.CacheLineSize - 1) &^ (cpu.CacheLineSize - 1)
	return
}

func (v *buffer) Free() {
	v.pos = 0
}

// Loop returns a new io.ReadSeeker that loops the underlying io.ReadSeeker. The
// new stream begins at offset 0 and expands to any positive offset. Note that
// using [io.SeekEnd] for the whence argument in Seek is illegal.
func Loop(rs io.ReadSeeker) io.ReadSeeker {
	current, _ := rs.Seek(0, io.SeekCurrent)
	end, _ := rs.Seek(0, io.SeekEnd)
	start, _ := rs.Seek(0, io.SeekStart)
	rs.Seek(current, io.SeekCurrent)
	return &looper{rs, start, current - start, end - start}
}

type looper struct {
	io.ReadSeeker
	base int64
	off  int64
	n    int64
}

func (v *looper) Read(p []byte) (n int, err error) {
	for n < len(p) && err == nil {
		var nn int
		nn, err = io.ReadFull(v.ReadSeeker, p[n:])
		n += nn
		v.off += int64(nn)
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			_, err = v.ReadSeeker.Seek(0, io.SeekStart)
			if err != nil {
				return
			}
			err = nil
		}
	}
	return
}

var errWhence = errors.New("Seek: invalid whence")
var errOffset = errors.New("Seek: invalid offset")

func (v *looper) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	default:
		return 0, errWhence
	case io.SeekStart:
		// do nothing
	case io.SeekCurrent:
		offset += v.off
	}
	if offset < 0 {
		return 0, errOffset
	}
	v.off = offset
	_, err := v.ReadSeeker.Seek(offset%v.n+v.base, io.SeekStart)
	return offset, err
}

func (v *looper) Size() int64 {
	return v.n
}
