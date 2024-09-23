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
var ErrNoSpace = errors.New("no space left")
var ErrReadOnly = errors.New("read-only file system")

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

func notesReader(dev io.ReaderAt, bankCount uint8) *io.SectionReader {
	off := (1 + int64(bankCount)<<1) << pageBits
	n := int64(2) << pageBits
	return io.NewSectionReader(dev, off, n)
}
