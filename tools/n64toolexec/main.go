// n64toolexec is invocated with go build's -toolexec flag. It enforces settings
// that are required for n64 build.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
)

func main() {
	cmdname := os.Args[1]
	cmdargs := os.Args[2:]

	tool := filepath.Base(cmdname)
	switch tool {
	case "link":
		// Enforce symbols cause they are currently needed by mkrom
		for {
			if idx := slices.Index(cmdargs, "-s"); idx > 0 {
				cmdargs = slices.Delete(cmdargs, idx, idx+1)
			} else {
				break
			}
		}
	}

	cmd := exec.Command(cmdname, cmdargs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err, ok := err.(*exec.ExitError); ok {
		os.Exit(err.ExitCode())
	}
	if err != nil {
		fmt.Println("toolexec:", err)
		os.Exit(1)
	}
}
