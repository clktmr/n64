package cpu

import "unsafe"

const (
	ClockSpeed = 93.75e6
)

// Memory regions in 32bit Kernel mode
const (
	KSEG0 uintptr = 0xffffffff_80000000 // unmapped, cached
	KSEG1 uintptr = 0xffffffff_a0000000 // unmapped, uncached
)

// Addr represents a physical memory address
type Addr uint32

// Returns the physical address of a virtual address in KSEG0 or KSEG1.
func PhysicalAddress(addr uintptr) Addr {
	return Addr(addr & ^uintptr(0xe000_0000))
}

func PhysicalAddressSlice(s []byte) Addr {
	return PhysicalAddress(uintptr(unsafe.Pointer(unsafe.SliceData(s))))
}
