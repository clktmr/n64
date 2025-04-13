// Copyright 2024 The Embedded Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	_ "embed"
)

// https://n64brew.dev/wiki/ROM_Header
var n64Header = [0x40]byte{
	0x00: 0x80, 0x37, 0x12, 0x40, // PI BSD DOM1 Configuration Flags
	0x04: 0x00, 0x00, 0x00, 0x0f, // Clock Rate
	0x08: 0x80, 0x00, 0x04, 0x00, // Boot Address
	0x0c: 0x00, 0x00, 0x14, 0x44, // Libultra Version
	//0x10: Check Code (8 bytes)
	//0x18: Reserved (8 bytes)
	//0x20: Game Title (20 bytes)
	//0x34: Reserved (7 bytes)
	0x3b: 'N',      // Category Code ('N' = 0x4e = "Game Pak"),
	0x3c: ' ', ' ', // Unique Code
	0x3e: ' ', // Destination Code
	0x3f: 0,   // ROM Version
}

const n64ChecksumLen = 1024 * 1024

// n64CRC is a loose translation to Go of the calculate_crc function from
// the n64chain:
//
// https://github.com/tj90241/n64chain
//
// The original copyright notes from the tools/checksum.c file:
//
// n64chain: A (free) open-source N64 development toolchain.
// Copyright 2014 Tyler J. Stachecki <tstache1@binghamton.edu>
//
// This file is more or less a direct rip of chksum64:
// Copyright 1997 Andreas Sterbenz <stan@sbox.tu-graz.ac.at>
func n64CRC(buf []byte) (crc [2]uint32) {
	const CIC_NUS6102_SEED uint32 = 0xF8CA4DDC
	t1 := CIC_NUS6102_SEED
	t2 := CIC_NUS6102_SEED
	t3 := CIC_NUS6102_SEED
	t4 := CIC_NUS6102_SEED
	t5 := CIC_NUS6102_SEED
	t6 := CIC_NUS6102_SEED

	for i := 0; i < len(buf); i += 4 {
		c1 := binary.BigEndian.Uint32(buf[i:])
		k1 := t6 + c1
		if k1 < t6 {
			t4++
		}
		t6 = k1
		t3 ^= c1
		k2 := c1 & 0x1F
		k1 = c1<<k2 | c1>>(32-k2)
		t5 += k1
		if c1 < t2 {
			t2 ^= k1
		} else {
			t2 ^= t6 ^ c1
		}
		t1 += c1 ^ t5
	}

	crc[0] = t6 ^ t4 ^ t3
	crc[1] = t5 ^ t2 ^ t1
	return
}

func n64WriteROMFile(obj, format string, buf *bytes.Buffer) {
	pad := n64ChecksumLen - buf.Len()
	if pad > 0 {
		buf.Write(padBytes(&ones, pad, 0xff))
	}
	crc := n64CRC(buf.Bytes()[:n64ChecksumLen])
	binary.BigEndian.PutUint32(n64Header[0x10:], crc[0])
	binary.BigEndian.PutUint32(n64Header[0x14:], crc[1])
	copy(n64Header[0x20:0x34], obj) // Game Title
	rom := make([]byte, 0, len(n64Header)+len(n64IPL3)+buf.Len())
	rom = append(rom, n64Header[:]...)
	rom = append(rom, n64IPL3...)
	rom = append(rom, buf.Bytes()...)

	switch format {
	case "z64":
		must(0, os.WriteFile(obj, rom, 0644))
	case "uf2":
		n64WriteUF2(obj, rom)
	default:
		fmt.Printf("objcopy: %s format not supported", format)
		os.Exit(1)
	}
}

// 6102/7101 MD5=e24dd796b2fa16511521139d28c8356b
//
//go:embed ipl3.bin
var n64IPL3 []byte
