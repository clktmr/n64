package toolexec

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"io"
	"slices"
)

// elfFile64 provides very limited access to modifying elf binaries.
type elfFile64 struct {
	ByteOrder binary.ByteOrder

	FileHeader     elf.Header64
	ProgHeaders    []elf.Prog64
	SectionHeaders []elf.Section64
	Sections       [][]byte

	SectionNames map[string]int
	Symbols      []elf.Symbol
}

func readElf64(r io.ReaderAt) (*elfFile64, error) {
	elfFile, err := elf.NewFile(r)
	if err != nil {
		return nil, err
	}
	if elfFile.Class != elf.ELFCLASS64 {
		return nil, errors.New("not a 64-bit elf")
	}
	symbols, err := elfFile.Symbols()
	if err != nil {
		return nil, err
	}

	ef64 := &elfFile64{
		ByteOrder: elfFile.ByteOrder,
		Symbols:   symbols,
	}

	sr := io.NewSectionReader(r, 0, 1<<63-1)
	err = binary.Read(sr, elfFile.ByteOrder, &ef64.FileHeader)
	if err != nil {
		return nil, err
	}

	ef64.ProgHeaders = make([]elf.Prog64, ef64.FileHeader.Phnum)
	ef64.SectionHeaders = make([]elf.Section64, ef64.FileHeader.Shnum)
	ef64.Sections = make([][]byte, ef64.FileHeader.Shnum)
	ef64.SectionNames = make(map[string]int)

	_, err = sr.Seek(int64(ef64.FileHeader.Phoff), io.SeekStart)
	if err != nil {
		return nil, err
	}
	err = binary.Read(sr, elfFile.ByteOrder, &ef64.ProgHeaders)
	if err != nil {
		return nil, err
	}

	_, err = sr.Seek(int64(ef64.FileHeader.Shoff), io.SeekStart)
	if err != nil {
		return nil, err
	}
	err = binary.Read(sr, elfFile.ByteOrder, &ef64.SectionHeaders)
	if err != nil {
		return nil, err
	}

	for i, section := range ef64.SectionHeaders {
		sr := io.NewSectionReader(r, int64(section.Off), int64(section.Size))
		ef64.Sections[i], err = io.ReadAll(sr)
		ef64.SectionNames[elfFile.Sections[i].Name] = i
		if err != nil {
			return nil, err
		}
	}

	return ef64, nil
}

func (p *elfFile64) Write(w io.WriterAt) error {
	ow := io.NewOffsetWriter(w, 0)
	err := binary.Write(ow, p.ByteOrder, p.FileHeader)
	if err != nil {
		return err
	}
	ow = io.NewOffsetWriter(w, int64(p.FileHeader.Phoff))
	err = binary.Write(ow, p.ByteOrder, p.ProgHeaders)
	if err != nil {
		return err
	}
	ow = io.NewOffsetWriter(w, int64(p.FileHeader.Shoff))
	err = binary.Write(ow, p.ByteOrder, p.SectionHeaders)
	if err != nil {
		return err
	}

	for i, section := range p.SectionHeaders {
		if section.Type == uint32(elf.SHT_NULL) || section.Type == uint32(elf.SHT_NOBITS) {
			continue
		}
		sr := bytes.NewReader(p.Sections[i])
		ow := io.NewOffsetWriter(w, int64(section.Off))
		_, err = io.Copy(ow, sr)
		if err != nil {
			return err
		}
	}

	return err
}

func alignUp(addr uint64, align uint64) uint64 {
	return (addr + align - 1) &^ (align - 1)
}

func (p *elfFile64) recalculateOffsets() {
	// Remove ProgHeaders instead of recalculating their offsets, we won't
	// need them.
	p.FileHeader.Phnum = 0
	p.ProgHeaders = []elf.Prog64{}

	seek := uint64(p.FileHeader.Ehsize)
	seek += uint64(p.FileHeader.Phentsize) * uint64(p.FileHeader.Phnum)
	seek += uint64(p.FileHeader.Shentsize) * uint64(p.FileHeader.Shnum)
	seek = alignUp(seek, 4096)
	for i, section := range p.SectionHeaders {
		if section.Type == uint32(elf.SHT_NULL) {
			continue
		}
		seek = alignUp(seek, section.Addralign)
		p.SectionHeaders[i].Off = seek
		if section.Type == uint32(elf.SHT_NOBITS) {
			continue
		}
		seek += section.Size
	}
}

func (p *elfFile64) AddProgSection(name string, align uint64, data []byte) (addr uint64) {
	nameidx := 0
	if shstrtab, ok := p.SectionNames[".shstrtab"]; ok {
		nameidx = len(p.Sections[shstrtab])
		p.Sections[shstrtab] = append(p.Sections[shstrtab], []byte(name)...)
		p.Sections[shstrtab] = append(p.Sections[shstrtab], 0)
		p.SectionHeaders[shstrtab].Size = uint64(len(p.Sections[shstrtab]))
		p.SectionNames[name] = len(p.Sections) - 1
	}

	for _, section := range p.SectionHeaders {
		if section.Type == uint32(elf.SHT_PROGBITS) &&
			section.Flags&uint64(elf.SHF_ALLOC) != 0 {
			addr = max(addr, section.Addr+section.Size)
		}
	}
	addr = alignUp(addr, align)

	p.SectionHeaders = append(p.SectionHeaders, elf.Section64{
		Name:      uint32(nameidx),
		Type:      uint32(elf.SHT_PROGBITS),
		Flags:     uint64(elf.SHF_ALLOC),
		Size:      uint64(len(data)),
		Addr:      addr,
		Addralign: align,
	})
	p.Sections = append(p.Sections, data)
	p.FileHeader.Shnum += 1

	p.recalculateOffsets()

	return
}

var errNoSymbol = errors.New("no such symbol")

func (p *elfFile64) Symbol(name string) (*elf.Symbol, error) {
	idx := slices.IndexFunc(p.Symbols, func(s elf.Symbol) bool {
		return s.Name == name
	})
	if idx == -1 {
		return nil, errNoSymbol
	}
	return &p.Symbols[idx], nil
}

func (p *elfFile64) SetSymbol(name string, value any) error {
	sym, err := p.Symbol(name)
	if err != nil {
		return err
	}
	sympos := sym.Value - p.SectionHeaders[sym.Section].Addr
	symdata := p.Sections[sym.Section][sympos : sympos+sym.Size]

	buf := bytes.NewBuffer(nil)
	binary.Write(buf, p.ByteOrder, value)

	if uint64(buf.Len()) > sym.Size {
		return errors.New("symbol size exceeded")
	}

	copy(symdata, buf.Bytes())

	return nil
}
