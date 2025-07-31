//go:build !unix

package rom

import (
	"os/exec"
	"syscall"
)

func killGroup(cmd *exec.Cmd, _ syscall.Signal) error {
	return cmd.Process.Kill() // TODO This doesn't kill childprocesses
}

func setSysProcAttr(cmd *exec.Cmd) {}
