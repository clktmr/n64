package pakfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"strings"
)

var ErrInconsistent = errors.New("damaged filesystem")
var ErrInvalidOffset = errors.New("invalid offset")

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

const noteCnt = 16

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

func (s iNodes) valid(firstPage int) bool {
	for lastPage := pagesPerBank; lastPage <= len(s); lastPage += pagesPerBank {
		var csum uint16
		for _, v := range s[firstPage:lastPage] {
			csum += v
		}
		if csum&0xff != s[0]&0xff {
			return false
		}

		firstPage = 1
	}
	return true
}

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
	for _, r := range [...]*io.SectionReader{iNodesReader(dev, fs.id.BankCount), iNodesBakReader(dev, fs.id.BankCount)} {
		fs.inodes = make(iNodes, r.Size()>>1)
		err = binary.Read(r, binary.BigEndian, &fs.inodes)
		if err != nil {
			return nil, err
		}

		if fs.inodes.valid(fs.firstPage()) {
			goto validINodes
		}
	}
	return nil, ErrInconsistent

validINodes:
	err = binary.Read(notesReader(dev, fs.id.BankCount), binary.BigEndian, &fs.notes)
	if err != nil {
		return nil, err
	}

	return fs, nil
}

func (p *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}

	if strings.Compare(name, ".") == 0 {
		return p.Root(), nil
	}

	for i, entry := range p.notes {
		name, err := N64FontCode.NewEncoder().String(name)
		if err != nil {
			return nil, err
		}
		l := min(len(entry.FileName), len(name))
		if strings.Compare(name, string(entry.FileName[:l])) == 0 {
			return newFile(p, i), nil
		}
	}

	return nil, fs.ErrNotExist
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
		root = append(root, fs.FileInfoToDirEntry(&file{
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

func (p *FS) readAt(noteIdx int, b []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, ErrInvalidOffset
	}

	pages, err := p.notePages(noteIdx)

	startIdx := off >> pageBits
	endIdx := ((off + int64(len(b))) >> pageBits) + 1
	if endIdx > int64(len(pages)) {
		err = io.EOF
		endIdx = int64(len(pages))
	}

	pages = pages[startIdx:endIdx]
	pageOff := off & pageMask

	for _, v := range pages {
		pageAddr := int64(v) << pageBits
		sr := io.NewSectionReader(p.dev, pageAddr+pageOff, pageSize-pageOff)
		written, err := io.ReadFull(sr, b[n:])
		n += written
		if err != io.ErrUnexpectedEOF && err != nil {
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

func iNodesReader(dev io.ReaderAt, bankCount uint8) *io.SectionReader {
	off := int64(1 << pageBits)
	n := int64(bankCount) << pageBits
	return io.NewSectionReader(dev, off, n)
}

func iNodesBakReader(dev io.ReaderAt, bankCount uint8) *io.SectionReader {
	off := (1 + int64(bankCount)) << pageBits
	n := int64(bankCount) << pageBits
	return io.NewSectionReader(dev, off, n)
}

func notesReader(dev io.ReaderAt, bankCount uint8) *io.SectionReader {
	off := (1 + int64(bankCount)<<1) << pageBits
	n := int64(2) << pageBits
	return io.NewSectionReader(dev, off, n)
}
