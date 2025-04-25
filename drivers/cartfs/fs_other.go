//go:build !n64

package cartfs

import (
	"embed"
	"io/fs"
)

type baseType = *embed.FS

func embedfs(f embed.FS) FS { return FS{base: &f} }

func (f *FS) baseOpen(name string) (fs.File, error) {
	if f.base != nil {
		f, err := (*f.base).Open(name)
		return f, err
	}
	return f.cartfsOpen(name)
}

func (f *FS) baseReadFile(name string) ([]byte, error) {
	if f.base != nil {
		return (*f.base).ReadFile(name)
	}
	return f.cartfsReadFile(name)
}

func (f *FS) baseReadDir(name string) ([]fs.DirEntry, error) {
	if f.base != nil {
		return (*f.base).ReadDir(name)
	}
	return f.cartfsReadDir(name)
}
