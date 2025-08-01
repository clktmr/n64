package rcp_test

import (
	"embedded/rtos"
	"sync/atomic"
	"testing"
	"time"

	_ "unsafe" // for linkname

	"github.com/clktmr/n64/drivers/carts/summercart64"
	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/rdp"
)

var blocker atomic.Bool
var sc64 *summercart64.Cart
var note rtos.Cond

//go:linkname cartHandler IRQ4_Handler
//go:interrupthandler
func cartHandler() {
	if sc64 == nil {
		panic("sc64 not initialized")
	}
	blocker.Store(false)
	sc64.ClearInterrupt()
}

//go:nosplit
//go:nowritebarrierrec
func blockingHandler() {
	rcp.ClearDPIntr()
	start := time.Now()
	for time.Since(start) < 5*time.Second && blocker.Load() == true {
		// block
	}
	note.Signal()
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
			rcp.IrqCart.Enable(tc.prio, 0)

			_, prio, err := rcp.IrqCart.Status(0)
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

			// generate single 5 second blocking low prio interrupt
			t.Log("Press SummerCart64 button in the next 5 seconds")

			rdpHandler := rcp.Handler(rcp.IntrRDP)
			rcp.SetHandler(rcp.IntrRDP, blockingHandler)

			start := time.Now()
			rdp.RDP.Push(rdp.SyncFull)
			note.Wait(5 * time.Second)

			_, err = sc64.SetConfig(summercart64.CfgButtonMode, summercart64.ButtonModeDisabled)
			if err != nil {
				t.Fatal(err)
			}
			rcp.SetHandler(rcp.IntrRDP, rdpHandler)

			if blocker.Load() == true {
				t.Fatal("no button press detected")
			}
			if time.Since(start) > 5*time.Second == tc.preempt {
				t.Fatal("priorities not applied")
			}
		})
	}
}
