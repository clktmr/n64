//go:build debug

package debug

func Assert(b bool, message string) {
	if !b {
		panic(message)
	}
}
