//go:build debug

package debug

// Guard more complex assertions (i.e. anything that could panic) with `if
// debug.Enabled{...}`, otherwise they can't be removed in release builds.
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
