package main

import (
	"fmt"
	"image/color"
	"runtime"
	"time"

	"embedded/arch/r4000/systim"

	"n64/fonts/gomono12"
	"n64/framebuffer"
	"n64/rcp"
	"n64/rcp/cpu"
	"n64/rcp/serial"
	"n64/rcp/video"

	"github.com/embeddedgo/display/pix"
)

func init() {
	systim.Setup(cpu.ClockSpeed)
}

func main() {
	fb := framebuffer.NewFramebuffer(video.BBP32)
	fbAddr := fb.Swap()
	rcp.EnableInterrupts(rcp.SerialInterface)
	rcp.EnableInterrupts(rcp.VideoInterface)
	rcp.EnableInterrupts(rcp.DisplayProcessor)

	disp := pix.NewDisplay(fb)

	a := disp.NewArea(disp.Bounds())

	video.SetFramebuffer(fbAddr)
	video.SetupPAL(video.BBP32)

	var titleFont = gomono12.NewFace(gomono12.X0000_00ff())
	tw := a.NewTextWriter(titleFont)
	tw.SetColor(color.White)

	serial.StartJoybus()

	for {
		printPressedButton(tw)

		runtime.GC() // run garbace collector while we wait on vblank
		video.VBlank.Clear()
		video.VBlank.Sleep(-1)
	}
}

var waitVBlankNs time.Duration

func printPressedButton(tw *pix.TextWriter) {
	controllers := serial.ControllerStates()
	if controllers[0].Changed != 0 {
		tw.WriteString(fmt.Sprintln(controllers[0].Down))
	}
}
