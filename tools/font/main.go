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
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/clktmr/n64/rcp/texture"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
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

	switch flags.NArg() {
	case 0:
		break
	case 1:
		fontfile = flags.Arg(0)
	default:
		flags.Usage()
		os.Exit(1)
	}

	// TODO check for overlapping with previously generated subfonts

	var face font.Face
	var name string
	if fontfile == "" {
		face = basicfont.Face7x13
		name = "basicfont"
	} else {
		// Read the font data.
		fontBytes, err := os.ReadFile(fontfile)
		if err != nil {
			log.Fatalln(err)
		}
		f, err := freetype.ParseFont(fontBytes)
		if err != nil {
			log.Fatalln(err)
		}

		options := &truetype.Options{
			Size: *size,
			DPI:  *dpi,
		}
		switch *hinting {
		default:
			options.Hinting = font.HintingNone
		case "vertical":
			options.Hinting = font.HintingVertical
		case "full":
			options.Hinting = font.HintingFull
		}
		face = truetype.NewFace(f, options)
		defer face.Close()
		name = f.Name(truetype.NameIDFontFullName)
	}

	// Initialize the context.
	fontMap := texture.NewI4(image.Rect(0, 0, dim, dim))
	drawer := font.Drawer{Dst: fontMap, Src: image.White, Face: face}

	// Draw the font file
	spacingFixed := fixed.Int26_6(*spacing * (1 << 6))
	lineHeight := face.Metrics().Height.Mul(spacingFixed).Ceil()
	drawer.Dot = fixed.Point26_6{0, fixed.I(lineHeight)}
	var missing []byte
	for s := rune(*start); s <= rune(*end); s++ {
		// Use a common "missing" glyph

		if _, ok := face.GlyphAdvance(s); !ok && missing != nil {
			positions = append(positions, missing...)
			continue
		}
		// Always start drawing at full pixels
		drawer.Dot = fixed.P(drawer.Dot.X.Ceil(), drawer.Dot.Y.Ceil())

		// Check if we need to wrap
		const padding = 1
		adv, _ := face.GlyphAdvance(s)
		nextDot := drawer.Dot.Add(fixed.P(adv.Ceil()+padding, 0))

		if nextDot.X.Ceil() >= dim {
			drawer.Dot.Y += fixed.I(lineHeight + padding)
			drawer.Dot.X = fixed.I(0)
		}
		if nextDot.Y.Ceil() >= dim {
			log.Fatalln("Too many glyphs to fit into font image map")
		}

		positions = append(positions, byte(drawer.Dot.X.Round()))
		positions = append(positions, byte(drawer.Dot.Y.Round()))
		positions = append(positions, byte(adv.Ceil()))

		if _, ok := face.GlyphAdvance(s); !ok && missing == nil {
			missing = positions[len(positions)-3:]
		}

		// Actual drawing
		drawer.DrawString(string(s))
		drawer.Dot = drawer.Dot.Add(fixed.P(padding, 0))
	}

	lastLine := drawer.Dot.Y.Ceil() + lineHeight
	shrinkedFontMap := fontMap.SubImage(image.Rect(0, 0, dim, lastLine))

	// Save the font map image to disk
	pkgname := name
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

	err = tmpl.Execute(subfontsFile, struct {
		Name, Package  string
		Height, Ascent int
	}{
		Name:    fmt.Sprintf("%s %g", name, *size),
		Package: pkgname,
		Height:  lineHeight,
		Ascent:  face.Metrics().Ascent.Round(),
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
