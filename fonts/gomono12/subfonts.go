package gomono12

import (
	_ "embed"

	"github.com/clktmr/n64/fonts"
	"github.com/embeddedgo/display/font/subfont"
)

const (
	Height = 15
	Ascent = 12
)

//go:embed 0000_00ff.pos
var X0000_pos string

//go:embed 0000_00ff.png
var X0000_png string

func NewFace(subfonts ...*subfont.Subfont) *fonts.Face {
	return &fonts.Face{
		subfont.Face{Height: Height, Ascent: Ascent, Subfonts: subfonts},
	}
}

func X0000_00ff() *subfont.Subfont {
	return &subfont.Subfont{
		First:  0x0000,
		Last:   0x00ff,
		Offset: 0,
		Data:   fonts.NewSubfontData(X0000_pos, X0000_png, Height, Ascent),
	}
}
