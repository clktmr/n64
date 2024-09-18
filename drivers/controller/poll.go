package controller

import (
	"github.com/clktmr/n64/rcp/serial"
	"github.com/clktmr/n64/rcp/serial/joybus"
)

var (
	cmdAllStates      *serial.CommandBlock
	cmdAllStatesPorts [4]joybus.ControllerStateCommand

	cmdAllInfo      *serial.CommandBlock
	cmdAllInfoPorts [4]joybus.InfoCommand
)

func init() {
	var err error

	cmdAllStates = serial.NewCommandBlock(serial.CmdConfigureJoybus)
	for i := range cmdAllStatesPorts {
		cmdAllStatesPorts[i], err = joybus.NewControllerStateCommand(cmdAllStates)
		if err != nil {
			panic(err)
		}
	}
	err = joybus.ControlByte(cmdAllStates, joybus.CtrlAbort)
	if err != nil {
		panic(err)
	}

	cmdAllInfo = serial.NewCommandBlock(serial.CmdConfigureJoybus)
	for i := range cmdAllInfoPorts {
		cmdAllInfoPorts[i], err = joybus.NewInfoCommand(cmdAllInfo)
		if err != nil {
			panic(err)
		}
	}
	err = joybus.ControlByte(cmdAllInfo, joybus.CtrlAbort)
	if err != nil {
		panic(err)
	}
}

var states [4]Controller

func Poll() [4]Controller {
	for _, v := range cmdAllStatesPorts {
		v.Command.Reset()
	}

	serial.Run(cmdAllStates)

	for i := range states {
		states[i].last = states[i].current
		cur := &states[i].current
		cur.down, cur.xAxis, cur.yAxis, states[i].err = cmdAllStatesPorts[i].State()
	}

	return states
}

func PollInfo() {
	for _, v := range cmdAllInfoPorts {
		v.Command.Reset()
	}
	serial.Run(cmdAllInfo)

	for i := range states {
		var err error
		states[i].lastInfo = states[i].currentInfo
		info, pak, err := cmdAllInfoPorts[i].Info()
		states[i].err = err
		states[i].currentInfo.plugged = (info == joybus.Controller)
		states[i].currentInfo.pak = pak&0x01 != 0
	}
}
