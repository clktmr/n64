// Package test builds a ROM for executing tests on n64.
package main

import (
	"embedded/rtos"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"syscall"
	"testing"

	"github.com/clktmr/n64/drivers/carts"
	"github.com/clktmr/n64/drivers/carts/isviewer"
	"github.com/clktmr/n64/drivers/controller"
	_ "github.com/clktmr/n64/machine"
	"github.com/clktmr/n64/rcp/serial/joybus"
	"github.com/clktmr/n64/test/internal/drivers/cartfs_test"
	"github.com/clktmr/n64/test/internal/drivers/carts/summercart64_test"
	"github.com/clktmr/n64/test/internal/drivers/controller_test"
	"github.com/clktmr/n64/test/internal/drivers/draw_test"
	"github.com/clktmr/n64/test/internal/drivers/rspq_test"
	"github.com/clktmr/n64/test/internal/fonts_test"
	"github.com/clktmr/n64/test/internal/rcp_test"
	"github.com/clktmr/n64/test/internal/rcp_test/cpu_test"
	"github.com/clktmr/n64/test/internal/rcp_test/periph_test"
	"github.com/clktmr/n64/test/internal/rcp_test/rdp_test"
	"github.com/clktmr/n64/test/internal/rcp_test/rsp_test"
	"github.com/clktmr/n64/test/internal/runtime_test"

	"github.com/embeddedgo/fs/termfs"
)

func main() {
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

	os.Args = append(os.Args, "-test.v")
	os.Args = append(os.Args, "-test.bench=.")
	os.Args = append(os.Args, "-test.benchmem")

	print("Hold START to enable interactive test.. ")
	controller.Poll()
	if controller.States[0].Down()&joybus.ButtonStart == 0 {
		os.Args = append(os.Args, "-test.short")
		println("skipping")
	} else {
		println("ok")
	}

	video.Setup(false)
	res := video.NativeResolution()
	res.X /= 2
	fb := texture.NewRGBA32(image.Rectangle{Max: res})
	video.SetFramebuffer(fb)

	testing.Main(
		matchAll,
		[]testing.InternalTest{
			newInternalTest(cartfs_test.TestEmbed),
			newInternalTest(cartfs_test.TestGlobal),
			newInternalTest(cartfs_test.TestDir),
			newInternalTest(cartfs_test.TestHidden),
			newInternalTest(cartfs_test.TestUninitialized),
			newInternalTest(cartfs_test.TestOffset),
			newInternalTest(runtime_test.TestFPUPreemption),
			newInternalTest(runtime_test.TestInterruptPrio),
			newInternalTest(rcp_test.TestReadWriteIO),
			newInternalTest(cpu_test.TestMakePaddedSlice),
			newInternalTest(cpu_test.TestPadSlice),
			newInternalTest(cpu_test.TestMakePaddedSliceAligned),
			newInternalTest(cpu_test.TestUncached),
			newInternalTest(cpu_test.TestPadded),
			newInternalTest(rsp_test.TestDMA),
			newInternalTest(rsp_test.TestRun),
			newInternalTest(rsp_test.TestInterrupt),
			newInternalTest(rdp_test.TestFillRect),
			newInternalTest(draw_test.TestDrawMask),
			newInternalTest(periph_test.TestReaderWriterAt),
			newInternalTest(periph_test.TestConcurrent),
			newInternalTest(summercart64_test.TestUSBRead),
			newInternalTest(summercart64_test.TestSaveStorage),
			newInternalTest(controller_test.TestControllerState),
			newInternalTest(rspq_test.TestCrash),
			newInternalTest(rspq_test.TestWrite),
			newInternalTest(rspq_test.TestDMA),
			newInternalTest(rspq_test.TestVecUCode),
		},
		[]testing.InternalBenchmark{
			newInternalBenchmark(runtime_test.BenchmarkSchedule),
			newInternalBenchmark(fonts_test.BenchmarkGlyphMap),
			newInternalBenchmark(draw_test.BenchmarkDrawText),
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
