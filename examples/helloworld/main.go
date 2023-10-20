package main

import (
	"time"

	"embedded/arch/r4000/systim"

	"n64/rcp"
	"n64/rcp/cpu"
	"n64/rcp/video"
)

func init() {
	systim.Setup(cpu.ClockSpeed)
}

func main() {
	rcp.EnableInterrupts(rcp.VideoInterface)
	video.SetupNTSC(video.BBP16)

	for {
		start := time.Now()

		video.VBlank.Clear()
		video.VBlank.Sleep(-1)

		println(time.Since(start) / time.Microsecond)
	}
}
