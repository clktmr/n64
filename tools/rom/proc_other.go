//go:build !unix

package rom

import (
	"os/exec"
)

func kill(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}

func setSysProcAttr(cmd *exec.Cmd) {}
