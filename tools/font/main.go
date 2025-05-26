// Copyright 2010 The Freetype-Go Authors. All rights reserved.
// Use of this source code is governed by your choice of either the
// FreeType License or the GNU General Public License version 2 (or
// any later version), both of which can be found in the LICENSE file.

package font

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

var (
	flags = flag.NewFlagSet("font", flag.ExitOnError)

	dpi      = flags.Float64("dpi", 72, "screen resolution in Dots Per Inch")
	fontfile = flags.String("fontfile", "", "filename of the ttf font")
	hinting  = flags.String("hinting", "none", "none | full")
	size     = flags.Float64("size", 12, "font size in points")
	spacing  = flags.Float64("spacing", 1.25, "line spacing")
	start    = flags.Uint("start", 0, "Unicode value of first character")
	end      = flags.Uint("end", 0xff, "Unicode value of last character")
)

const (
	dim = 256
)

var positions []byte

func Main(args []string) {
	flags.Parse(args[1:])

	// Read the font data.
	fontBytes, err := os.ReadFile(*fontfile)
	if err != nil {
		log.Fatalln(err)
	}
	f, err := freetype.ParseFont(fontBytes)
	if err != nil {
		log.Fatalln(err)
	}

	// Initialize the context.
	fg, bg := image.White, image.Black
	fontMap := image.NewGray(image.Rect(0, 0, dim, dim))
	draw.Draw(fontMap, fontMap.Bounds(), bg, image.ZP, draw.Src)
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

		if f.Index(s) == 0 && missing == nil {
			missing = positions[len(positions)-3:]
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
	directory := fmt.Sprintf("fonts/%s%.0f/", f.Name(truetype.NameIDFontFullName), (*size))
	directory = strings.ReplaceAll(directory, " ", "")
	directory = strings.ToLower(directory)
	os.MkdirAll(directory, 0775)
	basename := fmt.Sprintf("%04x_%04x", *start, *end)

	filename := fmt.Sprintf("%s.png", basename)
	mapFile, err := os.Create(filepath.Join(directory, filename))
	if err != nil {
		log.Fatalln(err)
	}
	defer mapFile.Close()
	b := bufio.NewWriter(mapFile)
	err = png.Encode(b, shrinkedFontMap)
	if err != nil {
		log.Fatalln(err)
	}
	err = b.Flush()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("Wrote %s\n", mapFile.Name())

	// Save glyph positions to disk
	filename = fmt.Sprintf("%s.pos", basename)
	posFile, err := os.Create(filepath.Join(directory, filename))
	defer posFile.Close()

	for _, pos := range positions {
		// TODO use gob instead of binary encoding?
		err = binary.Write(posFile, binary.BigEndian, pos)
		if err != nil {
			log.Fatalln(err)
		}
	}
	fmt.Printf("Wrote %s\n", posFile.Name())
}
