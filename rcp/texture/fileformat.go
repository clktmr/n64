package texture

import (
	"compress/zlib"
	"encoding/binary"
	"errors"
	"image"
	"io"
)

type Format uint64

const (
	fmtRGBA32 = Format(RGBA) | Format(BPP32)
	fmtRGBA16 = Format(RGBA) | Format(BPP16)
	fmtYUV16  = Format(YUV) | Format(BPP16)
	fmtIA16   = Format(IA) | Format(BPP16)
	fmtIA8    = Format(IA) | Format(BPP8)
	fmtIA4    = Format(IA) | Format(BPP4)
	fmtI8     = Format(I) | Format(BPP8)
	fmtI4     = Format(I) | Format(BPP4)
	fmtCI8    = Format(CI) | Format(BPP8)
	fmtCI4    = Format(CI) | Format(BPP4)
)

type header struct {
	Format        Format
	Premult       bool
	Width, Height uint16
	PaletteSize   uint16
}

func Load(r io.Reader) (tex *Texture, err error) {
	zr, err := zlib.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	var hdr header
	err = binary.Read(zr, binary.BigEndian, &hdr)
	if err != nil {
		return nil, err
	}
	switch hdr.Format {
	case fmtRGBA32:
		if hdr.Premult {
			tex = NewRGBA32(image.Rect(0, 0, int(hdr.Width), int(hdr.Height)))
		} else {
			tex = NewNRGBA32(image.Rect(0, 0, int(hdr.Width), int(hdr.Height)))
		}
	case fmtRGBA16:
		tex = NewRGBA16(image.Rect(0, 0, int(hdr.Width), int(hdr.Height)))
	// case fmtYUV16:
	// case fmtIA16:
	// case fmtIA8:
	// case fmtIA4:
	case fmtI8:
		tex = NewI8(image.Rect(0, 0, int(hdr.Width), int(hdr.Height)))
	case fmtI4:
		tex = NewI4(image.Rect(0, 0, int(hdr.Width), int(hdr.Height)))
	// case fmtCI8:
	// case fmtCI4:
	default:
		return nil, errors.New("unsupported format")
	}

	_, err = io.ReadFull(zr, tex.pix)
	if err != nil && err != io.EOF {
		return nil, err
	}

	tex.Writeback()

	return tex, nil
}

func (p *Texture) Store(w io.Writer) error {
	if p.stride != p.Bounds().Dx() {
		return errors.New("is subimage")
	}

	var hdr = header{
		Format:  Format(p.BPP()) | Format(p.Format()),
		Premult: p.Premult(),
		Width:   uint16(p.Bounds().Dx()),
		Height:  uint16(p.Bounds().Dy()),
	}

	zw := zlib.NewWriter(w)
	defer zw.Close()
	err := binary.Write(zw, binary.BigEndian, hdr)
	if err != nil {
		return err
	}

	_, err = zw.Write(p.pix)

	return err
}
