package gomono12

import "github.com/embeddedgo/display/font/subfont"

const (
	Height = 15
	Ascent = 12
)

func NewFace(subfonts ...*subfont.Subfont) *subfont.Face {
	return &subfont.Face{Height: Height, Ascent: Ascent, Subfonts: subfonts}
}

func X0000_00ff() *subfont.Subfont {
	return &subfont.Subfont{
		First:  0x0000,
		Last:   0x00ff,
		Offset: 0,
		Data:   load(),
	}
}
