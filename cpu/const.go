package cpu

const (
	ClockSpeed = 93.75e6
)

// Memory regions in 32bit Kernel mode
const (
	KSEG1 = 0x20000000 // unmapped, uncached
)

// Returns the physical address of an address in KSEG0 or KSEG1.
func PhysicalAddress(addr uintptr) uint32 {
	return uint32(addr & ^uintptr(0xe000_0000))
}
