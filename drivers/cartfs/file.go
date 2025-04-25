package cartfs

import (
	"errors"
	"io"
	"io/fs"
	"time"
)

// dotFile is a file for the root directory,
// which is omitted from the files list in a FS.
var dotFile = &file{name: "./"}

type file struct {
	name   string
	size   int64
	offset int64
}

var (
	_ fs.FileInfo = (*file)(nil)
	_ fs.DirEntry = (*file)(nil)
)

func (f *file) Name() string               { _, elem, _ := split(f.name); return elem }
func (f *file) Size() int64                { return f.size }
func (f *file) ModTime() time.Time         { return time.Time{} }
func (f *file) IsDir() bool                { _, _, isDir := split(f.name); return isDir }
func (f *file) Sys() any                   { return nil }
func (f *file) Type() fs.FileMode          { return f.Mode().Type() }
func (f *file) Info() (fs.FileInfo, error) { return f, nil }

func (f *file) Mode() fs.FileMode {
	if f.IsDir() {
		return fs.ModeDir | 0555
	}
	return 0444
}

func (f *file) String() string {
	return fs.FormatFileInfo(f)
}

// An openFile is a regular file open for reading.
type openFile struct {
	*io.SectionReader
	f *file // the file itself
}

func (f *openFile) Close() error               { return nil }
func (f *openFile) Stat() (fs.FileInfo, error) { return f.f, nil }

// An openDir is a directory open for reading.
type openDir struct {
	f      *file  // the directory file itself
	files  []file // the directory contents
	offset int    // the read offset, an index into the files slice
}

func (d *openDir) Close() error               { return nil }
func (d *openDir) Stat() (fs.FileInfo, error) { return d.f, nil }

func (d *openDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.f.name, Err: errors.New("is a directory")}
}

func (d *openDir) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.files) - d.offset
	if n == 0 {
		if count <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = &d.files[d.offset+i]
	}
	d.offset += n
	return list, nil
}
