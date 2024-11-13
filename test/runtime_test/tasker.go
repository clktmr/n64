package runtime_test

import (
	"image"
	"sync"
	"testing"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/texture"
	"github.com/clktmr/n64/rcp/video"
)

var f float32

func fpuClobber() {
	video.Handler()
	f += 0.33
}

func TestFPUPreemption(t *testing.T) {
	rcp.DisableInterrupts(rcp.VideoInterface)
	rcp.SetHandler(rcp.VideoInterface, fpuClobber)
	rcp.EnableInterrupts(rcp.VideoInterface)

	// generate some fpu using hardware interrupts
	video.SetupPAL(false, false)
	video.SetFramebuffer(texture.NewNRGBA32(image.Rect(0, 0, 320, 240)))
	t.Cleanup(func() {
		video.SetFramebuffer(nil)
		rcp.DisableInterrupts(rcp.VideoInterface)
		rcp.SetHandler(rcp.VideoInterface, video.Handler)
		rcp.EnableInterrupts(rcp.VideoInterface)
	})

	const numGoroutines = 10
	results := [numGoroutines]float64{}
	wg := sync.WaitGroup{}

	wg.Add(numGoroutines)
	for i := range numGoroutines {
		i := i
		go func(f float64) {
			for range 1000000 {
				f += 0.125
			}
			results[i] = f
			wg.Done()
		}(float64(i))
	}

	wg.Wait()

	for i, v := range results {
		expected := float64(i) + 125000.0
		if v != expected {
			t.Errorf("unexpected result: %v != %v", v, expected)
		}
	}
}

func BenchmarkSchedule(b *testing.B) {
	start := make(chan bool)
	stop := make(chan bool)

	go func() {
		for <-start {
			stop <- true
		}
		stop <- false
	}()

	for i := 0; i < b.N; i++ {
		start <- true
		<-stop
	}
	start <- false
	<-stop
}
