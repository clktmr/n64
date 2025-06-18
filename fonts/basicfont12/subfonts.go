// basicfont 12
package basicfont12

import (
	"embed"

	"github.com/clktmr/n64/drivers/cartfs"
	"github.com/clktmr/n64/fonts"
	"github.com/embeddedgo/display/font/subfont"
)

const (
	Height = 13
	Ascent = 11
)

//go:embed *.tex *.pos
var _fontData embed.FS
var fontData = cartfs.Embed(_fontData)

func NewFace() *fonts.Face {
	return &fonts.Face{
		subfont.Face{Height: Height,
			Ascent: Ascent,
			Loader: &fonts.Loader{&fontData, Height, Ascent},
		},
	}
}
