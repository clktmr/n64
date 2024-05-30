package main

import (
	"embedded/arch/r4000/systim"
	"embedded/rtos"
	"io"
	"os"
	"reflect"
	"runtime"
	"syscall"
	"testing"

	"n64/drivers"
	"n64/drivers/carts"
	"n64/rcp/cpu"

	"n64/test/rcp/cpu_test"
	"n64/test/rcp/rsp_test"

	"github.com/embeddedgo/fs/termfs"
)

func init() {
	systim.Setup(cpu.ClockSpeed)
}

func main() {
	var err error
	var syswriter io.Writer

	// Redirect stdout and stderr either to isviewer or everdrive64 usb,
	// using UNFLoader protocol.
	if syswriter = carts.ProbeAll(); syswriter == nil {
		panic("no logging peripheral found")
	}
	rtos.SetSystemWriter(drivers.NewSystemWriter(syswriter))

	console := termfs.NewLight("termfs", nil, syswriter)
	rtos.Mount(console, "/dev/console")
	os.Stdout, err = os.OpenFile("/dev/console", syscall.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	os.Stderr = os.Stdout

	os.Args = append(os.Args, "-test.v")
	testing.Main(
		nil,
		[]testing.InternalTest{
			newInternalTest(cpu_test.TestMakePaddedSlice),
			newInternalTest(rsp_test.TestDMA),
			newInternalTest(rsp_test.TestRun),
		},
		nil, nil,
	)
}

func newInternalTest(testFn func(*testing.T)) testing.InternalTest {
	return testing.InternalTest{
		runtime.FuncForPC(reflect.ValueOf(testFn).Pointer()).Name(),
		testFn,
	}
}
