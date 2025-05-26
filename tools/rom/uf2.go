// Copyright 2024 The Embedded Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rom

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"unsafe"
)

const (
	UF2NotMainFlash         = 0x00000001
	UF2FileContainer        = 0x00001000
	UF2FamilyIDPresent      = 0x00002000
	UF2MD5ChecksumPresent   = 0x00004000
	UF2ExtensionTagsPresent = 0x00008000
)

// UF2 families
const (
	uf2_rp2040        = 0xe48bff56
	uf2_absolute      = 0xe48bff57
	uf2_data          = 0xe48bff58
	uf2_rp2350_arm_s  = 0xe48bff59
	uf2_rp2350_riscv  = 0xe48bff5a
	uf2_rp2350_arm_ns = 0xe48bff5b
)

type uf2block struct {
	Magic0 uint32
	Magic1 uint32
	Flags  uint32
	Addr   uint32
	Len    uint32
	Seq    uint32
	Total  uint32
	Family uint32
	Data   [256]byte
	_      [476 - 256]byte
	Magic2 uint32
}

type UF2Writer struct {
	w io.Writer
	b uf2block
}

func NewUF2Writer(w io.Writer, addr, flags, family uint32, size int) *UF2Writer {
	u := new(UF2Writer)
	u.w = w
	u.b.Magic0 = 0x0a324655
	u.b.Magic1 = 0x9e5d5157
	u.b.Flags = flags
	u.b.Addr = addr
	u.b.Total = uint32((size + len(u.b.Data) - 1) / len(u.b.Data))
	u.b.Family = family
	u.b.Magic2 = 0x0ab16f30
	return u
}

func (u *UF2Writer) WriteString(s string) (n int, err error) {
	b := &u.b
	for len(s) != 0 {
		m := copy(b.Data[b.Len:], s)
		n += m
		s = s[m:]
		b.Len += uint32(m)
		if int(b.Len) == len(b.Data) {
			err = binary.Write(u.w, binary.LittleEndian, b)
			if err != nil {
				return
			}
			b.Addr += b.Len
			b.Seq++
			b.Len = 0
		}
	}
	return
}

func (u *UF2Writer) Write(p []byte) (n int, err error) {
	return u.WriteString(*(*string)(unsafe.Pointer(&p)))
}

func (u *UF2Writer) Flush() (err error) {
	b := &u.b
	if b.Len == 0 {
		return
	}
	clear(b.Data[b.Len:])
	b.Len = uint32(len(b.Data))
	err = binary.Write(u.w, binary.LittleEndian, b)
	b.Addr += b.Len
	b.Seq++
	b.Len = 0
	return
}

// n64WriteUF2 is a translation to Go of the generateAndSaveUF2 function from
// https://kbeckmann.github.io/PicoCart64/js/PicoCart64.js
// Original author: Konrad Beckmann.
func n64WriteUF2(obj string, rom []byte) error {
	const (
		chunkSize = 1024
		header    = "picocartcompress"
		_1M       = 1024 * 1024
	)

	// Split ROM into chunks

	var (
		chunkData   []byte
		chunkMap    [(0x8000 - len(header)) / 2]uint16
		chunkMapLen int
	)

	for i := 0; i < len(rom); i += chunkSize {
		k := min(len(rom), i+chunkSize)
		chunk := rom[i:k]

		// Check if chunk is in chunkData
		for k = 0; k < len(chunkData); k += chunkSize {
			if bytes.HasPrefix(chunkData[k:], chunk) {
				break
			}
		}
		if k == len(chunkData) {
			// Found a unique chunk
			chunkData = append(chunkData, chunk...)
		}
		if chunkMapLen >= len(chunkMap) {
			return fmt.Errorf("n64 uf2: chunk map overflow")
		}
		k /= chunkSize // chunk number in chunkData
		chunkMap[chunkMapLen] = uint16(k)
		chunkMapLen++
	}

	newSize := len(header) + len(chunkMap)*2 + len(chunkData)
	flashStart := 0x10000000
	lastAddr := 0x10030000 + newSize
	flashEnd := flashStart + 2*_1M
	if lastAddr > flashEnd {
		log.Printf(
			"n64 uf2: the compressed ROM requires %d MiB of Flash (> 2 MiB)\n",
			(lastAddr-flashStart+_1M-1)/_1M,
		)
	}

	// Save compressed ROM
	f, err := os.Create(obj)
	if err != nil {
		return err
	}
	defer f.Close()

	w := NewUF2Writer(f, 0x10030000, UF2FamilyIDPresent, uf2_rp2040, newSize)
	_, err = w.WriteString(header)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, chunkMap)
	if err != nil {
		return err
	}
	_, err = w.Write(chunkData)
	if err != nil {
		return err
	}
	err = w.Flush()
	if err != nil {
		return err
	}
	return nil
}
