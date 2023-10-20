package main

import (
	"blinky/framebuffer"
	"bytes"
	"embed"
	"embedded/arch/r4000/systim"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"runtime"
	"time"
	"unsafe"

	"github.com/embeddedgo/display/font/subfont/font9/vga"
	"github.com/embeddedgo/display/pix"
)

//go:embed images
var images embed.FS

func init() {
	systim.Setup(93.75e6)
}

func initDAC(fb_p unsafe.Pointer) {
	p := unsafe.Pointer(uintptr(0xA440_0000))
	*((*uint32)(p)) = 0x3
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = uint32(uintptr(fb_p))
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = 320
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = 0x200
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = 352
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = 0x3E5_2239
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = 525
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = (0 << 16) | 3093
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = (3093 << 16) | 3093
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = (108 << 16) | 748
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = (37 << 16) | 511
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = (14 << 16) | 516
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = (0 << 16) | 512
	p = unsafe.Add(p, 4)
	*((*uint32)(p)) = (0 << 16) | 1024
}

func main() {
	println("hello n64")
	fb_p := unsafe.Pointer(&framebuffer.Buf)
	initDAC(fb_p)

	fb := framebuffer.NewFramebuffer()
	disp := pix.NewDisplay(fb)

	a := disp.NewArea(disp.Bounds())
	a.SetColor(color.RGBA{100, 100, 120, 255})
	a.Fill(disp.Bounds())

	n64logo, err := images.ReadFile("images/n64.png")
	if err != nil {
		println(err.Error())
	}

	img, err := png.Decode(bytes.NewReader(n64logo))
	if err != nil {
		println(err.Error())
	}

	a.Draw(disp.Bounds().Inset(50),
		img, img.Bounds().Min,
		nil, image.Point{},
		draw.Over)

	time.Sleep(2 * time.Second)

	var titleFont = vga.NewFace(
		vga.X0000_007f,
	)
	tw := a.NewTextWriter(titleFont)
	tw.SetColor(color.White)

	// go drawStuff(a)

	for {
		a.Fill(disp.Bounds())
		memstats := &runtime.MemStats{}
		runtime.ReadMemStats(memstats)
		tw.WriteString(fmt.Sprintf("alloc: %d\n", memstats.Alloc))
		tw.WriteString(fmt.Sprintf("heapalloc: %d\n", memstats.HeapAlloc))
		tw.WriteString(fmt.Sprintf("stack: %d\n", memstats.StackInuse))
		tw.WriteString(fmt.Sprintf("next GC: %d\n", memstats.NextGC))
		tw.WriteString(fmt.Sprintf("num GC: %d\n", memstats.NumGC))
		tw.WriteString(fmt.Sprintf("GC pause: %d\n", memstats.PauseTotalNs))
		time.Sleep(100 * time.Millisecond)
		tw.Pos = image.Point{0, 0}
	}
}

func drawStuff(a *pix.Area) {
	pos := 0
	c := color.RGBA{0xff, 0, 0, 0xff}
	for {
		c = color.RGBA{c.R + 34, c.G + 101, c.A + 10, 0xff}
		a.SetColor(c)
		a.Fill(image.Rect(40, pos, 100, 100))
		pos = (pos + 1) % 240
		time.Sleep(50 * time.Millisecond)
	}
}

var i int

//go:interrupthandler
func RCP_Handler() {
	i += 1
}
