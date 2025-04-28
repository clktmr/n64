package pakfs

import (
	"io"
	"io/fs"
	"time"
)

// pakfs doesn't support subdirectories, only the root dir exists.
type rootDir struct {
	fs      *FS
	entries []fs.DirEntry
}

// fs.File implementation
func (d *rootDir) Stat() (fs.FileInfo, error) { return d, nil }
func (d *rootDir) Read(p []byte) (int, error) { return 0, fs.ErrInvalid }
func (d *rootDir) Close() error               { return nil }

func (d *rootDir) ReadDir(n int) (root []fs.DirEntry, err error) {
	if d.entries == nil {
		d.entries = d.fs.ReadDirRoot()
	}

	cut := len(d.entries)
	if n > 0 {
		if n >= cut {
			err = io.EOF
			n = cut
		}
		cut = n
	}
	root = d.entries[:cut]
	d.entries = d.entries[cut:]

	return
}

// fs.FileInfo implementation
func (d *rootDir) Name() string       { return "." }
func (d *rootDir) Size() int64        { return 0 }
func (d *rootDir) Mode() fs.FileMode  { return fs.ModeDir | 0777 }
func (d *rootDir) ModTime() time.Time { return time.Time{} }
func (d *rootDir) IsDir() bool        { return true }
func (d *rootDir) Sys() any           { return nil }

// Holds only the filename and tries to open it on Info(). This resembles the
// behavoiur of the os package, at least on linux ext4. fs.FileInfoToDirEntry
// shouldn't be used create dir entries on writable filesystems, because Name()
// will fail if the underlying file is (re)moved.
type dirEntry struct {
	fs   *FS
	name string
}

func (p *dirEntry) Name() string      { return p.name }
func (p *dirEntry) IsDir() bool       { return p.Type().IsDir() }
func (p *dirEntry) Type() fs.FileMode { return 0 }

func (p *dirEntry) Info() (fs.FileInfo, error) {
	f, err := p.fs.Open(p.name)
	if err != nil {
		return nil, err
	}
	return f.Stat()
}
