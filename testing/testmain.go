// Package testing provides utilities for writing n64 specific tests.
package testing

import (
	"embedded/rtos"
	"fmt"
	"image"
	"io"
	"os"
	"syscall"
	"testing"

	"github.com/clktmr/n64/drivers/carts"
	"github.com/clktmr/n64/drivers/carts/isviewer"
	"github.com/clktmr/n64/drivers/console"
	"github.com/clktmr/n64/drivers/controller"
	_ "github.com/clktmr/n64/machine"
	"github.com/clktmr/n64/rcp/serial/joybus"
	"github.com/clktmr/n64/rcp/texture"
	"github.com/clktmr/n64/rcp/video"

	"github.com/embeddedgo/fs/termfs"
)

// TestMain should be used as TestMain for n64 specific tests.
func TestMain(m *testing.M) {
	var err error
	var cart carts.Cart

	// Redirect stdout and stderr either to cart's logger
	if cart = carts.ProbeAll(); cart == nil {
		panic("no logging peripheral found")
	}

	guiconsole := console.NewConsole()

	fs := termfs.NewLight("termfs", nil, io.MultiWriter(cart, guiconsole))
	rtos.Mount(fs, "/dev/console")
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

	// TODO find a way to pass these from the 'go test' command
	os.Args = append(os.Args, "-test.v")
	os.Args = append(os.Args, "-test.bench=.")
	os.Args = append(os.Args, "-test.benchmem")

	print("Hold START to enable interactive test.. ")
	inputs := [4]controller.Controller{}
	controller.Poll(&inputs)
	if inputs[0].Down()&joybus.ButtonStart == 0 {
		os.Args = append(os.Args, "-test.short")
		println("skipping")
	} else {
		println("ok")
	}

	video.Setup(false)
	res := video.NativeResolution()
	res.X /= 2
	fb := texture.NewFramebuffer(image.Rectangle{Max: res})
	video.SetFramebuffer(fb)

	os.Exit(m.Run())
}
