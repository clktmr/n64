package main

import (
	"embedded/arch/r4000/systim"
	"embedded/rtos"
	"io"
	"os"
	"syscall"
	"testing"

	"n64/drivers"
	"n64/drivers/carts/everdrive64"
	"n64/drivers/carts/isviewer"
	"n64/rcp/cpu"

	"n64/test/rcp/cpu_test"

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
	isv := isviewer.Probe()
	if isv != nil {
		syswriter = isv
	}
	ed64 := everdrive64.Probe()
	if ed64 != nil {
		syswriter = everdrive64.NewUNFLoader(ed64)
	}
	if syswriter == nil {
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
			{"TestMakePaddedSlice", cpu_test.TestMakePaddedSlice},
		},
		nil, nil,
	)
}
