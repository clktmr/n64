package controller

import (
	"github.com/clktmr/n64/rcp/serial/joybus"
)

type state struct {
	down  joybus.ButtonMask
	xAxis int8
	yAxis int8
}

type info struct {
	plugged bool
	pak     bool
}

// TODO move this to something like n64/engine/input
type Controller struct {
	currentInfo, lastInfo info
	current, last         state
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
	return c.currentInfo.plugged && !c.lastInfo.plugged
}

func (c *Controller) Unplugged() bool {
	return !c.currentInfo.plugged && c.lastInfo.plugged
}

func (c *Controller) PakInserted() bool {
	return c.currentInfo.pak && !c.lastInfo.pak
}

func (c *Controller) PakRemoved() bool {
	return !c.currentInfo.pak && c.lastInfo.pak
}
