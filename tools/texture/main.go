package texture

import (
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"strings"

	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/clktmr/n64/rcp/texture"
	"github.com/ericpauley/go-quantize/quantize"
)

var (
	flags = flag.NewFlagSet("texture", flag.ExitOnError)

	format  = flags.String("format", "RGBA32", "image format and bit depth")
	dither  = flags.Bool("dither", false, "enable Floyd-Steinberg error diffusion")
	palette = flags.Int("palette", 256, "number of colors in CI4 and CI8 format")

	imagefile string
)

const usageString = `Image to n64 texture converter.

Usage: %s [flags] <image>

`

func usage() {
	fmt.Fprintf(flags.Output(), usageString, "texture")
	flags.PrintDefaults()
}

func Main(args []string) {
	flags.Usage = usage
	flags.Parse(args[1:])

	if flags.NArg() == 1 {
		imagefile = flags.Arg(0)
	} else {
		flags.Usage()
		os.Exit(1)
	}

	// Read the font data.
	r, err := os.Open(imagefile)
	if err != nil {
		log.Fatalln(err)
	}

	src, _, err := image.Decode(r)
	if err != nil {
		log.Fatalln(err)
	}

	var dst *texture.Texture

	switch *format {
	case "RGBA32":
		dst = texture.NewRGBA32(src.Bounds())
	case "RGBA16":
		dst = texture.NewRGBA16(src.Bounds())
	// case "YUV16":
	// case "IA16":
	// case "IA8":
	// case "IA4":
	case "I8":
		dst = texture.NewI8(src.Bounds())
	case "I4":
		dst = texture.NewI4(src.Bounds())
	case "CI8":
		q := quantize.MedianCutQuantizer{}
		p := q.Quantize(make([]color.Color, 0, *palette), src)
		cp, err := texture.CopyColorPalette(p)
		if err != nil {
			log.Fatal(err)
		}
		dst = texture.NewCI8(src.Bounds(), cp)
	// case "CI4":
	default:
		log.Fatal("unsupported format:", *format)
	}

	var d draw.Drawer = draw.Src
	if *dither {
		d = draw.FloydSteinberg
	}

	d.Draw(dst, dst.Bounds(), src, image.Point{})

	outfile := strings.TrimSuffix(imagefile, filepath.Ext(imagefile))
	outfile += "." + *format
	w, err := os.Create(outfile)
	if err != nil {
		log.Fatalln(err)
	}
	defer w.Close()

	err = dst.Store(w)
	if err != nil {
		log.Fatalln(err)
	}
}
