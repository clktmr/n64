//go:build n64

package cartfs

import (
	"embed"
	"errors"
	"io/fs"
	"math"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

// On target n64 base specifies the pi bus address where to find the cartfs to
// read from. If is set to a non-zero value, a pi bus device is initialized on
// first use of the filesystem.
type baseType = cpu.Addr

func embedfs(_ embed.FS) FS {
	// Initialize with non-zero value, just to force the symbol to be placed
	// in .data instead of .bss. Actual initialization will be done after
	// linking by the mkrom tool.
	return FS{base: 0xffff_ffff}
}

func (f *FS) baseInit() error {
	if f.dev != nil || f.base == 0x0 {
		return nil
	}
	if f.base == 0xffff_ffff {
		return errors.New("cartfs.Embed not initialized by mkrom")
	}
	dev := periph.NewDevice(f.base, math.MaxUint32)
	fnew, err := Read(dev)
	if err != nil {
		return err
	}
	*f = *fnew
	return nil
}

func (f *FS) baseOpen(name string) (fs.File, error) {
	if err := f.baseInit(); err != nil {
		return nil, err
	}
	return f.cartfsOpen(name)
}
func (f *FS) baseReadFile(name string) ([]byte, error) {
	if err := f.baseInit(); err != nil {
		return nil, err
	}
	return f.cartfsReadFile(name)
}
func (f *FS) baseReadDir(name string) ([]fs.DirEntry, error) {
	if err := f.baseInit(); err != nil {
		return nil, err
	}
	return f.cartfsReadDir(name)
}
