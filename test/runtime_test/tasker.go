package runtime_test

import (
	"sync"
	"testing"
	"time"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/rdp"
)

var f float32

func fpuClobber() {
	rcp.ClearDPIntr()
	f += 0.33
}

func TestFPUPreemption(t *testing.T) {
	rdpHandler := rcp.Handler(rcp.IntrRDP)
	rcp.SetHandler(rcp.IntrRDP, fpuClobber)
	t.Cleanup(func() {
		rcp.SetHandler(rcp.IntrRDP, rdpHandler)
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

	// generate some fpu preemptions using hardware interrupts
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				rdp.RDP.Push(rdp.SyncFull)
				time.Sleep(time.Millisecond)
			}
		}
	}()

	wg.Wait()
	done <- struct{}{}

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
