package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"

	"github.com/clktmr/n64/drivers/controller/pakfs"
	"rsc.io/rsc/fuse"
)

func must[T any](ret T, err error) T {
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
	return ret
}

const usageString = `Controller Pak File System Utility.

Usage:

	%s <command> [arguments]

The commands are:

	mount <image> <dir>	serve pakfs image via fuse
`

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), usageString, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	sigintr := make(chan os.Signal)
	signal.Notify(sigintr, os.Interrupt)

	switch flag.Arg(0) {
	case "mount":
		if flag.NArg() < 3 {
			flag.Usage()
			os.Exit(1)
		}
		image := flag.Arg(1)
		dir := flag.Arg(2)
		c := must(fuse.Mount(dir))
		r := must(os.OpenFile(image, os.O_RDWR, 0))
		fs := must(pakfs.Read(r))

		go c.Serve(&FS{fs})
		<-sigintr

		cmd := exec.Command("/bin/umount", dir)
		must(cmd.CombinedOutput())
	default:
		fmt.Fprintf(flag.CommandLine.Output(), "%s: unknown command\n", flag.Arg(0))
		flag.Usage()
		os.Exit(1)
	}
}
