package pakfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"math"
	"path"
	"strings"
)

var ErrInconsistent = errors.New("damaged filesystem")
var ErrNoSpace = errors.New("no space left on device")
var ErrReadOnly = errors.New("read-only file system")
var ErrIsDir = errors.New("is a directory")
var ErrNameTooLong = errors.New("file name too long")

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

type FS struct {
	dev io.ReaderAt

	id     idSector
	inodes iNodes
	notes  [noteCnt]note
}

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
	offset, n := notesOffset(fs.id.BankCount, 0)
	sr := io.NewSectionReader(dev, offset, n)
	err = binary.Read(sr, binary.BigEndian, &fs.notes)
	if err != nil {
		return nil, err
	}

	return fs, nil
}

func (p *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	if name == "." {
		return p.Root(), nil
	}

	for i := range p.notes {
		if p.notes[i].StartPage == 0 {
			continue
		}
		f := newFile(p, i)
		if name == f.Name() {
			return f, nil
		}
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (p *FS) Label() string {
	label := [blockLen]byte{}
	_, err := p.dev.ReadAt(label[:], baseLabel)
	if err != nil {
		return ""
	}
	return string(label[:])
}

func (p *FS) Root() rootDir {
	root := make([]fs.DirEntry, 0, noteCnt)
	for i := range p.notes {
		if p.notes[i].StartPage == 0 {
			continue
		}
		root = append(root, fs.FileInfoToDirEntry(&File{
			fs:      p,
			noteIdx: i,
		}))
	}
	return root
}

func (p *FS) Size() int64 {
	totalPages := int64(len(p.inodes)) - int64(p.id.BankCount) - int64(p.id.BankCount<<1) - 2
	return totalPages << pageBits
}

func (p *FS) Free() int64 {
	freePages := 0
	inodes(p)(func(page int, inode uint16) bool {
		if inode == inodeFree {
			freePages += 1
		}
		return true
	})
	return int64(freePages << pageBits)
}

func (p *FS) Create(name string) (*File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrInvalid}
	}
	if path.Dir(name) != "." {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrNotExist}
	}

	_, err := p.Open(name)
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

	filename := name
	extension := path.Ext(filename)
	if extension != "." {
		filename = strings.TrimSuffix(filename, extension)
	}
	extension = strings.TrimPrefix(extension, ".")

	note := &p.notes[noteIdx]
	for _, v := range [...]struct {
		dst []byte
		src string
	}{
		{note.FileName[:], filename},
		{note.Extension[:], extension},
	} {
		s, err := N64FontCode.NewEncoder().String(v.src)
		if err != nil {
			return nil, &fs.PathError{Op: "create", Path: name, Err: err}
		}
		if len(s) > len(v.dst[:]) {
			return nil, &fs.PathError{Op: "create", Path: name, Err: ErrNameTooLong}
		}
		n := copy(v.dst[:], s)
		for i := range v.dst[n:] {
			v.dst[n+i] = 0
		}
	}

	note.StartPage = inodeLast
	note.Status = 0x2

	p.writeNote(noteIdx)

	return newFile(p, noteIdx), nil
}

func (p *FS) Remove(name string) (err error) {
	fd, err := p.Open(name)
	if err != nil {
		return
	}

	f, ok := fd.(*File)
	if !ok {
		// If not a file this must be the root directory
		return &fs.PathError{Op: "remove", Path: name, Err: ErrIsDir}
	}

	err = p.freePages(f.noteIdx, math.MaxInt)
	if err != nil {
		return
	}

	p.notes[f.noteIdx] = note{}
	err = p.writeNote(f.noteIdx)
	return
}

func (p *FS) Truncate(name string, size int64) (err error) {
	fd, err := p.Open(name)
	if err != nil {
		return
	}

	f, ok := fd.(*File)
	if !ok {
		// If not a file this must be the root directory
		return &fs.PathError{Op: "truncate", Path: name, Err: ErrIsDir}
	}

	pages, err := p.notePages(f.noteIdx)
	if err != nil {
		return &fs.PathError{Op: "truncate", Path: name, Err: err}
	}

	pageDelta := int((size+pageMask)>>pageBits) - len(pages)
	if pageDelta > 0 {
		p.allocPages(f.noteIdx, pageDelta)
	} else if pageDelta < 0 {
		p.freePages(f.noteIdx, -pageDelta)
	}

	return
}

// Returns the pages of a note which contain a section of the note with offset
// `off` and length `n`.  If the section extends beyond EOF, pagesEOF is the
// number of pages to be allocated.
func (p *FS) notePagesSection(noteIdx int, off int64, n int64) (pages []uint16, pagesEOF int, err error) {
	if off < 0 {
		err = fs.ErrInvalid
		return
	}

	pages, err = p.notePages(noteIdx)
	if err != nil {
		return
	}

	startIdx := int(off >> pageBits)
	endIdx := int((off+n)>>pageBits) + 1
	if endIdx > len(pages) {
		pagesEOF = endIdx - len(pages)
		endIdx = len(pages)
		startIdx = min(startIdx, endIdx)
	}

	pages = pages[startIdx:endIdx]

	return
}

func (p *FS) readAt(noteIdx int, b []byte, off int64) (n int, err error) {
	pages, pagesEOF, err := p.notePagesSection(noteIdx, off, int64(len(b)))
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
		written, err := p.dev.ReadAt(b[n:n+l], pageAddr+pageOff)
		n += written
		if err != nil {
			return n, err
		}

		pageOff = 0
	}

	return
}

func (p *FS) writeAt(noteIdx int, b []byte, off int64) (n int, err error) {
	dev, ok := p.dev.(io.WriterAt)
	if !ok {
		return 0, ErrReadOnly
	}

	pages, pagesEOF, err := p.notePagesSection(noteIdx, off, int64(len(b)))
	if err != nil {
		return
	}

	if pagesEOF > 0 {
		err = p.allocPages(noteIdx, pagesEOF)
		if err != nil {
			return
		}
		pages, _, err = p.notePagesSection(noteIdx, off, int64(len(b)))
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

func (p *FS) notePages(noteIdx int) ([]uint16, error) {
	pages := []uint16{}
	page := p.notes[noteIdx].StartPage
	if page == 0 {
		return pages, nil
	}
	for page != inodeLast {
		if !p.validPage(page) {
			return nil, ErrInconsistent
		}
		pages = append(pages, page)
		page = p.inodes[page]
	}
	return pages, nil
}

func (p *FS) firstPage() int {
	return 1 + int(p.id.BankCount)<<1 + 2
}

func (p *FS) validPage(page uint16) bool {
	return !(page < uint16(p.firstPage()) ||
		page >= uint16(len(p.inodes)) ||
		page&pageMask == 0)
}

func (p *FS) allocPages(noteIdx int, pageCnt int) (err error) {
	newPages := make([]uint16, 0)
	inodes(p)(func(page int, inode uint16) bool {
		if inode == inodeFree {
			newPages = append(newPages, uint16(page))
		}
		if len(newPages) >= pageCnt {
			return false
		}
		return true
	})

	if len(newPages) < pageCnt {
		return ErrNoSpace
	}

	pages, err := p.notePages(noteIdx)
	if err != nil {
		return
	}

	for i, page := range newPages[:len(newPages)-1] {
		p.inodes[page] = newPages[i+1]
	}
	p.inodes[newPages[len(newPages)-1]] = inodeLast
	p.inodes[pages[len(pages)-1]] = newPages[0]

	if err = p.writeINodes(); err != nil {
		return
	}

	return nil
}

func (p *FS) freePages(noteIdx int, pageCnt int) (err error) {
	pages, err := p.notePages(noteIdx)
	if err != nil {
		return
	}

	pageCnt = min(pageCnt, len(pages))
	for _, page := range pages[len(pages)-pageCnt:] {
		p.inodes[page] = inodeFree
	}
	pages = pages[:len(pages)-pageCnt]

	if len(pages) == 0 {
		p.notes[noteIdx].StartPage = inodeLast
		err = p.writeNote(noteIdx)
		if err != nil {
			return
		}
	} else {
		p.inodes[pages[len(pages)-1]] = inodeLast
	}

	err = p.writeINodes()
	return
}

// Write inodes back to disk.  Also updates the inode backup.
func (p *FS) writeINodes() (err error) {
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

// Write game note back to disk.
func (p *FS) writeNote(noteIdx int) (err error) {
	dev, ok := p.dev.(io.WriterAt)
	if !ok {
		return ErrReadOnly
	}

	off, _ := notesOffset(p.id.BankCount, noteIdx)
	ow := io.NewOffsetWriter(dev, off)
	err = binary.Write(ow, binary.BigEndian, &p.notes[noteIdx])
	if err != nil {
		return
	}

	return
}

func (p *FS) iNodesChecksum(update bool) (valid bool) {
	valid = true
	var csum uint16
	inodes(p)(func(page int, inode uint16) bool {
		csum += inode
		if (page+1)%pagesPerBank == 0 { // last page in this bank
			csumIdx := page &^ (pagesPerBank - 1)
			if csum&0xff != p.inodes[csumIdx]&0xff {
				valid = false
				if update {
					p.inodes[csumIdx] = csum&0xff | p.inodes[csumIdx]&0xff00
				} else {
					return false
				}
			}
			csum = 0
		}
		return true
	})
	return
}

// rangefunc for iterating inodes
// TODO use range syntax at callsites after updating to Go1.23
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

func notesOffset(bankCount uint8, noteIdx int) (offset, n int64) {
	offset = (1 + int64(bankCount)<<1) << pageBits
	offset += int64(noteIdx << noteBits)
	n = int64(2) << pageBits
	return
}
