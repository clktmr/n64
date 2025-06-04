package ucode

import (
	"debug/elf"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const usageString = `RSP microcode converter.

Usage: %s [flags] <elffile>

`

var (
	flags = flag.NewFlagSet("ucode", flag.ExitOnError)

	infile string
)

func usage() {
	fmt.Fprintf(flags.Output(), usageString, "ucode")
	flags.PrintDefaults()
}

func Main(args []string) {
	flags.Usage = usage
	flags.Parse(args[1:])

	if flags.NArg() == 1 {
		infile = flags.Arg(0)
	} else {
		log.Println("too many arguments")
		flags.Usage()
		os.Exit(1)
	}

	outfile, _ := strings.CutSuffix(infile, ".elf")

	elffile, err := elf.Open(infile)
	if err != nil {
		log.Fatalln(err)
	}
	defer elffile.Close()

	for _, s := range elffile.Sections {
		if s.Type != elf.SHT_PROGBITS || s.Flags&elf.SHF_ALLOC == 0 {
			continue
		}

		f, err := os.Create(outfile + s.Name)
		if err != nil {
			log.Fatal(err)
		}
		_, err = io.Copy(f, io.NewSectionReader(s, 0x0, 0x1000))
		if err != nil {
			log.Fatal(err)
		}
	}
}
