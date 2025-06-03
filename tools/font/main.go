// Copyright 2010 The Freetype-Go Authors. All rights reserved.
// Use of this source code is governed by your choice of either the
// FreeType License or the GNU General Public License version 2 (or
// any later version), both of which can be found in the LICENSE file.

package font

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/clktmr/n64/rcp/texture"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

var (
	flags = flag.NewFlagSet("font", flag.ExitOnError)

	dpi      = flags.Float64("dpi", 72, "screen resolution in Dots Per Inch")
	hinting  = flags.String("hinting", "none", "none | full")
	size     = flags.Float64("size", 12, "font size in points")
	spacing  = flags.Float64("spacing", 1.25, "line spacing")
	start    = flags.Uint("start", 0, "Unicode value of first character")
	end      = flags.Uint("end", 0xff, "Unicode value of last character")
	fontfile string
)

const usageString = `TrueType Font to n64 glyphmap converter.

Usage: %s [flags] <ttffile>

`

func usage() {
	fmt.Fprintf(flags.Output(), usageString, "font")
	flags.PrintDefaults()
}

const (
	dim = 256
)

var positions []byte

func Main(args []string) {
	flags.Usage = usage
	flags.Parse(args[1:])

	if flags.NArg() == 1 {
		fontfile = flags.Arg(0)
	} else {
		flags.Usage()
		os.Exit(1)
	}

	// TODO check for overlapping with previously generated subfonts

	// Read the font data.
	fontBytes, err := os.ReadFile(fontfile)
	if err != nil {
		log.Fatalln(err)
	}
	f, err := freetype.ParseFont(fontBytes)
	if err != nil {
		log.Fatalln(err)
	}

	// Initialize the context.
	fg, bg := image.White, image.Black
	fontMap := texture.NewI4(image.Rect(0, 0, dim, dim))
	draw.Draw(fontMap, fontMap.Bounds(), bg, image.Point{}, draw.Src)
	c := freetype.NewContext()
	c.SetDPI(*dpi)
	c.SetFont(f)
	c.SetFontSize(*size)
	c.SetClip(fontMap.Bounds())
	c.SetDst(fontMap)
	c.SetSrc(fg)
	switch *hinting {
	default:
		c.SetHinting(font.HintingNone)
	case "full":
		c.SetHinting(font.HintingFull)
	}

	// Draw the font file
	lineHeight := c.PointToFixed((*size) * (*spacing)).Ceil()
	pt := freetype.Pt(0, lineHeight)
	var missing []byte
	for s := rune(*start); s <= rune(*end); s++ {
		// Use a common "missing" glyph
		if f.Index(s) == 0 && missing != nil {
			positions = append(positions, missing...)
			continue
		}
		// Always start drawing at a fraction of a pixel
		pt = fixed.P(pt.X.Ceil(), pt.Y.Ceil())

		// Try to draw and check if we need to wrap
		c.SetSrc(bg)
		nextPt, err := c.DrawString(string(s), pt)
		if err != nil {
			log.Fatalln(err)
		}

		adv := byte(nextPt.X.Floor() - pt.X.Floor())

		if nextPt.X.Ceil() >= dim {
			pt.Y += c.PointToFixed(float64(lineHeight))
			pt.X = c.PointToFixed(0)
		}
		if nextPt.Y.Ceil() >= dim {
			log.Fatalln("Too many glyphs to fit into font image map")
		}

		positions = append(positions, byte(pt.X.Floor()))
		positions = append(positions, byte(pt.Y.Floor()))
		positions = append(positions, adv)

		if f.Index(s) == 0 && missing == nil {
			missing = positions[len(positions)-3:]
		}

		// Actual drawing
		c.SetSrc(fg)
		pt, err = c.DrawString(string(s), pt)
		if err != nil {
			log.Fatalln(err)
		}
	}

	lastLine := pt.Y.Ceil() + lineHeight
	shrinkedFontMap := fontMap.SubImage(image.Rect(0, 0, dim, lastLine))

	// Save the font map image to disk
	pkgname := f.Name(truetype.NameIDFontFullName)
	pkgname = strings.ReplaceAll(pkgname, " ", "")
	pkgname = fmt.Sprintf("%s%.0f", strings.ToLower(pkgname), (*size))

	directory := filepath.Join("fonts", pkgname)
	os.MkdirAll(directory, 0775)
	basename := fmt.Sprintf("%04x_%04x", *start, *end)

	filename := fmt.Sprintf("%s.tex", basename)
	mapFile, err := os.Create(filepath.Join(directory, filename))
	if err != nil {
		log.Fatalln(err)
	}
	defer mapFile.Close()
	err = shrinkedFontMap.Store(mapFile)
	if err != nil {
		log.Fatalln(err)
	}

	// Save glyph positions to disk
	filename = fmt.Sprintf("%s.pos", basename)
	posFile, err := os.Create(filepath.Join(directory, filename))
	defer posFile.Close()

	for _, pos := range positions {
		err = binary.Write(posFile, binary.BigEndian, pos)
		if err != nil {
			log.Fatalln(err)
		}
	}

	// Write package file
	tmpl, err := template.New("subfontsGoTemplate").Parse(subfontsGoTemplate)
	if err != nil {
		log.Fatalln(err)
	}
	subfontsFile, err := os.Create(filepath.Join(directory, "subfonts.go"))
	if err != nil {
		log.Fatalln(err)
	}
	defer subfontsFile.Close()

	// TODO store ascent per rune
	ascent := f.VMetric(c.PointToFixed((*size)), f.Index('A')).AdvanceHeight

	err = tmpl.Execute(subfontsFile, struct {
		Name, Package  string
		Height, Ascent int
	}{
		Name:    fmt.Sprintf("%s %g", f.Name(truetype.NameIDFontFullName), *size),
		Package: pkgname,
		Height:  lineHeight,
		Ascent:  ascent.Ceil(),
	})
	if err != nil {
		log.Fatalln(err)
	}
}

const subfontsGoTemplate = `// {{ .Name }}
package {{ .Package }}

import (
	"embed"

	"github.com/clktmr/n64/drivers/cartfs"
	"github.com/clktmr/n64/fonts"
	"github.com/embeddedgo/display/font/subfont"
)

const (
	Height = {{ .Height }}
	Ascent = {{ .Ascent }}
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
`
