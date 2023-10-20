package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"runtime"
	"time"

	"embed"
	"embedded/arch/r4000/systim"

	"n64/fonts/gomono12"
	"n64/framebuffer"
	"n64/rcp"
	"n64/rcp/cpu"
	"n64/rcp/rdp"
	"n64/rcp/serial"
	"n64/rcp/video"

	"github.com/embeddedgo/display/pix"
)

//go:embed images
var images embed.FS

func init() {
	systim.Setup(cpu.ClockSpeed)
}

func main() {
	// isv := carts.ProbeISViewer()
	// if isv != nil {
	// 	rtos.SetSystemWriter(isv.SystemWriter)
	// }

	println("hello n64")

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

	n64logo, err := images.ReadFile("images/n64.png")
	if err != nil {
		println(err.Error())
	}

	img, err := png.Decode(bytes.NewReader(n64logo))
	if err != nil {
		println(err.Error())
	}

	centeredLogo := img.Bounds()
	centeredLogo = centeredLogo.Add(image.Point{
		X: disp.Bounds().Dx()/2 - img.Bounds().Dx()/2,
		Y: disp.Bounds().Dy()/2 - img.Bounds().Dy()/2,
	})
	a.Draw(centeredLogo,
		img, img.Bounds().Min,
		nil, image.Point{},
		draw.Over)

	video.SetFramebuffer(fbAddr)
	video.SetupPAL(video.BBP16)
	time.Sleep(500 * time.Millisecond)

	var titleFont = gomono12.NewFace(gomono12.X0000_00ff())
	tw := a.NewTextWriter(titleFont)
	tw.SetColor(color.White)

	// serial.StartJoybus()

	// lr := disp.Bounds().Max
	// go spinner(disp.NewArea(image.Rect(lr.X-10, lr.Y-15, lr.X, lr.Y-5)))

	n64logosmall, err := images.ReadFile("images/n64_s.png")
	if err != nil {
		println(err.Error())
	}

	imgsmall, err := png.Decode(bytes.NewReader(n64logosmall))
	if err != nil {
		println(err.Error())
	}

	imgsmall16 := framebuffer.NewRGBA16(imgsmall.Bounds())
	draw.Draw(imgsmall16, imgsmall16.Bounds(), imgsmall, image.Point{}, draw.Over)

	logoPos := image.Point{}
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

		logoPos.X = (logoPos.X + 5) % disp.Bounds().Dx()
		logoPos.Y = (logoPos.Y + 2) % disp.Bounds().Dy()

		pixDriver.Draw(imgsmall16.Bounds().Add(logoPos), imgsmall16, image.Point{}, nil, image.Point{}, draw.Over)

		// tw.Pos = image.Point{}

		// tw.Wrap = pix.BreakSpace
		//tw.WriteString("Lorem ipsum dolor sit amet, consectetur adipisici elit, sed eiusmod tempor incidunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquid ex ea commodi consequat. Quis aute iure reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint obcaecat cupiditat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.\n")
		// tw.WriteString(fmt.Sprintf("%06d µs\n", waitVBlankNs/time.Microsecond))
		// println(fmt.Sprintf("%06d µs\n", waitVBlankNs/time.Microsecond))

		//runtime.GC() // run garbace collector while we wait on vblank
		waitVBlankNs = time.Since(start)
		a.Flush()

		// twSW.Pos = image.Point{0, 0}
		// aSW.Fill(aSW.Bounds())
		// twSW.WriteString(fmt.Sprintf("%06d µs\n", waitVBlankNs/time.Microsecond))
		// twSW.WriteString(fmt.Sprintf("%06d µs\n", rdp.DrawDuration/time.Microsecond))
		// rdp.DrawDuration = 0
		// tw.WriteString(fmt.Sprintf("rdp irq cnt: %06d\n", rdp.IrqCnt))

		//runtime.GC() // run garbace collector while we wait on vblank
		video.VBlank.Clear()
		video.VBlank.Sleep(-1)

		video.SetFramebuffer(fbAddr)
	}
}

var waitVBlankNs time.Duration

func printPressedButton(tw *pix.TextWriter) {
	controllers := serial.ControllerStates()
	if controllers[0].Pressed(serial.A) {
		tw.WriteString("A pressed")
	}
	if controllers[0].Released(serial.A) {
		tw.WriteString("A released")
	}

}

func printSysStats(tw *pix.TextWriter) {
	start := time.Now()
	vBlankNs := time.Since(start)
	memstats := &runtime.MemStats{}
	runtime.ReadMemStats(memstats)
	start = time.Now()
	tw.WriteString(fmt.Sprintf("time: %v\n", time.Now().Format(time.DateTime)))
	tw.WriteString(fmt.Sprintf("alloc: %d\n", memstats.Alloc))
	tw.WriteString(fmt.Sprintf("heap objects: %d\n", memstats.HeapObjects))
	tw.WriteString(fmt.Sprintf("sys: %d\n", memstats.Sys))
	tw.WriteString(fmt.Sprintf("stack: %d\n", memstats.StackInuse))
	tw.WriteString(fmt.Sprintf("next GC: %d\n", memstats.NextGC))
	tw.WriteString(fmt.Sprintf("num GC: %d\n", memstats.NumGC))
	tw.WriteString(fmt.Sprintf("GC pause: %d\n", memstats.PauseTotalNs))
	tw.WriteString(fmt.Sprintf("num goroutine: %d\n", runtime.NumGoroutine()))
	tw.WriteString(fmt.Sprintf("VI intr: %d\n", video.VBlankCnt))
	tw.WriteString(fmt.Sprintf("vblank ms: %d\n", vBlankNs/time.Millisecond))
	tw.WriteString(fmt.Sprintf("wait vblank ms: %d\n", waitVBlankNs/time.Millisecond))
	textNs := time.Since(start)
	tw.WriteString(fmt.Sprintf("text ms: %d\n", textNs/time.Millisecond))
}

// Draws a spinning arc.  Used for debugging to see if system has locked up.
func spinner(a *pix.Area) {
	var i int8
	a.SetColor(color.White)
	for {
		i += 32
		a.Arc(image.Point{5, 5}, 4, 4, 5, 5, int32(i)<<24, int32(i-32)<<24, true)
		time.Sleep(250 * time.Millisecond)
	}
}
