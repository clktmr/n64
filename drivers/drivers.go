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
