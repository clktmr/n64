// Builds upon the rcp package to provide common interfaces and higher-level
// features.
package drivers

import "io"

type Logger interface {
	io.Writer
}

type Console interface {
	io.ReadWriter
}

// Returns a SystemWriter from an io.Writer for rtos.SetSystemWriter()
func NewSystemWriter(w io.Writer) func(int, []byte) int {
	return func(fd int, p []byte) int {
		written := 0
		for written < len(p) {
			n, _ := w.Write(p[written:])
			written += n
		}
		return written
	}
}
