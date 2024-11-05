package fonts

import (
	"image"

	"github.com/embeddedgo/display/font/subfont"
)

// GlyphMap returns an image containing all glyphs of a Subfont and a rect
// describing subimage that represents the glyph.  All images returned by
// GlyphMap are guaranteed to have the same format/type.
// This interface is an optimization for the subfont.Data interface. Subfonts
// can implement this for optimization, to avoid frequent changes in the RDP's
// texture image.
type Data interface {
	GlyphMap(i int) (img image.Image, rect image.Rectangle, origin image.Point, advance int)
}

type Face struct {
	subfont.Face
}

// Glyph implements font.Face interface.
func (f *Face) GlyphMap(r rune) (img image.Image, rect image.Rectangle, origin image.Point, advance int) {
	sf := getSubfont(f, r)
	if sf == nil {
		sf = getSubfont(f, 0) // try to use rune(0) to render unsupported codepoints
		if sf == nil {
			return
		}
	}
	if sf, ok := sf.Data.(Data); ok {
		return sf.GlyphMap(int(r))
	}
	img, origin, advance = sf.Data.Glyph(int(r - sf.First))
	rect = img.Bounds()
	return
}

func getSubfont(f *Face, r rune) (sf *subfont.Subfont) {
	// TODO: binary search
	for _, sf = range f.Subfonts {
		if sf != nil && sf.First <= r && r <= sf.Last {
			return sf
		}
	}
	if f.Loader == nil {
		return nil
	}
	sf, f.Subfonts = f.Loader.Load(r, f.Subfonts)
	return sf
}
