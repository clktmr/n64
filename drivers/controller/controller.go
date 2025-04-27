// Package controller implements helpers for reading the states of the gamepads
// and their pak accessories.
package controller

import (
	"github.com/clktmr/n64/rcp/serial/joybus"
)

// Port represents the state of a joybus port as returned by
// [joybus.InfoCommand].
type Port struct {
	current, last struct {
		device joybus.Device
		flags  byte
	}

	number uint8
	err    error
}

// Nr returns the joybus port's number from 0 to 3.
func (p *Port) Nr() uint8 {
	return p.number
}

const (
	pakInserted    = 0x01
	pakNotInserted = 0x02
)

// Controller represents the state of a connected controller port as returned by
// [joybus.ControllerStateCommand].
type Controller struct {
	// The joybus port the controller is connected to.
	Port

	current, last struct {
		down  joybus.ButtonMask
		xAxis int8
		yAxis int8
	}

	err error
}

// Down reports which buttons were pressed during the last call to [Poll].
func (c *Controller) Down() joybus.ButtonMask {
	return c.current.down
}

// Changed reports which buttons changed state between the last two calls to
// [Poll].
func (c *Controller) Changed() joybus.ButtonMask {
	return c.current.down ^ c.last.down
}

// Pressed reports which buttons were pressed between the last two calls to
// [Poll].
func (c *Controller) Pressed() joybus.ButtonMask {
	return c.Changed() & c.current.down
}

// Released reports which buttons were released between the last two calls to
// [Poll].
func (c *Controller) Released() joybus.ButtonMask {
	return c.Changed() & c.last.down
}

// X returns the raw horizontal position of the analog stick. It typically
// ranges from -85 to 85.
func (c *Controller) X() int8 {
	return c.current.xAxis
}

// Y returns the raw vertical position of the analog stick. It typically ranges
// from -85 to 85.
func (c *Controller) Y() int8 {
	return c.current.yAxis
}

// DX returns the change of the analog stick's horizontal position between the
// last two calls to [Poll].
func (c *Controller) DX() int8 {
	return c.current.xAxis - c.last.xAxis
}

// DX returns the change of the analog stick's vertical position between the
// last two calls to [Poll].
func (c *Controller) DY() int8 {
	return c.current.yAxis - c.last.yAxis
}

// Present reports whether a controller is connected to the port. It will return
// false if no device is connected or
func (c *Controller) Present() bool {
	return c.Port.current.device == joybus.Controller
}

// Plugged reports if a controller was plugged into the port between the last
// two calls to [Poll].
func (c *Controller) Plugged() bool {
	return c.Port.current.device == joybus.Controller &&
		c.Port.last.device != joybus.Controller
}

// Unplugged reports if a controller was unplugged from the port between the
// last two calls to [Poll].
func (c *Controller) Unplugged() bool {
	return c.Port.current.device != joybus.Controller &&
		c.Port.last.device == joybus.Controller
}

// PakInserted reports if a pak accessory was inserted into the controller
// between the last two calls to [Poll].
func (c *Controller) PakInserted() bool {
	return c.Port.current.flags&pakInserted != 0 &&
		c.Port.last.flags&pakInserted == 0
}

// PakRemoved reports if a pak accessory was removed from the controller between
// the last two calls to [Poll].
func (c *Controller) PakRemoved() bool {
	return c.Port.current.flags&pakNotInserted != 0 &&
		c.Port.last.flags&pakInserted != 0
}
