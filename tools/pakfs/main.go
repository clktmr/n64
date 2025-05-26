package pakfs

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

var flags = flag.NewFlagSet("pakfs", flag.ExitOnError)

func usage() {
	fmt.Fprintf(flags.Output(), usageString, "pakfs")
	flags.PrintDefaults()
}

func Main(args []string) {
	flags.Usage = usage
	flags.Parse(args[1:])

	if flags.NArg() < 1 {
		flags.Usage()
		os.Exit(1)
	}

	sigintr := make(chan os.Signal)
	signal.Notify(sigintr, os.Interrupt)

	switch flags.Arg(0) {
	case "mount":
		if flags.NArg() < 3 {
			flags.Usage()
			os.Exit(1)
		}
		image := flags.Arg(1)
		dir := flags.Arg(2)
		c := must(fuse.Mount(dir))
		r := must(os.OpenFile(image, os.O_RDWR, 0))
		fs := must(pakfs.Read(r))

		go c.Serve(&FS{fs})
		<-sigintr

		cmd := exec.Command("/bin/umount", dir)
		must(cmd.CombinedOutput())
	default:
		fmt.Fprintf(flags.Output(), "unknown command: %s\n", flags.Arg(0))
		flags.Usage()
		os.Exit(1)
	}
}
