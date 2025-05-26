// Copyright 2024 The Embedded Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rom

import (
	"bufio"
	"debug/elf"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	_ "embed"

	"github.com/kballard/go-shellquote"
)

const usageString = `ELF to n64 ROM converter.

Usage: %s [flags] <elffile>

`

var (
	flags = flag.NewFlagSet("rom", flag.ExitOnError)

	infile string
	format = flags.String("format", "z64", "z64 | uf2")
	run    = flags.String("run", "", "Run the ROM with command")
)

func usage() {
	fmt.Fprintf(flags.Output(), usageString, "rom")
	flags.PrintDefaults()
}

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

// libdragon IPL3 r8 (compatibility mode)
// Author: Giovanni Bajo (giovannibajo@gmail.com)
//
//go:embed ipl3_compat.z64
var n64IPL3 []byte

func n64WriteROMHeader(rom *os.File, gametitle string) error {
	copy(n64IPL3[0x20:0x34], fmt.Sprintf("%-20s", gametitle)) // TODO encode in ascii
	_, err := rom.WriteAt(n64IPL3, 0)
	if err != nil {
		return err
	}

	return nil
}

func Main(args []string) {
	flags.Usage = usage
	flags.Parse(args[1:])

	if flags.NArg() == 1 {
		infile = flags.Arg(0)
	} else {
		flags.Usage()
		os.Exit(1)
	}

	outfile, _ := strings.CutSuffix(infile, ".elf")
	outfile += "." + *format

	elffile, err := elf.Open(infile)
	if err != nil {
		log.Fatalln(err)
	}
	defer elffile.Close()

	rom, err := os.CreateTemp("", "rom")
	if err != nil {
		log.Fatalln(err)
	}
	defer rom.Close()

	err = objcopy(io.NewOffsetWriter(rom, int64(len(n64IPL3))), elffile)
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
		err = n64WriteUF2(outfile, rom)
		if err != nil {
			log.Fatalln(err)
		}
	default:
		log.Fatalf("objcopy: %s format not supported", *format)
	}

	if *run != "" {
		runROM(*run, outfile)
	}
}

func runROM(cmdpath, rompath string) {
	args, err := shellquote.Split(cmdpath)
	if err != nil {
		log.Fatal("run:", err)
	}
	args = append(args, rompath)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	processGroupEnable(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("open stdout:", err)
	}

	sigintr := make(chan os.Signal, 1)
	signal.Notify(sigintr, os.Interrupt)

	err = cmd.Start()
	if err != nil {
		log.Fatal("start command:", err)
	}

	go func() {
		<-sigintr
		stdout.Close()
		err := processGroupKill(cmd)
		if err != nil {
			log.Println(err)
		}
	}()

	scanner := bufio.NewScanner(stdout)
	exiting := false
	code := 0
	for scanner.Scan() {
		log.Println(scanner.Text())
		if exiting {
			continue
		}
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "fatal error:"), strings.HasPrefix(line, "panic:"):
			fallthrough
		case line == "FAIL":
			code = 1
			fallthrough
		case line == "PASS":
			exiting = true
			go func() {
				// give panic() time to print the stacktrace
				time.Sleep(500 * time.Millisecond)
				stdout.Close()
				err := processGroupKill(cmd)
				if err != nil {
					log.Println(err)
				}
			}()
		}
	}
	cmd.Wait()
	os.Exit(code)
}
