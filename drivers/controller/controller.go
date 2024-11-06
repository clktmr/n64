package controller

import (
	"github.com/clktmr/n64/rcp/serial/joybus"
)

type Port struct {
	current, last struct {
		device joybus.Device
		flags  byte
	}

	number uint8
	err    error
}

func (p *Port) Nr() uint8 {
	return p.number
}

const (
	pakInserted    = 0x01
	pakNotInserted = 0x02
)

type Controller struct {
	Port

	current, last struct {
		down  joybus.ButtonMask
		xAxis int8
		yAxis int8
	}

	err error
}

func (c *Controller) Down() joybus.ButtonMask {
	return c.current.down
}

func (c *Controller) Changed() joybus.ButtonMask {
	return c.current.down ^ c.last.down
}

func (c *Controller) Pressed() joybus.ButtonMask {
	return c.Changed() & c.current.down
}

func (c *Controller) Released() joybus.ButtonMask {
	return c.Changed() & c.last.down
}

func (c *Controller) X() int8 {
	return c.current.xAxis
}

func (c *Controller) Y() int8 {
	return c.current.yAxis
}

func (c *Controller) DX() int8 {
	return c.current.xAxis - c.last.xAxis
}

func (c *Controller) DY() int8 {
	return c.current.yAxis - c.last.yAxis
}

func (c *Controller) Plugged() bool {
	return c.Port.current.device == joybus.Controller &&
		c.Port.last.device != joybus.Controller
}

func (c *Controller) Unplugged() bool {
	return c.Port.current.device != joybus.Controller &&
		c.Port.last.device == joybus.Controller
}

func (c *Controller) PakInserted() bool {
	return c.Port.current.flags&pakInserted != 0 &&
		c.Port.last.flags&pakInserted == 0
}

func (c *Controller) PakRemoved() bool {
	return c.Port.current.flags&pakNotInserted != 0 &&
		c.Port.last.flags&pakInserted != 0
}
