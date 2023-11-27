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
	"embedded/rtos"

	"n64/drivers/carts/everdrive64"
	"n64/fonts/gomono12"
	"n64/framebuffer"
	"n64/rcp"
	"n64/rcp/cpu"
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
	// cart := isviewer.Probe()
	cart := everdrive64.Probe()
	if cart != nil {
		rtos.SetSystemWriter(cart.SystemWriter)
	}

	println("hi")

	fb := framebuffer.NewFramebuffer(video.BBP32)
	fbAddr := fb.Swap()
	rcp.EnableInterrupts(rcp.SerialInterface)
	rcp.EnableInterrupts(rcp.VideoInterface)

	disp := pix.NewDisplay(fb)

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
	video.SetupPAL(video.BBP32)
	time.Sleep(500 * time.Millisecond)

	var titleFont = gomono12.NewFace(gomono12.X0000_00ff())
	tw := a.NewTextWriter(titleFont)
	tw.SetColor(color.White)

	serial.StartJoybus()

	n64logosmall, err := images.ReadFile("images/n64_s.png")
	if err != nil {
		println(err.Error())
	}

	imgsmall, err := png.Decode(bytes.NewReader(n64logosmall))
	if err != nil {
		println(err.Error())
	}

	logoPos := image.Point{}
	for {
		start := time.Now()

		fbAddr = fb.Swap()
		logoPos.X = (logoPos.X + 5) % disp.Bounds().Dx()
		logoPos.Y = (logoPos.Y + 2) % disp.Bounds().Dy()

		// a.Fill(a.Bounds())
		a.Draw(imgsmall.Bounds().Add(logoPos), imgsmall, image.Point{}, nil, image.Point{}, draw.Over)

		tw.Pos = image.Point{}
		tw.Wrap = pix.BreakSpace

		// tw.WriteString("Lorem ipsum dolor sit amet, consectetur adipisici elit, sed eiusmod tempor incidunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquid ex ea commodi consequat. Quis aute iure reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint obcaecat cupiditat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.\n")
		// println("Lorem ipsum dolor sit amet, consectetur adipisici elit, sed eiusmod tempor incidunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquid ex ea commodi consequat. Quis aute iure reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint obcaecat cupiditat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipisici elit, sed eiusmod tempor incidunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquid ex ea commodi consequat. Quis aute iure reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint obcaecat cupiditat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.\n")
		println(fmt.Sprintf("%06d µs", waitVBlankNs/time.Microsecond))
		waitVBlankNs = time.Since(start)
		a.Flush()

		runtime.GC() // run garbace collector while we wait on vblank
		video.VBlank.Clear()
		video.VBlank.Sleep(-1)

		// println("sunt in culpa qui officia deserunt mollit anim id est laborum.\n")
		video.SetFramebuffer(fbAddr)
	}
}

var waitVBlankNs time.Duration
