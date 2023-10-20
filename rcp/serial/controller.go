package serial

type Button uint16

const (
	A Button = 1 << (15 - iota)
	B
	Z
	Start
	DPadUp
	DPadDown
	DPadLeft
	DPadRight
	Reset // L+R+Start pressed simltaneously
	_
	L
	R
	CUp
	CDown
	CLeft
	CRight
)

type ControllerState struct {
	Down    Button
	Changed Button
	XAxis   int8
	YAxis   int8
}

func (c *ControllerState) Pressed(b Button) bool {
	return (c.Down&b) != 0 && (c.Changed&b) != 0
}
func (c *ControllerState) Released(b Button) bool {
	return (c.Down&b) == 0 && (c.Changed&b) != 0
}

var lastDown [4]Button

func ControllerStates() [4]ControllerState {
	msg := message{
		buf: [32]uint16{
			0xff01, 0x0401, 0xffff, 0xffff,
			0xff01, 0x0401, 0xffff, 0xffff,
			0xff01, 0x0401, 0xffff, 0xffff,
			0xff01, 0x0401, 0xffff, 0xffff,
			0xfe00, 0x0000, 0x0000, 0x0000,
			0x0000, 0x0000, 0x0000, 0x0000,
			0x0000, 0x0000, 0x0000, 0x0000,
			0x0000, 0x0000, 0x0000, 0x0001,
		},
	}

	response := Query(&msg).buf
	states := [4]ControllerState{}

	off := 0
	for i := range states {
		states[i].Down = Button(response[off+2])
		states[i].Changed = states[i].Down ^ lastDown[i]
		states[i].XAxis = int8(response[off+3] >> 8)
		states[i].YAxis = int8(response[off+3])

		lastDown[i] = states[i].Down
		off += 4
	}

	return states
}
