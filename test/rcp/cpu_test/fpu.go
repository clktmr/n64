package cpu_test

import (
	"sync"
	"testing"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/video"
)

func TestFPUPreemption(t *testing.T) {
	rcp.EnableInterrupts(rcp.VideoInterface)
	t.Cleanup(func() {
		rcp.DisableInterrupts(rcp.VideoInterface)
	})
	video.SetupPAL(video.BBP32) // generate some hardware interrupts for preemption

	const numGoroutines = 10
	results := [numGoroutines]float64{}
	wg := sync.WaitGroup{}

	wg.Add(numGoroutines)
	for i := range numGoroutines {
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
