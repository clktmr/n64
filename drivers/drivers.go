// Package drivers and it's subdirectories build upon
// [github.com/clktmr/n64/rcp] to provide common interfaces and higher-level
// features.
package drivers

import "io"

// SystemWriter is a function that implements the builtin print(). It can be
// passed to [embedded/rtos.SetSystemWriter].
type SystemWriter func(int, []byte) int

// Returns a SystemWriter from an io.Writer.
func NewSystemWriter(w io.Writer) SystemWriter {
	// FIXME SystemWriter needs go:nosplit pragma
	return func(fd int, p []byte) int {
		n, _ := w.Write(p)
		return n
	}
}
