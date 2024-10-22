package runtime_test

import (
	"embedded/rtos"
	"image"
	"sync/atomic"
	"testing"
	"time"

	_ "unsafe" // for linkname

	"github.com/clktmr/n64/drivers/carts/summercart64"
	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/texture"
	"github.com/clktmr/n64/rcp/video"
)

var blocker atomic.Bool
var sc64 *summercart64.SummerCart64

//go:linkname cartHandler IRQ4_Handler
//go:interrupthandler
func cartHandler() {
	blocker.Store(false)
	sc64.ClearInterrupt()
}

func blockingHandler() {
	video.Handler()
	rcp.DisableInterrupts(rcp.VideoInterface)
	start := time.Now()
	for time.Since(start) < 5*time.Second && blocker.Load() == true {
		// block
	}
}

func TestInterruptPrio(t *testing.T) {
	sc64 = summercart64.Probe()
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	if sc64 == nil {
		t.Skip("requires SummerCart64")
	}

	tests := map[string]struct {
		prio    int
		preempt bool
	}{
		"high":   {rtos.IntPrioHighest, true},
		"normal": {rtos.IntPrioMid, false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rcp.CART.Enable(tc.prio, 0)

			_, prio, err := rcp.CART.Status(0)
			if err != nil {
				t.Error(err)
			}
			if prio != tc.prio {
				t.Fatal("prio not set")
			}

			_, err = sc64.SetConfig(summercart64.CfgButtonMode, summercart64.ButtonModeInterrupt)
			if err != nil {
				t.Fatal(err)
			}
			blocker.Store(true)

			t.Log("Press SummerCart64 button in the next 5 seconds")

			// generate single 5 second blocking low prio interrupt
			start := time.Now()
			video.SetupPAL(texture.NewNRGBA32(image.Rect(0, 0, video.WIDTH, video.HEIGHT)))
			rcp.SetHandler(rcp.VideoInterface, blockingHandler)
			rcp.EnableInterrupts(rcp.VideoInterface)
			t.Cleanup(func() {
				rcp.DisableInterrupts(rcp.VideoInterface)
				rcp.SetHandler(rcp.VideoInterface, video.Handler)
			})

			if blocker.Load() == true {
				t.Fatal("no button press detected")
			}
			if time.Since(start) > 5*time.Second == tc.preempt {
				t.Fatal("priorities not applied")
			}
		})
	}
}
