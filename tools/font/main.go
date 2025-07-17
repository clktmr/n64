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
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/clktmr/n64/rcp/texture"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

var (
	flags = flag.NewFlagSet("font", flag.ExitOnError)

	dpi      = flags.Float64("dpi", 72, "screen resolution in Dots Per Inch")
	hinting  = flags.String("hinting", "none", "none | full")
	size     = flags.Float64("size", 12, "font size in points")
	spacing  = flags.Float64("spacing", 1.0, "line spacing")
	start    = flags.Uint("start", 0, "Unicode value of first character")
	end      = flags.Uint("end", 0xff, "Unicode value of last character")
	genpng   = flags.Bool("png", false, "Generate PNG for debugging")
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
	if fontfile == "basicfont" {
		face = basicfont.Face7x13
		name = "basicfont"
	} else {
		// Read the font data.
		var err error
		var fontBytes []byte
		if fontfile == "gomono" {
			fontBytes = gomono.TTF
			name = "gomono"
		} else if fontfile == "goregular" {
			fontBytes = goregular.TTF
			name = "goregular"
		} else {
			fontBytes, err = os.ReadFile(fontfile)
			if err != nil {
				log.Fatalln(err)
			}
		}
		f, err := opentype.Parse(fontBytes)
		if err != nil {
			log.Fatalln(err)
		}

		options := &opentype.FaceOptions{
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
		face, err = opentype.NewFace(f, options)
		if err != nil {
			log.Fatalln(err)
		}
		defer face.Close()
		name, _ = f.Name(nil, sfnt.NameIDFull)
		if err != nil {
			log.Fatalln(err)
		}
	}

	// Initialize the context.
	fontMap := texture.NewI4(image.Rect(0, 0, dim, dim))
	drawer := font.Drawer{Dst: fontMap, Src: image.White, Face: face}

	// Draw the font file
	spacingFixed := fixed.Int26_6(*spacing * (1 << 6))
	lineHeight := face.Metrics().Height.Mul(spacingFixed).Ceil()

	// One pixel pads allows us to use clamping to draw larger background
	const pad = 1
	drawer.Dot = fixed.P(pad, pad)

	var missing []byte
	for s := rune(*start); s <= rune(*end); s++ {
		bounds, adv, hasGlyph := face.GlyphBounds(s)
		bounds.Max = bounds.Max.Add(fixed.P(pad, pad))

		// Use a common "missing" glyph
		if !hasGlyph && missing != nil {
			positions = append(positions, missing...)
			continue
		}

		// Always advance origin to full pixels
		nextDot := drawer.Dot
		nextDot.X -= fixed.I(bounds.Min.X.Floor())
		nextDot.Y -= fixed.I(bounds.Min.Y.Floor())

		// Check if we need to wrap
		if bounds.Add(nextDot).Max.X.Ceil() >= dim {
			drawer.Dot.Y += fixed.I(lineHeight + pad)
			drawer.Dot.X = fixed.I(pad)
			nextDot = drawer.Dot
			nextDot.X -= fixed.I(bounds.Min.X.Floor())
			nextDot.Y -= fixed.I(bounds.Min.Y.Floor())
		}
		if bounds.Add(nextDot).Max.Y.Ceil() >= dim {
			log.Fatalln("Too many glyphs to fit into font image map")
		}

		drawer.Dot = nextDot
		bounds = bounds.Add(drawer.Dot)

		positions = append(positions, byte(drawer.Dot.X.Round()))
		positions = append(positions, byte(drawer.Dot.Y.Round()))
		positions = append(positions, byte(bounds.Min.X.Floor()-pad))
		positions = append(positions, byte(bounds.Min.Y.Floor()-pad))
		positions = append(positions, byte(bounds.Max.X.Ceil()))
		positions = append(positions, byte(bounds.Max.Y.Ceil()))
		positions = append(positions, byte(adv.Round()))

		if !hasGlyph && missing == nil {
			missing = positions[len(positions)-7:]
		}

		// Actual drawing
		drawer.DrawString(string(s))
		drawer.Dot = fixed.P(bounds.Max.X.Ceil(), bounds.Min.Y.Floor())
	}

	lastLine := drawer.Dot.Y.Ceil() + lineHeight + pad
	shrinkedFontMap := fontMap.SubImage(image.Rect(0, 0, dim, lastLine))

	// Save the font map image to disk
	pkgname := name
	pkgname = strings.ReplaceAll(pkgname, " ", "")
	pkgname = fmt.Sprintf("%s%.0f", strings.ToLower(pkgname), (*size))

	directory := filepath.Join(".", pkgname)
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

	if *genpng {
		filename = fmt.Sprintf("%s.png", basename)
		pngFile, err := os.Create(filepath.Join(directory, filename))
		if err != nil {
			log.Fatalln(err)
		}
		defer pngFile.Close()
		err = png.Encode(pngFile, shrinkedFontMap)
		if err != nil {
			log.Fatalln(err)
		}
	}

	// Save glyph positions to disk
	filename = fmt.Sprintf("%s.pos", basename)
	posFile, err := os.Create(filepath.Join(directory, filename))
	if err != nil {
		log.Fatalln(err)
	}
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
			Loader: fonts.NewLoader(&fontData, Height, Ascent),
		},
	}
}
`
