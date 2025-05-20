// Copyright 2024 The Embedded Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"debug/elf"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

const usageString = `ELF to n64 ROM converter.

Usage: %s [flags] <elffile>

`

var (
	infile string
	format = flag.String("format", "z64", "z64 | uf2")
	run    = flag.String("run", "", "Run the ROM with this command")
)

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), usageString, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	log.Default().SetFlags(log.Lshortfile)
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

	elffile, err := elf.Open(infile)
	if err != nil {
		log.Fatalln(err)
	}
	defer elffile.Close()

	rom, err := os.CreateTemp("", "mkrom")
	if err != nil {
		log.Fatalln(err)
	}
	defer rom.Close()

	err = objcopy(io.NewOffsetWriter(rom, int64(len(n64Header)+len(n64IPL3))), elffile)
	if err != nil {
		log.Fatalln("objcopy:", err)
	}

	err = n64WriteROMHeader(rom, outfile)
	if err != nil {
		log.Fatalln("write rom header:", err)
	}

	switch *format {
	case "z64":
		out, err := os.Create(outfile)
		if err != nil {
			log.Fatalln(err)
		}
		defer out.Close()
		rom.Seek(0, io.SeekStart)
		_, err = io.Copy(out, rom)
		if err != nil {
			log.Fatalln(err)
		}
	case "uf2":
		// TODO pass file to n64WriteUF2
		rom, err := io.ReadAll(rom)
		if err != nil {
			log.Fatalln(err)
		}
		n64WriteUF2(outfile, rom)
	default:
		fmt.Printf("objcopy: %s format not supported", *format)
		os.Exit(1)
	}

	if *run != "" {
		cmd := exec.Command(*run, outfile)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err, ok := err.(*exec.ExitError); ok {
			os.Exit(err.ExitCode())
		}
		if err != nil {
			fmt.Println("run:", err)
			os.Exit(1)
		}
	}
}
