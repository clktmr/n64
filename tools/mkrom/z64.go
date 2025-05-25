// Copyright 2024 The Embedded Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
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

func n64WriteROMHeader(rom *os.File, gametitle string) error {
	copy(n64IPL3[0x20:0x34], fmt.Sprintf("%-20s", gametitle)) // TODO encode in ascii
	_, err := rom.WriteAt(n64IPL3, 0)
	if err != nil {
		return err
	}

	return nil
}

// libdragon IPL3 r8 (compatibility mode)
// Author: Giovanni Bajo (giovannibajo@gmail.com)
//
//go:embed ipl3_compat.z64
var n64IPL3 []byte
