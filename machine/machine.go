// Package machine is imported by the runtime and allows the target to implement
// some hooks, most importantly rt0.
package machine

import (
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

type VideoType uint32

const (
	VideoPAL  VideoType = 0
	VideoNTSC VideoType = 1
	VideoMPAL VideoType = 2
)

type ResetType uint32

const (
	ResetCold ResetType = 0
	ResetWarm ResetType = 1
)

// Variables set by IPL3
var (
	Reset ResetType = *(*ResetType)(unsafe.Pointer(cpu.KSEG1 | 0x8000_030C))
	Video VideoType = *(*VideoType)(unsafe.Pointer(cpu.KSEG1 | 0x8000_0300))
)
