package main

import (
	"embedded/arch/r4000/systim"
	"embedded/rtos"
	"os"
	"reflect"
	"runtime"
	"syscall"
	"testing"

	"github.com/clktmr/n64/drivers"
	"github.com/clktmr/n64/drivers/carts"
	_ "github.com/clktmr/n64/machine"
	"github.com/clktmr/n64/rcp/cpu"

	"github.com/clktmr/n64/test/drivers/carts/summercart64_test"
	"github.com/clktmr/n64/test/rcp/cpu_test"
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
	syswriter := drivers.NewSystemWriter(cart)
	rtos.SetSystemWriter(syswriter)

	console := termfs.NewLight("termfs", nil, syswriter)
	rtos.Mount(console, "/dev/console")
	os.Stdout, err = os.OpenFile("/dev/console", syscall.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	os.Stderr = os.Stdout
}

func main() {
	os.Args = append(os.Args, "-test.v")
	os.Args = append(os.Args, "-test.bench=.")
	testing.Main(
		matchAll,
		[]testing.InternalTest{
			newInternalTest(runtime_test.TestFPUPreemption),
			newInternalTest(runtime_test.TestInterruptPrio),
			newInternalTest(cpu_test.TestMakePaddedSlice),
			newInternalTest(cpu_test.TestMakePaddedSliceAligned),
			newInternalTest(rsp_test.TestDMA),
			newInternalTest(rsp_test.TestRun),
			newInternalTest(rsp_test.TestInterrupt),
			newInternalTest(rdp_test.TestFillRect),
			newInternalTest(rdp_test.TestDraw),
			newInternalTest(summercart64_test.TestUSBRead),
		},
		[]testing.InternalBenchmark{
			newInternalBenchmark(runtime_test.BenchmarkSchedule),
			newInternalBenchmark(rdp_test.BenchmarkFillScreen),
			newInternalBenchmark(rdp_test.BenchmarkTextureRectangle),
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
