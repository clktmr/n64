package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/clktmr/n64/tools/font"
	"github.com/clktmr/n64/tools/pakfs"
	"github.com/clktmr/n64/tools/rom"
	"github.com/clktmr/n64/tools/toolexec"
)

const usageString = `n64go is a tool for development of Nintendo64 ROMs.

Usage:

	%s <command> [arguments]

The commands are:

	rom      convert and execute elf to n64 ROMs
	font     generate fonts to be used on the n64
	pakfs    modify and inspect pakfs images
	toolexec used as 'go build -toolexec' parameter
`

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), usageString, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	log.Default().SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	switch flag.Arg(0) {
	case "rom":
		rom.Main(flag.Args())
	case "pakfs":
		pakfs.Main(flag.Args())
	case "toolexec":
		toolexec.Main(flag.Args())
	case "font":
		font.Main(flag.Args())
	default:
		fmt.Fprintf(flag.CommandLine.Output(), "unknown command: %s\n", flag.Arg(0))
		flag.Usage()
		os.Exit(1)
	}
}
