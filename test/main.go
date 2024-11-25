package main

import (
	"embedded/arch/r4000/systim"
	"embedded/rtos"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"syscall"
	"testing"

	"github.com/clktmr/n64/drivers/carts"
	"github.com/clktmr/n64/drivers/carts/isviewer"
	_ "github.com/clktmr/n64/machine"
	"github.com/clktmr/n64/rcp/cpu"

	"github.com/clktmr/n64/test/drivers/carts/summercart64_test"
	"github.com/clktmr/n64/test/drivers/controller_test"
	"github.com/clktmr/n64/test/drivers/draw_test"
	"github.com/clktmr/n64/test/rcp/cpu_test"
	"github.com/clktmr/n64/test/rcp/periph_test"
	"github.com/clktmr/n64/test/rcp/rdp_test"
	"github.com/clktmr/n64/test/rcp/rsp_test"
	"github.com/clktmr/n64/test/runtime_test"

	"github.com/embeddedgo/fs/termfs"
)

func init() {
	systim.Setup(cpu.ClockSpeed)

	var err error
	var cart carts.Cart

	// Redirect stdout and stderr either to cart's logger
	if cart = carts.ProbeAll(); cart == nil {
		panic("no logging peripheral found")
	}

	console := termfs.NewLight("termfs", nil, cart)
	rtos.Mount(console, "/dev/console")
	os.Stdout, err = os.OpenFile("/dev/console", syscall.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	os.Stderr = os.Stdout

	// The default syswriter is a failsafe ISViewer implementation, which
	// will print panics.
	if isviewer.Probe() == nil {
		fmt.Print("\nWARN: no isviewer found, print() and panic() won't printed\n\n")
	}
}

func main() {
	os.Args = append(os.Args, "-test.v")
	os.Args = append(os.Args, "-test.short")
	os.Args = append(os.Args, "-test.bench=.")
	testing.Main(
		matchAll,
		[]testing.InternalTest{
			newInternalTest(runtime_test.TestFPUPreemption),
			newInternalTest(runtime_test.TestInterruptPrio),
			newInternalTest(cpu_test.TestMakePaddedSlice),
			newInternalTest(cpu_test.TestPadSlice),
			newInternalTest(cpu_test.TestMakePaddedSliceAligned),
			newInternalTest(rsp_test.TestDMA),
			newInternalTest(rsp_test.TestRun),
			newInternalTest(rsp_test.TestInterrupt),
			newInternalTest(rdp_test.TestFillRect),
			newInternalTest(draw_test.TestDrawMask),
			newInternalTest(periph_test.TestReadWriteSeeker),
			newInternalTest(periph_test.TestReadWriteIO),
			newInternalTest(summercart64_test.TestUSBRead),
			newInternalTest(summercart64_test.TestSaveStorage),
			newInternalTest(controller_test.TestControllerState),
		},
		[]testing.InternalBenchmark{
			newInternalBenchmark(runtime_test.BenchmarkSchedule),
			newInternalBenchmark(draw_test.BenchmarkFillScreen),
			newInternalBenchmark(draw_test.BenchmarkTextureRectangle),
		}, nil,
	)
}

func matchAll(_ string, _ string) (bool, error) { return true, nil }

func newInternalTest(testFn func(*testing.T)) testing.InternalTest {
	return testing.InternalTest{
		runtime.FuncForPC(reflect.ValueOf(testFn).Pointer()).Name(),
		testFn,
	}
}

func newInternalBenchmark(testFn func(*testing.B)) testing.InternalBenchmark {
	return testing.InternalBenchmark{
		runtime.FuncForPC(reflect.ValueOf(testFn).Pointer()).Name(),
		testFn,
	}
}
