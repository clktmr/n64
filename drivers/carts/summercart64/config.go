package summercart64

type config uint32

// Configuration options for [Cart.SetConfig].
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

// Config option values for [CfgButtonMode].
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
