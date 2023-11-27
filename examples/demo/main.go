package main

import (
	"embedded/arch/r4000/systim"
	"math/rand"
	"runtime"

	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"n64/fonts/gomono12"
	"n64/framebuffer"
	"n64/rcp"
	"n64/rcp/cpu"
	"n64/rcp/rdp"
	"n64/rcp/video"
	"strings"
	"time"

	"github.com/embeddedgo/display/pix"
)

//go:embed gopher.png
var gopherPng string

func init() {
	systim.Setup(cpu.ClockSpeed)
}

func main() {
	// isv := isviewer.Probe()
	// if isv != nil {
	// 	rtos.SetSystemWriter(isv.SystemWriter)
	// }

	// println("hello n64")

	fb := framebuffer.NewFramebuffer(video.BBP16)
	fbAddr := fb.Swap()
	rcp.EnableInterrupts(rcp.SerialInterface)
	rcp.EnableInterrupts(rcp.VideoInterface)
	rcp.EnableInterrupts(rcp.DisplayProcessor)

	rdp.Start()
	rdp.SetColorImage(fbAddr, framebuffer.WIDTH, rdp.RGBA, rdp.BBP16)

	pixDriver := rdp.NewRdp(fb)
	disp := pix.NewDisplay(pixDriver)

	a := disp.NewArea(disp.Bounds())

	video.SetFramebuffer(fbAddr)
	video.SetupPAL(video.BBP16)

	var titleFont = gomono12.NewFace(gomono12.X0000_00ff())
	tw := a.NewTextWriter(titleFont)
	tw.SetColor(color.White)

	gopherImg, err := png.Decode(strings.NewReader(gopherPng))
	if err != nil {
		println(err.Error())
	}

	gopherImg16bpp := framebuffer.NewRGBA16(gopherImg.Bounds())
	draw.Draw(gopherImg16bpp, gopherImg16bpp.Bounds(), gopherImg, image.Point{}, draw.Over)

	logoPos := [32]image.Point{}
	logoMov := [32]image.Point{}
	for i := range logoMov {
		logoMov[i].X = rand.Intn(5) - 3
		logoMov[i].Y = rand.Intn(5) - 3
	}

	for {
		start := time.Now()

		fbAddr = fb.Swap()
		rdp.SetColorImage(fbAddr, framebuffer.WIDTH, rdp.RGBA, rdp.BBP16)

		rdp.Sync(rdp.Pipe)
		rdp.SetScissor(disp.Bounds(), rdp.InterlaceNone)
		rdp.SetFillColor(color.RGBA{R: 0, G: 0x37, B: 0x77, A: 0xff})
		rdp.SetOtherModes(rdp.RGBDitherNone |
			rdp.AlphaDitherNone | rdp.ForceBlend |
			rdp.CycleTypeFill | rdp.AtomicPrimitive)
		rdp.FillRectangle(disp.Bounds())

		for i := range logoPos {
			logoPos[i].X = (logoPos[i].X + (rand.Intn(9) - 4)) % (disp.Bounds().Dx() - 32)
			logoPos[i].Y = (logoPos[i].Y + (rand.Intn(9) - 4)) % (disp.Bounds().Dy() - 32)

			if logoPos[i].X < 0 {
				logoPos[i].X += disp.Bounds().Dx() - 32
			}
			if logoPos[i].Y < 0 {
				logoPos[i].Y += disp.Bounds().Dy() - 32
			}

			pixDriver.Draw(gopherImg16bpp.Bounds().Add(logoPos[i]), gopherImg16bpp, image.Point{}, nil, image.Point{}, draw.Over)
		}

		a.Flush()
		println(time.Since(start) / time.Microsecond)

		runtime.GC() // run garbace collector while we wait on vblank
		video.VBlank.Clear()
		video.VBlank.Sleep(-1)

		video.SetFramebuffer(fbAddr)
	}
}
