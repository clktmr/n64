//go:build !debug

package debug

func Assert(b bool, message string) {}
func AssertErrNil(err error)        {}
