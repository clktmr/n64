package controller_test

import (
	"io/fs"
	"testing"
	"time"

	"github.com/clktmr/n64/drivers/controller"
	"github.com/clktmr/n64/drivers/controller/pakfs"
	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/serial/joybus"
)

func TestControllerState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	rcp.EnableInterrupts(rcp.SerialInterface)
	t.Cleanup(func() {
		rcp.DisableInterrupts(rcp.SerialInterface)
	})

	t.Log("Press L+R+Start to end the test.")

	for {
		controllers := controller.Poll()
		controller.PollInfo()
		for i, gamepad := range controllers {
			if gamepad.Plugged() {
				t.Log(i, "plugged")
			}
			if gamepad.Unplugged() {
				t.Log(i, "unplugged")
			}
			if gamepad.PakInserted() {
				go func() {
					t.Log(i, gamepad)
					t.Log(i, "pak inserted")
					pak, err := controller.ProbePak(byte(i))
					if err != nil {
						t.Error(err)
					}
					switch pak := pak.(type) {
					case *controller.MemPak:
						t.Log(i, "controller pak detected")
						pfs, err := pakfs.Read(pak)
						if err != nil {
							t.Error(err)
							return
						}
						for _, v := range pfs.Root() {
							info, err := v.Info()
							if err != nil {
								t.Error(err)
							}
							t.Log(fs.FormatFileInfo(info))
						}
					case *controller.RumblePak:
						t.Log(i, "rumble pak detected")
						for range 6 {
							err = pak.Toggle()
							if err != nil {
								t.Error(err)
								return
							}
							time.Sleep(500 * time.Millisecond)
						}
					case *controller.TransferPak:
						t.Log(i, "transfer pak detected")
					default:
						t.Log(i, "no pak type detected")
					}
				}()
			}
			if gamepad.PakRemoved() {
				t.Log(i, "pak removed")
			}
			if gamepad.Pressed() != 0 {
				t.Log(i, "pressed:", gamepad.Pressed())
				if gamepad.Pressed()&joybus.ButtonReset != 0 {
					return
				}
			}
			if gamepad.Released() != 0 {
				t.Log(i, "released:", gamepad.Released())
			}
			if gamepad.DX() != 0 || gamepad.DY() != 0 {
				t.Log(i, "X: ", gamepad.X(), "Y:", gamepad.Y())
			}
		}
	}
}
