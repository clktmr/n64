package summercart64

type config uint32

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

func (v *SummerCart64) SetConfig(option config, value uint32) (old uint32, err error) {
	_, old, err = execCommand(cmdConfigSet, uint32(option), value)
	return
}

func (v *SummerCart64) Config(option config) (current uint32, err error) {
	_, current, err = execCommand(cmdConfigSet, uint32(option), 0)
	return
}
