// Copyright 2024 The Embedded Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"debug/elf"
	"errors"
	"io"
)

func objcopy(dst io.WriterAt, src *elf.File) error {
	for _, s := range src.Sections {
		if s.Type != elf.SHT_PROGBITS || s.Flags&elf.SHF_ALLOC == 0 {
			continue
		}
		data, err := s.Data()
		if err != nil {
			return err
		}

		if s.Addr < src.Entry {
			return errors.New("data before entry point")
		}

		_, err = dst.WriteAt(data, int64(s.Addr-src.Entry))
		if err != nil {
			return err
		}
	}

	return nil
}
