//go:build !(linux || darwin)

package pakfs

import (
	"fmt"
	"runtime"
)

func Mount(image, dir string) error {
	return fmt.Errorf("not supported on %s", runtime.GOOS)
}
