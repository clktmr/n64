//go:build !debug

package debug

const Enabled = false

func Assert(b bool, message string) {}
func AssertErrNil(err error)        {}
