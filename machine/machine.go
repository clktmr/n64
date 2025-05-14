// Package machine implements some target specific functions of the runtime and
// provides access to the bootloader.
//
// The machine package must be imported by all n64 applications. If unused
// import it for it's side effects:
//
//	import _ github.com/clktmr/n64/machine
package machine

import (
	"embedded/arch/r4000/systim"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

type video uint32

const (
	VideoPAL  video = 0
	VideoNTSC video = 1
	VideoMPAL video = 2
)

type reset uint32

const (
	ResetCold reset = 0 // Power switch
	ResetWarm reset = 1 // Reset button
)

type pak uint32

const (
	PakJumper    pak = 4 * 1024 * 1024
	PakExpansion pak = 8 * 1024 * 1024
)

// These variables are set by bootloader (aka IPL3).
var (
	// Reports whether the console booted from a power cycle or reset.
	ResetType reset = *(*reset)(unsafe.Pointer(cpu.KSEG1 | 0x8000_030C))

	// Reports the console's region.
	VideoType video = *(*video)(unsafe.Pointer(cpu.KSEG1 | 0x8000_0300))

	// Reports if an expansion pak is installed.
	PakType pak = *(*pak)(unsafe.Pointer(cpu.KSEG1 | 0x8000_0318))
)

func init() {
	systim.Setup(cpu.ClockSpeed)
}
