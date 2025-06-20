package cpu

import "unsafe"

// The CPU's clock speed
const ClockSpeed = 93.75e6

// Memory regions in 32bit Kernel mode
const (
	KSEG0 uintptr = 0xffffffff_80000000 // unmapped, cached
	KSEG1 uintptr = 0xffffffff_a0000000 // unmapped, uncached
)

// Addr represents a physical memory address
type Addr uint32

// PAddr returns the physical address of a virtual address.
func PAddr(addr uintptr) Addr {
	return Addr(addr & ^uintptr(0xe000_0000))
}

type Pointer[T any] interface{ *T }

// PhysicalAddress returns the physical address of a pointer.
func PhysicalAddress[T Pointer[Q], Q any](s T) Addr {
	return PAddr(uintptr(unsafe.Pointer(s)))
}

// Same as [PhysicalAddress] but for slices.
func PhysicalAddressSlice[T any](s []T) Addr {
	return PAddr(uintptr(unsafe.Pointer(unsafe.SliceData(s))))
}
