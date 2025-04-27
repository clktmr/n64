//go:build !debug

// Package debug provides assertions that can be enabled with the debug build
// tag or will otherwise compile to no-ops.
//
// This is not considered idiomatic Go, but might be useful in an embedded
// environment.
package debug

// Guard more complex assertions (i.e. anything that could panic) with `if
// debug.Enabled{...}`, otherwise they can't be removed in release builds.
const Enabled = false

// Assert panics if b is false.
func Assert(b bool, message string) {}

// AssertErrNil panics if err is not nil.
func AssertErrNil(err error) {}
