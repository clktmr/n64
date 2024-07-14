package controller_test

import (
	"testing"
	"time"

	"github.com/clktmr/n64/drivers/controller"
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
				t.Log(i, "pak inserted")
				pak, err := controller.ProbePak(byte(i))
				if err != nil {
					t.Error(err)
				}
				switch pak := pak.(type) {
				case *controller.RumblePak:
					t.Log(i, "rumble pak detected")
					for range 6 {
						err = pak.Toggle()
						if err != nil {
							t.Error(err)
						}
						time.Sleep(500 * time.Millisecond)
					}
				}
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
