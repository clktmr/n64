package main

import (
	"errors"
	"io"
	"io/fs"

	"github.com/clktmr/n64/drivers/controller/pakfs"
	"rsc.io/rsc/fuse"
)

// FS implements the file system and the root dir Node.
type FS struct {
	pakfs *pakfs.FS
}

func (p *FS) Root() (fuse.Node, fuse.Error) {
	return p, nil
}

func (p *FS) Attr() fuse.Attr {
	dir := must(p.pakfs.Open("."))
	stat := must(dir.Stat())
	return fuse.Attr{
		Mode:  stat.Mode(),
		Mtime: stat.ModTime(),
	}
}

func (p *FS) Lookup(name string, intr fuse.Intr) (fuse.Node, fuse.Error) {
	f, err := p.pakfs.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fuse.ENOENT
	} else if err != nil {
		return nil, fuse.EIO
	}
	pakfile, ok := f.(*pakfs.File)
	if !ok {
		return p, nil // must be root dir
	}
	return &File{pakfile, p.pakfs}, nil
}

func (p *FS) ReadDir(intr fuse.Intr) ([]fuse.Dirent, fuse.Error) {
	entries := p.pakfs.Root()
	fuseEntries := make([]fuse.Dirent, len(entries))
	for i, v := range entries {
		fuseEntries[i] = fuse.Dirent{
			Name: v.Name(),
		}
	}

	return fuseEntries, nil
}

func (p *FS) Create(req *fuse.CreateRequest, res *fuse.CreateResponse, intr fuse.Intr) (fuse.Node, fuse.Handle, fuse.Error) {
	f, err := p.pakfs.Create(req.Name)
	if err != nil {
		return nil, nil, fuse.EIO
	}

	file := &File{f, p.pakfs}
	return file, file, nil
}

func (p *FS) Remove(req *fuse.RemoveRequest, intr fuse.Intr) fuse.Error {
	err := p.pakfs.Remove(req.Name)
	if err != nil {
		return fuse.EIO
	}
	return nil
}

// File implements both Node and Handle.
type File struct {
	*pakfs.File

	pakfs *pakfs.FS
}

func (p *File) Attr() fuse.Attr {
	return fuse.Attr{
		Mode:  p.Mode(),
		Mtime: p.ModTime(),
		Size:  uint64(p.Size()),
	}
}

func (p *File) ReadAll(intr fuse.Intr) ([]byte, fuse.Error) {
	b := make([]byte, p.Size())
	_, err := p.ReadAt(b, 0)
	if err != io.EOF && err != nil {
		return nil, fuse.EIO
	}
	return b, nil
}

// Only WriteAll is supported.  Write is not implemented on purpose because it
// might cause unexpected behaviour when appending to a file, since filesize is
// always rounded up to the next page boundary.
func (p *File) WriteAll(data []byte, intr fuse.Intr) fuse.Error {
	err := p.pakfs.Truncate(p.File.Name(), int64(len(data)))
	if err != nil {
		return fuse.EIO
	}

	_, err = p.WriteAt(data, 0)
	if err != nil {
		return fuse.EIO
	}

	return nil
}

func (p *File) Fsync(req *fuse.FsyncRequest, intr fuse.Intr) fuse.Error {
	return nil
}
