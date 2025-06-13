package rspq_test

import (
	"embed"
	"math"
	"slices"
	"structs"
	"testing"
	"unsafe"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/drivers/cartfs"
	"github.com/clktmr/n64/drivers/rspq"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/rsp"
	"github.com/clktmr/n64/rcp/rsp/ucode"
)

var (
	// rsp_vec microcode from libdragon's examples
	// Version: 3feaaadf0 (RSPQ_DEBUG enabled)
	//
	//go:embed rsp_vec.ucode
	_rspVecFiles embed.FS
	rspVecFiles  cartfs.FS = cartfs.Embed(_rspVecFiles)
	rspVecId     uint32
)

const (
	cmdLoad  rspq.Command = 0x0
	cmdStore rspq.Command = 0x1
	cmdTrans rspq.Command = 0x2
)

type vec4 [4]float32
type mat4 [4]vec4

func (a vec4) Mul(b vec4) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] + a[3]*b[3] }
func (m mat4) Row(i int) vec4     { return vec4{m[0][i], m[1][i], m[2][i], m[3][i]} }
func (m mat4) Mul(b vec4) vec4 {
	return vec4{m.Row(0).Mul(b), m.Row(1).Mul(b), m.Row(2).Mul(b), m.Row(3).Mul(b)}
}

// vecSlot is the fixed point vector format required by the ucode. One vecSlot
// store two vec4.
type vecSlot struct {
	_ structs.HostLayout
	i [8]int16
	f [8]uint16
}

type matSlot [2]vecSlot

func (p *matSlot) Set(m *mat4) {
	pvec := (*[2]vecSlot)(p)
	for i := range 2 {
		pvec[i].Set((*[2]vec4)(m[i<<1:]))
	}
}

func (p *vecSlot) Set(vecs *[2]vec4) {
	for i := range 8 {
		fixed := uint32(vecs[i>>2][i&0x3] * (1 << 16))
		p.i[i] = int16(fixed & 0xffff_0000 >> 16)
		p.f[i] = uint16(fixed & 0x0000_ffff)
	}
}

func (p *vecSlot) Get() (vecs [2]vec4) {
	for i := range 8 {
		fixed := int32(p.i[i])<<16 | int32(p.f[i])
		vecs[i>>2][i&0x3] = float32(fixed) / (1 << 16)
	}
	return
}

func vecLoad(slot int, src []vecSlot) {
	debug.Assert(cpu.PhysicalAddressSlice(src)&0xf == 0, "vecSlot alignment")
	const size = int(unsafe.Sizeof(vecSlot{}))
	rspq.Write(cmdLoad|rspq.Command(rspVecId>>24),
		uint32(cpu.PhysicalAddressSlice(src)),
		uint32(((((len(src)*size)-1)&0xFFF)<<16)|((slot*size)&0xFF0)))
}

func vecStore(slot int, dst []vecSlot) {
	debug.Assert(cpu.PhysicalAddressSlice(dst)&0xf == 0, "vecSlot alignment")
	const size = int(unsafe.Sizeof(vecSlot{}))
	rspq.Write(cmdStore|rspq.Command(rspVecId>>24),
		uint32(cpu.PhysicalAddressSlice(dst)),
		uint32(((((len(dst)*size)-1)&0xFFF)<<16)|((slot*size)&0xFF0)))
}

func vecTransform(dst, matrix, vec int) {
	const size = int(unsafe.Sizeof(vecSlot{}))
	rspq.Write(cmdTrans|rspq.Command(rspVecId>>24),
		uint32((dst*size)&0xff0),
		uint32(((matrix*size)&0xff0)<<16|((vec*size)&0xff0)))
}

var vecs = [8]vec4{
	{0.1, -0.2, 0.3, -0.4}, {0.5, -0.6, -0.7, 0.8},
	{1.1, -1.2, 1.3, -1.4}, {1.5, -1.6, -1.7, 1.8},
	{2.1, -2.2, 2.3, -2.4}, {2.5, -2.6, -2.7, 2.8},
	{3.1, -3.2, 3.3, -3.4}, {3.5, -3.6, -3.7, 3.8},
}
var mat = mat4{
	{1.0, 0.0, -0.0, 0.0},
	{0.0, 1.0, -0.0, 0.0},
	{0.0, 0.0, -1.0, 0.0},
	{0.0, 9.0, -0.0, 1.0},
}

func TestVecUCode(t *testing.T) {
	rspq.Reset()

	r, err := rspVecFiles.Open("rsp_vec.ucode")
	if err != nil {
		panic(err)
	}
	uc, err := ucode.Load(r)
	if err != nil {
		panic(err)
	}
	rspVecId = rspq.Register(uc)

	srcPad := cpu.NewPadded[[4]vecSlot, cpu.Align16]()
	src := srcPad.Value()
	src[0].Set((*[2]vec4)(vecs[0:]))
	src[1].Set((*[2]vec4)(vecs[2:]))
	src[2].Set((*[2]vec4)(vecs[4:]))
	src[3].Set((*[2]vec4)(vecs[6:]))
	srcPad.Writeback()
	vecLoad(0, src[:])

	matrix := cpu.NewPadded[matSlot, cpu.Align16]()
	matrix.Value().Set(&mat)
	matrix.Writeback()
	value := matrix.Value()
	vecLoad(30, value[:])

	vecTransform(4, 30, 0)
	vecTransform(5, 30, 1)
	vecTransform(6, 30, 2)
	vecTransform(7, 30, 3)

	dstPad := cpu.NewPadded[[4]vecSlot, cpu.Align16]()
	dst := dstPad.Value()
	dstPad.Invalidate()
	vecStore(4, dst[:])

	for !rsp.Stopped() {
		// wait
	}
	if rspq.Crashed() {
		t.Fatal("rspq crashed")
	}

	almostEqual := func(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

	for i, vec := range vecs {
		vecCPU := mat.Mul(vec)
		vecRSP := dst[i>>1].Get()[i&0x1]
		if !slices.EqualFunc(vecCPU[:], vecRSP[:], almostEqual) {
			t.Logf("vec%d: %v != %v", i, vecCPU, vecRSP)
			t.Fatal("incorrect transform result")
		}
	}
}
