package pakfs

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
)

const (
	inodeLast = 1
	inodeFree = 3
)

// Implements [fs.File] and [fs.FileInfo] as well as [io.ReaderAt] and
// [io.WriterAt].
type File struct {
	io.Reader

	fs   *FS
	note *note
	off  int64
}

func newFile(fs *FS, noteIdx int) (f *File) {
	f = &File{
		fs:   fs,
		note: &fs.notes[noteIdx],
		off:  noteOffset(fs.id.BankCount, noteIdx),
	}
	f.Reader = io.NewSectionReader(f, 0, 1<<63-1)
	return
}

func (f *File) pages() ([]uint16, error) {
	pages := []uint16{}
	page := f.note.StartPage
	if page == 0 {
		return pages, nil
	}
	for page != inodeLast {
		if !f.fs.validPage(page) {
			return nil, ErrInconsistent
		}
		pages = append(pages, page)
		page = f.fs.inodes[page]
	}
	return pages, nil
}

// Returns the pages of a note which contain a section of the note with offset
// `off` and length `n`. If the section extends beyond EOF, pagesEOF is the
// number of pages to be allocated.
func (f *File) section(off int64, n int64) (pages []uint16, pagesEOF int, err error) {
	if off < 0 {
		err = fs.ErrInvalid
		return
	}
	if n <= 0 {
		return
	}

	pages, err = f.pages()
	if err != nil {
		return
	}

	startIdx := int(off >> pageBits)
	endIdx := int((off + n + pageMask) >> pageBits)
	if endIdx > len(pages) {
		pagesEOF = endIdx - len(pages)
		endIdx = len(pages)
		startIdx = min(startIdx, endIdx)
	}

	pages = pages[startIdx:endIdx]

	return
}

func (f *File) allocPages(pageCnt int) (err error) {
	dev, ok := f.fs.dev.(io.WriterAt)
	if !ok {
		return ErrReadOnly
	}

	newPages := make([]uint16, 0)
	for page, inode := range inodes(f.fs) {
		if inode == inodeFree {
			newPages = append(newPages, uint16(page))
		}
		if len(newPages) >= pageCnt {
			break
		}
	}

	if len(newPages) < pageCnt {
		return ErrNoSpace
	}

	pages, err := f.pages()
	if err != nil {
		return
	}

	for i, page := range newPages[:len(newPages)-1] {
		f.fs.inodes[page] = newPages[i+1]
	}
	f.fs.inodes[newPages[len(newPages)-1]] = inodeLast
	if len(pages) == 0 {
		f.note.StartPage = newPages[0]
		err = f.sync()
		if err != nil {
			return
		}
	} else {
		f.fs.inodes[pages[len(pages)-1]] = newPages[0]
	}

	// write zeroes to new pages
	var buf [pageSize]byte
	for _, v := range newPages {
		pageAddr := int64(v) << pageBits
		_, err = dev.WriteAt(buf[:], pageAddr)
		if err != nil {
			return
		}
	}

	return f.fs.sync()
}

func (f *File) freePages(pageCnt int) (err error) {
	pages, err := f.pages()
	if err != nil {
		return
	}

	pageCnt = min(pageCnt, len(pages))
	for _, page := range pages[len(pages)-pageCnt:] {
		f.fs.inodes[page] = inodeFree
	}
	pages = pages[:len(pages)-pageCnt]

	if len(pages) == 0 {
		f.note.StartPage = inodeLast
		err = f.sync()
		if err != nil {
			return
		}
	} else {
		f.fs.inodes[pages[len(pages)-1]] = inodeLast
	}

	return f.fs.sync()
}

// Write game note back to disk.
func (f *File) sync() (err error) {
	dev, ok := f.fs.dev.(io.WriterAt)
	if !ok {
		return ErrReadOnly
	}

	ow := io.NewOffsetWriter(dev, f.off)
	err = binary.Write(ow, binary.BigEndian, f.note)
	if err != nil {
		return
	}

	return
}

func (f *File) ReadAt(b []byte, off int64) (n int, err error) {
	f.fs.mtx.RLock()
	defer f.fs.mtx.RUnlock()

	pages, pagesEOF, err := f.section(off, int64(len(b)))
	if err != nil {
		return
	}
	if pagesEOF > 0 {
		err = io.EOF
	}

	pageOff := off & pageMask
	for _, v := range pages {
		pageAddr := int64(v) << pageBits
		l := min(pageSize-int(pageOff), len(b[n:]))
		written, err := f.fs.dev.ReadAt(b[n:n+l], pageAddr+pageOff)
		n += written
		if err != nil {
			return n, err
		}

		pageOff = 0
	}

	return
}

func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	f.fs.mtx.Lock()
	defer f.fs.mtx.Unlock()

	dev, ok := f.fs.dev.(io.WriterAt)
	if !ok {
		return 0, ErrReadOnly
	}

	pages, pagesEOF, err := f.section(off, int64(len(b)))
	if err != nil {
		return
	}

	if pagesEOF > 0 {
		err = f.allocPages(pagesEOF)
		if err != nil {
			return
		}
		pages, _, err = f.section(off, int64(len(b)))
		if err != nil {
			return
		}
	}

	pageOff := off & pageMask
	for _, v := range pages {
		pageAddr := int64(v) << pageBits
		l := min(pageSize-int(pageOff), len(b[n:]))
		written, err := dev.WriteAt(b[n:n+l], pageAddr+pageOff)
		n += written
		if err != nil {
			return n, err
		}

		pageOff = 0
	}

	return
}

// FIXME this can result in the same filename for two different notes, if the
// extension was stored in note.FileName by another pakfs implementation.
func (f *File) name() (s string) {
	for _, v := range [...][]byte{f.note.Extension[:], f.note.FileName[:]} {
		// filename is null terminated
		null := bytes.IndexByte(v, 0)
		if null == -1 {
			null = len(v)
		}

		decoder := N64FontCodeStrict.NewDecoder()
		vs, _ := decoder.String(string(v[:null]))
		if s == "" {
			s = vs
		} else {
			s = strings.Join([]string{vs, s}, ".")
		}
	}

	return
}

func (f *File) setName(filename string) error {
	extension := path.Ext(filename)
	if extension != "." {
		filename = strings.TrimSuffix(filename, extension)
	}
	extension = strings.TrimPrefix(extension, ".")

	for _, v := range [...]struct {
		dst []byte
		src string
	}{
		{f.note.FileName[:], filename},
		{f.note.Extension[:], extension},
	} {
		s, err := N64FontCodeStrict.NewEncoder().String(v.src)
		if err != nil {
			return err
		}
		if len(s) > len(v.dst[:]) {
			return ErrNameTooLong
		}
		n := copy(v.dst[:], s)
		for i := range v.dst[n:] {
			v.dst[n+i] = 0
		}
	}

	return nil
}

// Returns the ASCII encoded company code of this file.
func (f *File) CompanyCode() [2]byte {
	f.fs.mtx.RLock()
	defer f.fs.mtx.RUnlock()

	return f.note.PublisherCode
}

// Writes the ASCII encoded company code of this file.
func (f *File) SetCompanyCode(code [2]byte) error {
	f.fs.mtx.Lock()
	defer f.fs.mtx.Unlock()

	f.note.PublisherCode = code
	return f.sync()
}

// Returns the ASCII encoded game code of this file.
func (f *File) GameCode() [4]byte {
	f.fs.mtx.RLock()
	defer f.fs.mtx.RUnlock()

	return f.note.GameCode
}

// Writes the ASCII encoded game code of this file.
func (f *File) SetGameCode(code [4]byte) error {
	f.fs.mtx.Lock()
	defer f.fs.mtx.Unlock()

	f.note.GameCode = code
	return f.sync()
}

// fs.File implementation

func (f *File) Stat() (fs.FileInfo, error) { return f, nil }
func (f *File) Close() error               { return nil }

// fs.FileInfo implementation

func (f *File) Name() (s string) {
	f.fs.mtx.RLock()
	defer f.fs.mtx.RUnlock()

	return f.name()
}
func (f *File) Size() int64 {
	f.fs.mtx.RLock()
	defer f.fs.mtx.RUnlock()

	pages, err := f.pages()
	if err != nil {
		return 0
	}
	return int64(len(pages) << pageBits)
}
func (f *File) Mode() fs.FileMode  { return 0666 }
func (f *File) ModTime() time.Time { return time.Time{} }
func (f *File) IsDir() bool        { return f.Mode().IsDir() }
func (f *File) Sys() any           { return nil }
