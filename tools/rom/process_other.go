//go:build !unix

package rom

import (
	"os"
	"os/exec"
)

func processGroupEnable(cmd *exec.Cmd) {}

func processGroupKill(cmd *exec.Cmd) error {
	return cmd.Process.Signal(os.Interrupt)
}
