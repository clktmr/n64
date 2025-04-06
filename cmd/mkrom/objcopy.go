// Copyright 2024 The Embedded Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"debug/elf"
	"fmt"
	"os"
	"sort"
)

var ones []byte

type section struct {
	addr   uint64
	offset int64
	data   []byte
}

func padBytes(cache *[]byte, n int, b byte) []byte {
	if len(*cache) < n {
		*cache = make([]byte, n)
		for i := range *cache {
			(*cache)[i] = b
		}
	}
	return (*cache)[:n]
}

func objcopy(elfFile string) *bytes.Buffer {
	r := must(os.Open(elfFile))
	defer r.Close()
	f := must(elf.NewFile(r))
	defer f.Close()
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
		return bytes.NewBuffer([]byte(""))
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

	return w
}
