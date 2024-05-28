package rsp

func Init() {
	regs.status.Store(setHalt)
	pc.Store(0x1000)
}

func Handler() {
}
