package pakfs

import (
	"bytes"
	"io"
	"io/fs"
	"strings"
	"time"
)

const (
	inodeLast = 1
	inodeFree = 3
)

// pakfs doesn't support subdirectories, only the root dir exists.
type rootDir []fs.DirEntry

// fs.File implementation
func (d rootDir) Stat() (fs.FileInfo, error) { return d, nil }
func (d rootDir) Read(p []byte) (int, error) { return 0, fs.ErrInvalid }
func (d rootDir) Close() error               { return nil }

func (d rootDir) ReadDir(n int) ([]fs.DirEntry, error) {
	// FIXME subsequent calls must return next n entries
	if n <= 0 {
		return d, nil
	}
	return d[:min(n, len(d))], nil
}

// fs.FileInfo implementation
func (d rootDir) Name() string       { return "." }
func (d rootDir) Size() int64        { return 0 }
func (d rootDir) Mode() fs.FileMode  { return fs.ModeDir | 0777 }
func (d rootDir) ModTime() time.Time { return time.Unix(0, 0) }
func (d rootDir) IsDir() bool        { return true }
func (d rootDir) Sys() any           { return nil }

// Implements fs.File
type File struct {
	io.Reader

	fs      *FS
	noteIdx int
}

func newFile(fs *FS, noteIdx int) (f *File) {
	f = &File{
		fs:      fs,
		noteIdx: noteIdx,
	}
	f.Reader = io.NewSectionReader(f, 0, 1<<63-1)
	return
}

func (f *File) Stat() (fs.FileInfo, error)                    { return f, nil }
func (f *File) ReadAt(b []byte, off int64) (n int, err error) { return f.fs.readAt(f.noteIdx, b, off) }
func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	return f.fs.writeAt(f.noteIdx, b, off)
}
func (f *File) Close() error { return nil }

func (p *File) Name() (s string) {
	note := p.fs.notes[p.noteIdx]

	for _, v := range [...][]byte{note.FileName[:], note.Extension[:]} {
		// filename is null terminated
		null := bytes.IndexByte(v, 0)
		if null == 0 {
			continue
		} else if null == -1 {
			null = len(v)
		}

		decoder := N64FontCode.NewDecoder()
		vs, _ := decoder.String(string(v[:null]))
		if s == "" {
			s = vs
		} else {
			s = strings.Join([]string{s, vs}, ".")
		}
	}

	return
}

func (p *File) Size() int64 {
	pages, err := p.fs.notePages(p.noteIdx)
	if err != nil {
		return 0
	}
	return int64(len(pages) << pageBits)
}

func (p *File) Mode() fs.FileMode  { return 0666 }
func (p *File) ModTime() time.Time { return time.Unix(0, 0) }
func (p *File) IsDir() bool        { return p.Mode().IsDir() }
func (p *File) Sys() any           { return nil }
