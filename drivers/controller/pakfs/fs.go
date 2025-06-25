// Package pakfs implements access the Controller Pak's filesystem.
//
// The Controller Pak supports exactly sixteen files (aka notes) in a root
// directory. Each file has a GameCode and a PublisherCode, which aren't part of
// their name. The name can be at most sixteen characters long with a very
// limited character set (see [N64FontCode]). Additionally each file has a four
// character extension, which is part of the files name and was usually used to
// distinguish multiple savegames for the same game.
//
// Another peculiarity of pakfs files is their size, which can only be a
// multiple of the pagesize of 256 byte. Appending to a file will most probably
// have undesired effects.
package pakfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"math"
	"path"
	"sync"
)

// Errors returned by pakfs.
var (
	ErrInconsistent = errors.New("damaged filesystem")
	ErrNoSpace      = errors.New("no space left on device")
	ErrReadOnly     = errors.New("read-only file system")
	ErrIsDir        = errors.New("is a directory")
	ErrNameTooLong  = errors.New("file name too long")
)

const (
	pagesPerBankBits = 7
	pagesPerBank     = 1 << pagesPerBankBits
)
const (
	pageBits = 8
	pageSize = 1 << pageBits
	pageMask = pageSize - 1
)

const blockLen = 32
const (
	baseLabel     = 0x0000
	baseID        = 0x0020
	baseIDBackup1 = 0x0060
	baseIDBackup2 = 0x0080
	baseIDBackup3 = 0x00c0
)

const (
	noteCnt  = 16
	noteBits = 5
)

type idSector struct {
	Repaired    uint32
	Random      uint32
	Serial      [16]byte
	DeviceId    uint16
	BankCount   uint8
	Version     uint8
	Checksum    uint16
	ChecksumInv uint16
}

func (s idSector) checksum() (csum uint16, csumInv uint16) {
	const csumSize = 4
	buf := bytes.NewBuffer(make([]byte, blockLen))
	binary.Write(buf, binary.BigEndian, s)
	for i := 0; i < buf.Len()-csumSize; i += 2 {
		csum += uint16(buf.Bytes()[i])<<8 | uint16(buf.Bytes()[i+1])
	}
	csumInv = 0xfff2 - csum
	return
}

func (s idSector) valid() bool {
	csum, csumInv := s.checksum()
	return csum == s.Checksum && csumInv == s.ChecksumInv
}

type iNodes []uint16

// One of 16 game notes that the pak can store.
type note struct {
	GameCode      [4]byte
	PublisherCode [2]byte
	StartPage     uint16
	Status        uint8
	_             uint8
	_             uint16
	Extension     [4]byte
	FileName      [16]byte
}

// FS implements [fs.FS] for the Controller Pak's filesystem to read and write
// savegames from it.
type FS struct {
	mtx sync.RWMutex
	dev io.ReaderAt

	id     idSector
	inodes iNodes
	notes  [noteCnt]note
}

// Read opens an existing pakfs.
func Read(dev io.ReaderAt) (fs *FS, err error) {
	fs = &FS{dev: dev}

	for _, base := range [...]int64{baseID, baseIDBackup1, baseIDBackup2, baseIDBackup3} {
		r := io.NewSectionReader(dev, base, blockLen)
		err := binary.Read(r, binary.BigEndian, &fs.id)
		if err != nil {
			return nil, err
		}

		if fs.id.valid() {
			goto validId
		}
	}
	return nil, ErrInconsistent

validId:
	for _, offsetFunc := range [...]func(uint8) (int64, int64){iNodesOffset, iNodesBakOffset} {
		offset, n := offsetFunc(fs.id.BankCount)
		r := io.NewSectionReader(dev, offset, n)
		fs.inodes = make(iNodes, r.Size()>>1)
		err = binary.Read(r, binary.BigEndian, &fs.inodes)
		if err != nil {
			return nil, err
		}

		if fs.iNodesChecksum(false) {
			goto validINodes
		}
	}
	return nil, ErrInconsistent

validINodes:
	offset := noteOffset(fs.id.BankCount, 0)
	sr := io.NewSectionReader(dev, offset, int64(2)<<pageBits)
	err = binary.Read(sr, binary.BigEndian, &fs.notes)
	if err != nil {
		return nil, err
	}

	return fs, nil
}

// Open opens the named file for reading.
func (p *FS) Open(name string) (fs.File, error) {
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	return p.open(name)
}

func (p *FS) open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	if name == "." {
		return &rootDir{p, nil}, nil
	}

	for i, note := range p.notes {
		if note.StartPage == 0 {
			continue
		}
		f := newFile(p, i)
		if name == f.name() {
			return f, nil
		}
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// Label returns the filesystem label. Usually it doesn't contain useful data.
func (p *FS) Label() string {
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	label := [blockLen]byte{}
	_, err := p.dev.ReadAt(label[:], baseLabel)
	if err != nil {
		return ""
	}
	return string(label[:])
}

// Root returns the root directory. There are no other directories in a pakfs.
func (p *FS) Root() fs.ReadDirFile {
	return &rootDir{p, nil}
}

// ReadDirRoot returns the list of files in the root directory.
func (p *FS) ReadDirRoot() []fs.DirEntry {
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	root := make([]fs.DirEntry, 0, noteCnt)
	for i, note := range p.notes {
		if note.StartPage == 0 {
			continue
		}
		f := newFile(p, i)
		root = append(root, &dirEntry{p, f.name()})
	}
	return root
}

// Size returns the total available storage for file data in bytes.
func (p *FS) Size() int64 {
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	totalPages := int64(len(p.inodes)) - int64(p.id.BankCount) - int64(p.id.BankCount<<1) - 2
	return totalPages << pageBits
}

// Free returns the unused storage for file data in bytes.
func (p *FS) Free() int64 {
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	freePages := 0
	for _, inode := range inodes(p) {
		if inode == inodeFree {
			freePages += 1
		}
	}
	return int64(freePages << pageBits)
}

// Create creates the named file.
func (p *FS) Create(name string) (*File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrInvalid}
	}
	if path.Dir(name) != "." {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrNotExist}
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()

	_, err := p.open(name)
	if err == nil {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrExist}
	}

	var noteIdx int
	for noteIdx = range p.notes {
		if p.notes[noteIdx].StartPage == 0 {
			goto freeNote
		}
	}

	return nil, &fs.PathError{Op: "create", Path: name, Err: ErrNoSpace}

freeNote:

	f := newFile(p, noteIdx)
	err = f.setName(name)
	if err != nil {
		return nil, &fs.PathError{Op: "create", Path: name, Err: err}
	}

	f.note.StartPage = inodeLast
	f.note.Status = 0x2

	if err = f.sync(); err != nil {
		return nil, err
	}

	return f, nil
}

// Remove deletes the named file.
func (p *FS) Remove(name string) (err error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	return p.remove(name)
}

func (p *FS) remove(name string) (err error) {
	fd, err := p.open(name)
	if err != nil {
		return
	}

	f, ok := fd.(*File)
	if !ok {
		// If not a file this must be the root directory
		return &fs.PathError{Op: "remove", Path: name, Err: ErrIsDir}
	}

	if err = f.freePages(math.MaxInt); err != nil {
		return
	}

	*f.note = note{}
	err = f.sync()
	return
}

// Rename renames the file from oldpath to newpath.
func (p *FS) Rename(oldpath, newpath string) (err error) {
	if oldpath == newpath {
		return
	}

	if path.Dir(newpath) != "." {
		return &fs.PathError{Op: "rename", Path: oldpath, Err: fs.ErrNotExist}
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()

	fd, err := p.open(oldpath)
	if err != nil {
		return
	}

	f, ok := fd.(*File)
	if !ok {
		// If not a file this must be the root directory
		return &fs.PathError{Op: "rename", Path: oldpath, Err: ErrIsDir}
	}

	// Remove replaced note if we overwrite one
	err = p.remove(newpath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return &fs.PathError{Op: "rename", Path: oldpath, Err: err}
	}

	return f.setName(newpath)
}

// Truncate changes the size of the file. Note that size will always round up to
// a multiple of the pagesize, i.e. 256 byte.
func (p *FS) Truncate(name string, size int64) (err error) {
	dev, ok := p.dev.(io.WriterAt)
	if !ok {
		return ErrReadOnly
	}

	if size < 0 {
		return fs.ErrInvalid
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()

	fd, err := p.open(name)
	if err != nil {
		return
	}

	f, ok := fd.(*File)
	if !ok {
		// If not a file this must be the root directory
		return &fs.PathError{Op: "truncate", Path: name, Err: ErrIsDir}
	}

	pages, err := f.pages()
	if err != nil {
		return &fs.PathError{Op: "truncate", Path: name, Err: err}
	}

	pageDelta := int((size+pageMask)>>pageBits) - len(pages)
	if pageDelta > 0 {
		err = f.allocPages(pageDelta)
	} else {
		err = f.freePages(-pageDelta)
		if err != nil {
			return
		}

		lastPageIdx := len(pages) - 1 + pageDelta
		if lastPageIdx >= 0 {
			// write zeroes from `size` to end of last page
			pageAddr := int64(pages[lastPageIdx]) << pageBits
			zeroes := make([]byte, pageSize-(size&pageMask))
			_, err = dev.WriteAt(zeroes, pageAddr+(size&pageMask))
		}
	}

	return
}

// Write inodes back to disk. Also updates the inode backup.
func (p *FS) sync() (err error) {
	dev, ok := p.dev.(io.WriterAt)
	if !ok {
		return ErrReadOnly
	}

	p.iNodesChecksum(true)

	for _, offsetFunc := range [...]func(uint8) (int64, int64){iNodesOffset, iNodesBakOffset} {
		offset, _ := offsetFunc(p.id.BankCount)
		iNodesWriter := io.NewOffsetWriter(dev, offset)
		err = binary.Write(iNodesWriter, binary.BigEndian, p.inodes)
		if err != nil {
			return
		}
	}

	return
}

func (p *FS) firstPage() int {
	return 1 + int(p.id.BankCount)<<1 + 2
}

func (p *FS) validPage(page uint16) bool {
	return !(page < uint16(p.firstPage()) ||
		page >= uint16(len(p.inodes)) ||
		page&pageMask == 0)
}

func (p *FS) iNodesChecksum(update bool) (valid bool) {
	valid = true
	var csum uint16
	for page, inode := range inodes(p) {
		csum += inode
		if (page+1)%pagesPerBank == 0 { // last page in this bank
			csumIdx := page &^ (pagesPerBank - 1)
			if csum&0xff != p.inodes[csumIdx]&0xff {
				valid = false
				if update {
					p.inodes[csumIdx] = csum&0xff | p.inodes[csumIdx]&0xff00
				} else {
					break
				}
			}
			csum = 0
		}
	}
	return
}

// rangefunc for iterating inodes
func inodes(p *FS) func(func(int, uint16) bool) {
	return func(yield func(int, uint16) bool) {
		page := p.firstPage()
		lastPage := pagesPerBank
		for range p.id.BankCount {
			for page < lastPage {
				if !yield(page, p.inodes[page]) {
					return
				}
				page += 1
			}
			page += 1 // skip csum
			lastPage += pagesPerBank
		}
	}
}

func iNodesOffset(bankCount uint8) (offset, n int64) {
	offset = int64(1 << pageBits)
	n = int64(bankCount) << pageBits
	return
}

func iNodesBakOffset(bankCount uint8) (offset, n int64) {
	offset = (1 + int64(bankCount)) << pageBits
	n = int64(bankCount) << pageBits
	return
}

func noteOffset(bankCount uint8, noteIdx int) (offset int64) {
	offset = (1 + int64(bankCount)<<1) << pageBits
	offset += int64(noteIdx << noteBits)
	return
}
