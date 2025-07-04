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
	HasAlpha      bool
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
	rect := image.Rect(0, 0, int(hdr.Width), int(hdr.Height))
	switch hdr.Format {
	case RGBA32:
		tex = NewRGBA32(rect)
	case RGBA16:
		tex = NewRGBA16(rect)
	// case fmtYUV16:
	// case fmtIA16:
	// case fmtIA8:
	// case fmtIA4:
	case I8:
		if hdr.HasAlpha {
			tex = NewAlpha(rect)
		} else {
			tex = NewI8(rect)
		}
	case I4:
		tex = NewI4(rect)
	case CI8:
		palette, err := NewColorPalette(int(hdr.PaletteSize))
		if err != nil {
			return nil, err
		}
		tex = NewCI8(rect, palette)
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
		Format:   p.Format(),
		HasAlpha: p.HasAlpha(),
		Width:    uint16(p.Bounds().Dx()),
		Height:   uint16(p.Bounds().Dy()),
	}

	if p.palette != nil {
		r := p.palette.Bounds()
		hdr.PaletteSize = uint16(r.Dx() * r.Dy())
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
