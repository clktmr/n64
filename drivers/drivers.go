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

type SystemWriter func(int, []byte) int

// Returns a SystemWriter from an io.Writer for rtos.SetSystemWriter()
func NewSystemWriter(w io.Writer) SystemWriter {
	return func(fd int, p []byte) int {
		written := 0
		for written < len(p) {
			n, _ := w.Write(p[written:])
			written += n
		}
		return written
	}
}

// SystemWriter also implements io.Writer and is guaranteed to never return an
// error and block as long as all bytes have been written.  Because no one will
// ever check the returned error from a fmt.Print().
func (sw SystemWriter) Write(p []byte) (n int, err error) {
	return sw(0, p), nil
}
