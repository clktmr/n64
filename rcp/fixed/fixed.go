// Package fixed provides fixed-point arithmetic types used by the RCP.
package fixed

//go:generate go run mkfixed.go UInt14_2 uint16
type UInt14_2 uint16

//go:generate go run mkfixed.go Int11_5 int16
type Int11_5 int16

//go:generate go run mkfixed.go Int6_10 int16
type Int6_10 int16
