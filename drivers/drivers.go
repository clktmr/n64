// Builds upon the rcp package to provide common interfaces and higher-level
// features.
package drivers

import "io"

// FIXME SystemWriter needs go:nosplit pragma
type SystemWriter func(int, []byte) int

// Returns a SystemWriter from an io.Writer for rtos.SetSystemWriter()
func NewSystemWriter(w io.Writer) SystemWriter {
	return func(fd int, p []byte) int {
		n, _ := w.Write(p)
		return n
	}
}
