package main

import (
	"errors"
	"io"
	"io/fs"
	"syscall"

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
	dir := p.pakfs.Root()
	stat := must(dir.Stat())
	return fuse.Attr{
		Mode:  stat.Mode(),
		Mtime: stat.ModTime(),
	}
}

func (p *FS) Lookup(name string, intr fuse.Intr) (fuse.Node, fuse.Error) {
	f, err := p.pakfs.Open(name)
	if err != nil {
		return nil, errno(err)
	}
	pakfile, ok := f.(*pakfs.File)
	if !ok {
		return p, nil // must be root dir
	}
	return &File{pakfile, p.pakfs}, nil
}

func (p *FS) ReadDir(intr fuse.Intr) ([]fuse.Dirent, fuse.Error) {
	entries := p.pakfs.ReadDirRoot()
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
		return nil, nil, errno(err)
	}

	file := &File{f, p.pakfs}
	return file, file, nil
}

func (p *FS) Remove(req *fuse.RemoveRequest, intr fuse.Intr) fuse.Error {
	err := p.pakfs.Remove(req.Name)
	if err != nil {
		return errno(err)
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
		return nil, errno(err)
	}
	return b, nil
}

// Only WriteAll is supported.  Write is not implemented on purpose because it
// might cause unexpected behaviour when appending to a file, since filesize is
// always rounded up to the next page boundary.
func (p *File) WriteAll(data []byte, intr fuse.Intr) fuse.Error {
	err := p.pakfs.Truncate(p.File.Name(), int64(len(data)))
	if err != nil {
		return errno(err)
	}

	_, err = p.WriteAt(data, 0)
	if err != nil {
		return errno(err)
	}

	return nil
}

func (p *File) Fsync(req *fuse.FsyncRequest, intr fuse.Intr) fuse.Error {
	return nil
}

func errno(err error) fuse.Error {
	if errors.Is(err, pakfs.ErrNoSpace) {
		return fuse.Errno(syscall.ENOSPC)
	} else if errors.Is(err, pakfs.ErrReadOnly) {
		return fuse.Errno(syscall.EROFS)
	} else if errors.Is(err, pakfs.ErrIsDir) {
		return fuse.Errno(syscall.EISDIR)
	} else if errors.Is(err, pakfs.ErrNameTooLong) {
		return fuse.Errno(syscall.ENAMETOOLONG)
	} else if errors.Is(err, fs.ErrInvalid) {
		return fuse.Errno(syscall.EINVAL)
	} else if errors.Is(err, fs.ErrExist) {
		return fuse.Errno(syscall.EEXIST)
	} else if errors.Is(err, fs.ErrNotExist) {
		return fuse.Errno(syscall.ENOENT)
	} else {
		return fuse.EIO
	}
}
