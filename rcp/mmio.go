package rcp

import (
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

type Register32[T any] interface {
	*T
	Load() uint32
	Store(uint32)
}

// WriteIO copies slice p to physical address busAddr using SysAd bus MMIO. Note
// that it needs to read from the SysAd bus if p's start or end aren't 4 byte
// aligned. This might lead to unexpected behaviour of write-only address
// ranges.
//
//go:nosplit
func WriteIO[T Register32[Q], Q any](busAddr cpu.Addr, p []byte) {
	end := cpu.KSEG1 | uintptr(busAddr+cpu.Addr(len(p)+3))&^0x3
	shift := -(int(busAddr) & 0x3)
	endshift := ^(int(busAddr) + len(p) - 1) & 0x3

	busPtr := unsafe.Pointer(cpu.KSEG1 | uintptr(busAddr&^0x3))
	pPtr := unsafe.Pointer(unsafe.SliceData(p))
	pPtr = unsafe.Add(pPtr, shift)
	for uintptr(busPtr) < end {
		data, mask := uint32(0), uint32(0xffff_ffff)
		if shift != 0 { // first dword
			mask &= 0xffff_ffff >> ((-shift) << 3)
		}
		if end-uintptr(busPtr) == 4 && endshift != 0 { // last dword
			mask &= 0xffff_ffff << (endshift << 3)
		}
		if mask != 0xffff_ffff { // read data before writing
			data = (T)(busPtr).Load() &^ mask
		}
		if uintptr(pPtr)&0x3 == 0 {
			data |= *(*uint32)(pPtr) & mask
		} else { // unaligned access forbidden on mips
			p := *(*[4]byte)(pPtr)
			data |= (uint32(p[0])<<24 | uint32(p[1])<<16 | uint32(p[2])<<8 | uint32(p[3])) & mask
		}
		(T)(busPtr).Store(data)

		shift = 0
		pPtr = unsafe.Add(pPtr, 4)
		busPtr = unsafe.Add(busPtr, 4)
	}
}

// ReadIO copies from physical address busAddr to slice p using SysAd bud MMIO.
//
//go:nosplit
func ReadIO[T Register32[Q], Q any](busAddr cpu.Addr, p []byte) {
	end := cpu.KSEG1 | uintptr(busAddr+cpu.Addr(len(p)+3))&^0x3
	shift := -(int(busAddr) & 0x3)

	busPtr := unsafe.Pointer(cpu.KSEG1 | uintptr(busAddr&^0x3))
	pPtr := unsafe.Pointer(unsafe.SliceData(p))
	pPtr = unsafe.Add(pPtr, shift)
	for uintptr(busPtr) < end {
		data := (T)(busPtr).Load()
		if uintptr(pPtr)&0x3 == 0 {
			*(*uint32)(pPtr) = data
		} else { // unaligned access forbidden on mips
			i, s := 0, (3+shift)<<3
			for i < min(len(p), shift+4) {
				p[i] = byte(data >> s)
				i, s = i+1, s-8
			}
			p = p[i:]
			shift = 0
		}

		pPtr = unsafe.Add(pPtr, 4)
		busPtr = unsafe.Add(busPtr, 4)
	}
}
