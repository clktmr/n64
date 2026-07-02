package mixer

import (
	"embed"
	"errors"
	"io"
	"structs"
	"sync"
	"sync/atomic"
	"unsafe"

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
	mtx   sync.Mutex
	state = cpu.NewPadded[settings, cpu.Align16]()

	sampleRate = uint(48000)
	volume     = float32(1.0)
	inputs     = [MaxChannels]*Source{}
)

type settings struct {
	_ structs.HostLayout

	lvol, rvol [MaxChannels]int1_15
	channels   [MaxChannels]struct {
		_ structs.HostLayout

		pos      uint20_12    // Current position within the waveform (in bytes)
		step     uint20_12    // Step between samples (in bytes) to playback at the correct frequency
		len      uint20_12    // Length of the waveform (in bytes)
		loop_len uint20_12    // Length of the loop in the waveform (in bytes)
		ptr      cpu.Addr     // Pointer to the waveform
		flags    channelFlags // Misc flags (see CH_FLAGS_*)
	}
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

// Source represents an audio source. It's safe for concurrent use.
type Source struct {
	rs      io.ReadSeeker
	hz, vol atomic.Uint32
}

func NewSource(rs io.ReadSeeker, samplerate uint) *Source {
	s := &Source{rs: rs}
	s.SetSampleRate(samplerate)
	s.SetVolume(1.0, 0.5)
	return s
}

// SetSampleRate sets the playback speed.
func (v *Source) SetSampleRate(hz uint) {
	v.hz.Store(uint32(uint20_12U(hz)))
}

// SetVolume sets the volume and panning of this channel. Both will be clamped
// between zero and one.
func (v *Source) SetVolume(vol, pan float32) {
	vol = min(max(0.0, vol), 1.0)
	pan = min(max(0.0, pan), 1.0)
	lvol := uint32(int1_15F(vol * (1.0 - pan)))
	rvol := uint32(int1_15F(vol * pan))
	v.vol.Store(lvol<<16 | rvol)
}

func (v *Source) step() uint20_12 {
	step := uint20_12(v.hz.Load()).Div(uint20_12U(sampleRate))
	return step << bps
}

func (v *Source) volume() (lvol, rvol int1_15) {
	vol := v.vol.Load()
	return int1_15(vol >> 16), int1_15(vol)
}

func Init() {
	inputs = [MaxChannels]*Source{}

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

// SetSampleRate sets the sample rate of the mixers output. All inputs will be
// resampled to this frequency.
func SetSampleRate(hz uint) {
	mtx.Lock()
	defer mtx.Unlock()
	sampleRate = hz
}

// SetSource connects the audio source to the channel. Set src to nil to disable
// this channel.
func SetSource(channel int, src *Source) {
	// TODO make this non-blocking
	mtx.Lock()
	defer mtx.Unlock()

	s := state.Value()
	ch := &s.channels[channel]
	ch.pos = 0
	ch.len = 0
	ch.flags = channelFlags(bps) | ch16bit

	inputs[channel] = src
}

var Output = &Reader{}

type Reader struct{}

const loopOverread = 64

// exec queues an [cmdExec] command to the rspq.
func exec(volume float32, channels int, dst []byte) {
	debug.Assert(cpu.PhysicalAddressSlice(dst)&0xf == 0, "buffer alignment")
	debug.Assert(cpu.PhysicalAddress(state.Value())&0xf == 0, "settings alignment")

	state.Writeback()
	state.Invalidate()

	rspq.HighpriBegin()
	rspq.Write(cmdExec|rspq.Command(rspMixerId>>24),
		uint32(uint16(volume*0xffff)),
		uint32((len(dst)>>2)<<16|channels),
		uint32(cpu.PhysicalAddressSlice(dst)),
		uint32(cpu.PhysicalAddress(state.Value())))
	rspq.HighpriEnd()
}

func (b *Reader) Read(p []byte) (n int, err error) {
	mtx.Lock()
	defer mtx.Unlock()

	numChannels := 0
	for i, src := range inputs {
		state := state.Value()
		ch := &state.channels[i]

		if src == nil {
			ch.len = 0
			continue
		}

		state.lvol[i], state.rvol[i] = src.volume()
		ch.step = src.step()
		outputLen := uint20_12U(uint(len(p) >> 2))
		inputLen := ch.step.Mul(outputLen).Ceil()

		// seek back unread bytes and align down to cacheline
		seek := -int64(ch.len.Floor() - ch.pos.Floor()&^(cpu.CacheLineSize-1))
		_, err := src.rs.Seek(seek, io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		// only keep position relative to current cacheline
		ch.pos &= (cpu.CacheLineSize-1)<<12 | (1<<12 - 1)

		if l, ok := src.rs.(*looper); ok {
			// Check if we can loop in the rsp
			if l.Size() <= int64(inputLen) {
				buf := inputsBuf.Alloc(int(l.Size() + loopOverread))
				_, err := io.ReadFull(src.rs, buf) // TODO read only once
				if err != nil {
					return 0, err
				}
				pinner.Pin(unsafe.SliceData(buf))
				ch.len = uint20_12U(uint(l.Size()))
				ch.loop_len = ch.len
				ch.ptr = cpu.PhysicalAddressSlice(buf)
				cpu.WritebackSlice(buf)
				numChannels = i
				continue
			}
		}

		buf := inputsBuf.Alloc(int(inputLen + cpu.CacheLineSize))

		nn, err := io.ReadFull(src.rs, buf)
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			inputs[i] = nil
		} else if err != nil {
			return 0, err
		}

		pinner.Pin(unsafe.SliceData(buf))
		ch.len = uint20_12U(uint(nn))
		ch.loop_len = 0
		ch.ptr = cpu.PhysicalAddressSlice(buf)
		cpu.WritebackSlice(buf)

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
	inputsBuf.Reset()
	pinner.Unpin()

	return len(p), nil
}

var (
	inputsBuf buffer
	pinner    cpu.Pinner
)

type buffer []byte

func (p *buffer) Alloc(n int) (b []byte) {
	pos := len(*p)
	if pos+n > cap(*p) || !cpu.IsPadded((*p)[pos:pos+n]) {
		*p = cpu.MakePaddedSliceAligned[byte](pos+n, 16)
	}

	b = (*p)[pos : pos+n]
	pos = (pos + n + cpu.CacheLineSize - 1) &^ (cpu.CacheLineSize - 1)
	*p = (*p)[:pos]
	return
}

func (p *buffer) Reset() {
	*p = (*p)[:0]
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
