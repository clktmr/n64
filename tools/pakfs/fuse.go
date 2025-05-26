//go:build linux || darwin

package pakfs

import (
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/clktmr/n64/drivers/controller/pakfs"
	"rsc.io/rsc/fuse"
)

func mount(image, dir string) error {
	c, err := fuse.Mount(dir)
	if err != nil {
		return err
	}
	r, err := os.OpenFile(image, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	fs, err := pakfs.Read(r)
	if err != nil {
		return err
	}

	go c.Serve(&fusefs{fs})
	<-sigintr

	cmd := exec.Command("/bin/umount", dir)
	_, err = cmd.CombinedOutput()
	return err
}

// fusefs implements the file system and the root dir Node.
type fusefs struct {
	pakfs *pakfs.FS
}

func (p *fusefs) Root() (fuse.Node, fuse.Error) {
	return p, nil
}

func (p *fusefs) Attr() fuse.Attr {
	dir := p.pakfs.Root()
	stat, err := dir.Stat()
	if err != nil {
		log.Println("stat:", err)
		return fuse.Attr{}
	}
	return fuse.Attr{
		Mode:  stat.Mode(),
		Mtime: stat.ModTime(),
	}
}

func (p *fusefs) Lookup(name string, intr fuse.Intr) (fuse.Node, fuse.Error) {
	f, err := p.pakfs.Open(name)
	if err != nil {
		return nil, errno(err)
	}
	pakfile, ok := f.(*pakfs.File)
	if !ok {
		return p, nil // must be root dir
	}
	return &fusefile{pakfile, p.pakfs}, nil
}

func (p *fusefs) ReadDir(intr fuse.Intr) ([]fuse.Dirent, fuse.Error) {
	entries := p.pakfs.ReadDirRoot()
	fuseEntries := make([]fuse.Dirent, len(entries))
	for i, v := range entries {
		fuseEntries[i] = fuse.Dirent{
			Name: v.Name(),
		}
	}

	return fuseEntries, nil
}

func (p *fusefs) Create(req *fuse.CreateRequest, res *fuse.CreateResponse, intr fuse.Intr) (fuse.Node, fuse.Handle, fuse.Error) {
	f, err := p.pakfs.Create(req.Name)
	if err != nil {
		return nil, nil, errno(err)
	}

	file := &fusefile{f, p.pakfs}
	return file, file, nil
}

func (p *fusefs) Remove(req *fuse.RemoveRequest, intr fuse.Intr) fuse.Error {
	err := p.pakfs.Remove(req.Name)
	if err != nil {
		return errno(err)
	}
	return nil
}

func (p *fusefs) Rename(req *fuse.RenameRequest, newDir fuse.Node, intr fuse.Intr) fuse.Error {
	err := p.pakfs.Rename(req.OldName, req.NewName)
	if err != nil {
		return errno(err)
	}
	return nil
}

// fusefile implements both Node and Handle.
type fusefile struct {
	*pakfs.File

	pakfs *pakfs.FS
}

func (p *fusefile) Attr() fuse.Attr {
	return fuse.Attr{
		Mode:  p.Mode(),
		Mtime: p.ModTime(),
		Size:  uint64(p.Size()),
	}
}

func (p *fusefile) ReadAll(intr fuse.Intr) ([]byte, fuse.Error) {
	b := make([]byte, p.Size())
	_, err := p.ReadAt(b, 0)
	if err != io.EOF && err != nil {
		return nil, errno(err)
	}
	return b, nil
}

// Only WriteAll is supported. Write is not implemented on purpose because it
// might cause unexpected behaviour when appending to a file, since filesize is
// always rounded up to the next page boundary.
func (p *fusefile) WriteAll(data []byte, intr fuse.Intr) fuse.Error {
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

func (p *fusefile) Fsync(req *fuse.FsyncRequest, intr fuse.Intr) fuse.Error {
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
