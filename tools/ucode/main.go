package ucode

import (
	"debug/elf"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/rsp/ucode"
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
	outfile += ".ucode"

	elffile, err := elf.Open(infile)
	if err != nil {
		log.Fatalln(err)
	}
	defer elffile.Close()

	var ucode = &ucode.UCode{
		Name:  filepath.Base(infile),
		Entry: cpu.Addr(elffile.Entry),
		Text:  sectionData(elffile, ".text"),
		Data:  sectionData(elffile, ".data"),
	}

	w, err := os.Create(outfile)
	if err != nil {
		log.Fatalln(err)
	}
	defer w.Close()

	err = ucode.Store(w)
	if err != nil {
		log.Fatalln(err)
	}
}

func sectionData(elffile *elf.File, section string) []byte {
	s := elffile.Section(section)
	if s == nil {
		log.Fatalln("missing section:", section)
	}
	data, err := s.Data()
	if err != nil {
		log.Fatalln("reading section:", err)
	}
	return data
}

func symbolValue(elffile *elf.File, name string) uint64 {
	syms, err := elffile.Symbols()
	if err != nil {
		log.Fatalln("read symbols:", err)
	}
	idx := slices.IndexFunc(syms, func(sym elf.Symbol) bool {
		return sym.Name == name
	})
	if idx == -1 {
		log.Fatalln("read symbol:", err)
	}
	return syms[idx].Value
}
