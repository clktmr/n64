//go:build debug

package debug

const Enabled = true

func Assert(b bool, message string) {
	if !b {
		panic(message)
	}
}

func AssertErrNil(err error) {
	if err != nil {
		panic(err)
	}
}
