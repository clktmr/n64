// Copyright 2024 The Embedded Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

// Objcopy holds the binary copy and still provides access to symbols.
type Objcopy struct {
	elf    *elf.File
	offset uint64
	data   *bytes.Buffer
}

type section struct {
	addr   uint64
	offset int64
	data   []byte
}

var ones []byte

func padBytes(cache *[]byte, n int, b byte) []byte {
	if len(*cache) < n {
		*cache = make([]byte, n)
		for i := range *cache {
			(*cache)[i] = b
		}
	}
	return (*cache)[:n]
}

func NewObjcopy(f *elf.File) *Objcopy {
	sections := make([]*section, 0, 10)
	for i, s := range f.Sections {
		if s.Type != elf.SHT_PROGBITS || s.Flags&elf.SHF_ALLOC == 0 {
			if k := i + 1; k < len(f.Sections) && len(sections) != 0 {
				n := f.Sections[k]
				if n.Type == elf.SHT_PROGBITS && n.Flags&elf.SHF_ALLOC != 0 {
					fmt.Fprintf(os.Stderr, "objcopy: skipping section '%s' (%d bytes)\n", s.Name, s.Size)
				}
			}
			continue
		}
		data := must(s.Data())
		sections = append(sections, &section{s.Addr, int64(s.Offset), data})
	}
	if len(sections) == 0 {
		return &Objcopy{}
	}
	sort.Slice(
		sections,
		func(i, j int) bool {
			return sections[i].offset < sections[j].offset
		},
	)
	startAddr, startOffset := sections[0].addr, sections[0].offset
	for _, s := range sections {
		s.offset -= startOffset
		s.addr = startAddr + uint64(s.offset)
	}

	w := bytes.NewBuffer(make([]byte, 0, n64ChecksumLen))
	for i, s := range sections {
		must(w.Write(s.data))
		pad := 0
		if i+1 < len(sections) {
			pad = int(sections[i+1].offset-s.offset) - len(s.data)
		}
		if pad == 0 {
			continue
		}
		must(w.Write(padBytes(&ones, pad, 0xff)))
	}

	return &Objcopy{f, startAddr, w}
}

func (p *Objcopy) ByteOrder() binary.ByteOrder {
	return p.elf.ByteOrder
}

func (p *Objcopy) SymbolData(name string) []byte {
	syms := must(p.elf.Symbols())
	for _, sym := range syms {
		if sym.Name == name {
			sv, ss := sym.Value-p.offset, sym.Size
			return p.data.Bytes()[sv : sv+ss]
		}
	}
	return nil
}
