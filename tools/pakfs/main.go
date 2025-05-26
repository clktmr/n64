package pakfs

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
)

const usageString = `Controller Pak File System Utility.

Usage:

	%s <command> [arguments]

The commands are:

	mount <image> <dir>	serve pakfs image via fuse
`

var flags = flag.NewFlagSet("pakfs", flag.ExitOnError)

func usage() {
	fmt.Fprintf(flags.Output(), usageString, "pakfs")
	flags.PrintDefaults()
}

var sigintr = make(chan os.Signal, 1)

func Main(args []string) {
	flags.Usage = usage
	flags.Parse(args[1:])

	if flags.NArg() < 1 {
		flags.Usage()
		os.Exit(1)
	}

	signal.Notify(sigintr, os.Interrupt)

	switch flags.Arg(0) {
	case "mount":
		if flags.NArg() < 3 {
			flags.Usage()
			os.Exit(1)
		}
		image := flags.Arg(1)
		dir := flags.Arg(2)
		err := mount(image, dir)
		if err != nil {
			log.Fatalln("mount:", err)
		}
	default:
		fmt.Fprintf(flags.Output(), "unknown command: %s\n", flags.Arg(0))
		flags.Usage()
		os.Exit(1)
	}
}
