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
		d.entries = d.fs.Root()
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
func (d *rootDir) ModTime() time.Time { return time.Unix(0, 0) }
func (d *rootDir) IsDir() bool        { return true }
func (d *rootDir) Sys() any           { return nil }

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

func (f *File) Stat() (fs.FileInfo, error) { return f, nil }
func (f *File) Close() error               { return nil }

func (f *File) ReadAt(b []byte, off int64) (n int, err error) {
	f.fs.mtx.RLock()
	defer f.fs.mtx.RUnlock()

	return f.fs.readAt(f.noteIdx, b, off)
}

func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	f.fs.mtx.Lock()
	defer f.fs.mtx.Unlock()

	return f.fs.writeAt(f.noteIdx, b, off)
}

func (p *File) Name() (s string) {
	p.fs.mtx.RLock()
	defer p.fs.mtx.RUnlock()

	return p.name()
}

func (p *File) name() (s string) {
	note := p.fs.notes[p.noteIdx]

	for _, v := range [...][]byte{note.Extension[:], note.FileName[:]} {
		// filename is null terminated
		null := bytes.IndexByte(v, 0)
		if null == -1 {
			null = len(v)
		}

		decoder := N64FontCode.NewDecoder()
		vs, _ := decoder.String(string(v[:null]))
		if s == "" {
			s = vs
		} else {
			s = strings.Join([]string{vs, s}, ".")
		}
	}

	return
}

func (p *File) Size() int64 {
	p.fs.mtx.RLock()
	defer p.fs.mtx.RUnlock()

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

func (p *File) CompanyCode() [2]byte {
	p.fs.mtx.RLock()
	defer p.fs.mtx.RUnlock()

	return p.fs.notes[p.noteIdx].PublisherCode
}

func (p *File) SetCompanyCode(code [2]byte) error {
	p.fs.mtx.Lock()
	defer p.fs.mtx.Unlock()

	p.fs.notes[p.noteIdx].PublisherCode = code
	return p.fs.writeNote(p.noteIdx)
}

func (p *File) GameCode() [4]byte {
	p.fs.mtx.RLock()
	defer p.fs.mtx.RUnlock()

	return p.fs.notes[p.noteIdx].GameCode
}

func (p *File) SetGameCode(code [4]byte) error {
	p.fs.mtx.Lock()
	defer p.fs.mtx.Unlock()

	p.fs.notes[p.noteIdx].GameCode = code
	return p.fs.writeNote(p.noteIdx)
}

// Holds only the filename and tries to open it on Info().  This resembles the
// behavoiur of the os package, at least on linux ext4.  fs.FileInfoToDirEntry
// shouldn't be used create dir entries on writable filesystems, because Name()
// will fail if the underlying file is (re)moved.
type dirEntry struct {
	fs   *FS
	name string
}

func (p *dirEntry) Name() string      { return p.name }
func (p *dirEntry) IsDir() bool       { return p.Type().IsDir() }
func (p *dirEntry) Type() fs.FileMode { return 0666 }

func (p *dirEntry) Info() (fs.FileInfo, error) {
	f, err := p.fs.Open(p.name)
	if err != nil {
		return nil, err
	}
	return f.Stat()
}
