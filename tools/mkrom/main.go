// Copyright 2024 The Embedded Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const usageString = `ELF to n64 ROM converter.

Usage: %s [flags] <elffile>

`

var (
	infile string
	format = flag.String("format", "z64", "z64 | uf2")
)

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), usageString, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 1 {
		infile = flag.Arg(0)
	} else {
		flag.Usage()
		os.Exit(1)
	}

	outfile, _ := strings.CutSuffix(infile, ".elf")
	outfile += "." + *format

	obj := objcopy(infile)
	n64WriteROMFile(outfile, *format, obj)
}

func must[T any](ret T, err error) T {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return ret
}
