package texture

import (
	"compress/zlib"
	"encoding/binary"
	"errors"
	"image"
	"io"
)

type header struct {
	Format        Format
	Premult       bool
	Width, Height uint16
	PaletteSize   uint8
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
	rect := image.Rect(0, 0, int(hdr.Width), int(hdr.Height))
	switch hdr.Format {
	case RGBA32:
		if hdr.Premult {
			tex = NewRGBA32(rect)
		} else {
			tex = NewNRGBA32(rect)
		}
	case RGBA16:
		tex = NewRGBA16(rect)
	// case fmtYUV16:
	// case fmtIA16:
	// case fmtIA8:
	// case fmtIA4:
	case I8:
		tex = NewI8(rect)
	case I4:
		tex = NewI4(rect)
	case CI8:
		tex = NewCI8(rect, NewColorPalette(hdr.PaletteSize))
	// case fmtCI4:
	default:
		return nil, errors.New("unsupported format")
	}

	_, err = io.ReadFull(zr, tex.pix)
	if err != nil && err != io.EOF {
		return nil, err
	}
	tex.Writeback()

	if hdr.PaletteSize > 0 {
		_, err = io.ReadFull(zr, tex.palette.pix)
		if err != nil && err != io.EOF {
			return nil, err
		}
		tex.palette.Writeback()
	}

	return tex, nil
}

func (p *Texture) Store(w io.Writer) error {
	if p.stride != p.Bounds().Dx() {
		return errors.New("is subimage")
	}

	var hdr = header{
		Format:  p.Format(),
		Premult: p.Premult(),
		Width:   uint16(p.Bounds().Dx()),
		Height:  uint16(p.Bounds().Dy()),
	}

	if p.palette != nil {
		hdr.PaletteSize = uint8(p.palette.Bounds().Dx() * p.palette.Bounds().Dy())
	}

	zw := zlib.NewWriter(w)
	defer zw.Close()
	err := binary.Write(zw, binary.BigEndian, hdr)
	if err != nil {
		return err
	}

	_, err = zw.Write(p.pix)
	if err != nil {
		return err
	}

	if p.palette != nil {
		_, err = zw.Write(p.palette.pix)
		if err != nil {
			return err
		}
	}

	return nil
}
