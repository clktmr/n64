package rdp

import (
	"image"
	"image/color"
	"image/draw"
	"n64/framebuffer"
	"time"
	"unsafe"
)

type Rdp struct {
	format ImageFormat
	bpp    BitDepth

	bounds image.Rectangle
}

func NewRdp(fb *framebuffer.Framebuffer) *Rdp {
	rdp := &Rdp{
		format: RGBA,
		bounds: fb.Bounds(),
	}
	SetScissor(rdp.bounds, InterlaceNone)

	return rdp
}

func (fb *Rdp) Draw(r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point, op draw.Op) {
	start := time.Now()
	var imgData uintptr
	var format ImageFormat
	var bbp BitDepth
	var bounds image.Rectangle
	var stride int

	// TODO store error for unsupported formats
	switch srcImg := src.(type) {
	case *framebuffer.RGBA16:
		imgData = uintptr(unsafe.Pointer(unsafe.SliceData(srcImg.Pix)))
		format = RGBA
		bbp = BBP16
		bounds = src.Bounds().Sub(src.Bounds().Min)
		stride = srcImg.Stride >> 1
	case *image.NRGBA:
		imgData = uintptr(unsafe.Pointer(unsafe.SliceData(srcImg.Pix)))
		format = RGBA
		bbp = BBP32
		bounds = src.Bounds().Sub(src.Bounds().Min)
		stride = srcImg.Stride >> 2
	case *image.Uniform:
		alphaMask, ok := mask.(*image.Alpha)
		if !ok {
			return
		}
		imgData = uintptr(unsafe.Pointer(unsafe.SliceData(alphaMask.Pix)))

		format = I
		bbp = BBP8
		bounds = mask.Bounds().Sub(mask.Bounds().Min)
		stride = alphaMask.Stride
	default:
		return
	}

	width := tileSize(bounds.Dx())
	// height := uint32(tileSize(bounds.Dy()))

	bounds.Max = bounds.Max.Sub(image.Point{1, 1})

	DrawDuration += time.Since(start)
	SetOtherModes(RGBDitherNone |
		AlphaDitherNone | ForceBlend |
		CycleTypeCopy | AtomicPrimitive | AlphaCompare)
	SetTextureImage(imgData, uint32(stride), format, bbp)

	ts := TileDescriptor{
		Format:   format,
		Size:     bbp,
		Line:     uint16(pixelsToBytes(width, bbp) >> 3),
		TMEMAddr: 0,
		Idx:      0,
		MaskT:    5,
		MaskS:    5,
	}
	// some formats must indicate 16 byte instead of 8 byte texels
	if bbp == BBP32 && (format == RGBA || format == YUV) {
		ts.Line = ts.Line >> 1
	}
	SetTile(ts)
	LoadTile(0, bounds)

	r.Max = r.Max.Sub(image.Point{1, 1})
	TextureRectangle(r, 0)

	// TODO runtime.KeepAlive(imgData) until FullSync?
}

var DrawDuration time.Duration

func (fb *Rdp) Fill(rect image.Rectangle) {
	SetOtherModes(RGBDitherNone |
		AlphaDitherNone | ForceBlend |
		CycleTypeFill | AtomicPrimitive)
	FillRectangle(rect.Bounds())
}

func (fb *Rdp) SetColor(c color.Color) {
	SetFillColor(c)
}

func (fb *Rdp) SetDir(dir int) image.Rectangle {
	return fb.bounds
}

func (fb *Rdp) Flush() {
	Run()
}

func (fb *Rdp) Err(clear bool) error {
	return nil
}

func tileSize(size int) int {
	switch {
	case size <= 4:
		return 4
	case size <= 8:
		return 8
	case size <= 16:
		return 16
	case size <= 32:
		return 32
	case size <= 64:
		return 64
	case size <= 128:
		return 128
	}
	return 256
}
