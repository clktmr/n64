//go:build unix

package rom

import (
	"os/exec"
	"syscall"
)

func killGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	return syscall.Kill(-cmd.Process.Pid, sig)
}

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
		Setpgid:   true,
	}
}
