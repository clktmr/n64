package summercart64

import "errors"

type config uint32

var (
	ErrBadArgument = errors.New("bad argument")
	ErrBadAddress  = errors.New("bad address")
	ErrBadConfigId = errors.New("bad config id")
	ErrTimeout     = errors.New("timeout")
	ErrSdCard      = errors.New("sdcard")
	ErrUnknownCmd  = errors.New("unknown command")
)

var errCodes = map[uint32]error{
	1: ErrBadArgument,
	2: ErrBadAddress,
	3: ErrBadConfigId,
	4: ErrTimeout,
	5: ErrSdCard,

	0xffffff: ErrUnknownCmd,
}

const (
	CfgBootloaderSwitch = iota
	CfgROMWriteEnable
	CfgROMShadowEnable
	CfgDDMode
	CfgISVAddress
	CfgBootMode
	CfgSaveType
	CfgCICSeed
	CfgTVType
	CfgDDSDEnable
	CfgDDDriveType
	CfgDDDiskState
	CfgButtonState
	CfgButtonMode
	CfgROMExtendedEnable
)

// Config option values
const (
	ButtonModeDisabled uint32 = iota
	ButtonModeInterrupt
	ButtonModeUSBPacket
	ButtonMode64DDDiskChange
)

func (v *Cart) SetConfig(option config, value uint32) (old uint32, err error) {
	_, old, err = execCommand(cmdConfigSet, uint32(option), value)
	return
}

func (v *Cart) Config(option config) (current uint32, err error) {
	_, current, err = execCommand(cmdConfigGet, uint32(option), 0)
	return
}
