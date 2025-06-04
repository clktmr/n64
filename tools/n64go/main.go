// n64go bundles all commands into a single executable. Run available commands
// with:
//
//	n64go <command> [arguments]
//
// The commands are:
//
//   - [github.com/clktmr/n64/tools/rom]      convert and execute elf to n64 ROMs
//   - [github.com/clktmr/n64/tools/texture]  generate textures to be used on the n64
//   - [github.com/clktmr/n64/tools/font]     generate fonts to be used on the n64
//   - [github.com/clktmr/n64/tools/pakfs]    modify and inspect pakfs images
//   - [github.com/clktmr/n64/tools/ucode]    dump rsp microcode elf to binary
//   - [github.com/clktmr/n64/tools/toolexec] used as 'go build -toolexec' parameter
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/clktmr/n64/tools/font"
	"github.com/clktmr/n64/tools/pakfs"
	"github.com/clktmr/n64/tools/rom"
	"github.com/clktmr/n64/tools/texture"
	"github.com/clktmr/n64/tools/toolexec"
	"github.com/clktmr/n64/tools/ucode"
)

const usageString = `n64go is a tool for development of Nintendo 64 ROMs.

Usage:

	%s <command> [arguments]

The commands are:

	rom      convert and execute elf to n64 ROMs
	texture  convert images to n64 textures
	font     generate fonts to be used on the n64
	pakfs    modify and inspect pakfs images
	ucode    dump rsp microcode elf to binary
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
	case "texture":
		texture.Main(flag.Args())
	case "ucode":
		ucode.Main(flag.Args())
	default:
		fmt.Fprintf(flag.CommandLine.Output(), "unknown command: %s\n", flag.Arg(0))
		flag.Usage()
		os.Exit(1)
	}
}
