package mixer

import "fmt"

type uint20_12 uint32

func uint20_12U(u uint) uint20_12    { return uint20_12(u << 12) }
func uint20_12F(f float32) uint20_12 { return uint20_12(f * (1<<12 - 1)) }

func (x uint20_12) Floor() uint               { return uint(x >> 12) }
func (x uint20_12) Ceil() uint                { return uint((x + (1<<12 - 1)) >> 12) }
func (x uint20_12) Mul(y uint20_12) uint20_12 { return uint20_12((uint64(x) * uint64(y)) >> 12) }
func (x uint20_12) Div(y uint20_12) uint20_12 { return uint20_12(uint64(x) << 12 / uint64(y)) }

func (x uint20_12) String() string {
	const shift, mask = 12, 1<<12 - 1
	return fmt.Sprintf("%d:%04d", uint32(x>>shift), uint32(x&mask))
}

type int1_15 int16

func int1_15F(f float32) int1_15 { return int1_15(f * (1<<15 - 1)) }

func (x int1_15) Mul(y int1_15) int1_15 { return int1_15((int32(x) * int32(y)) >> 15) }
func (x int1_15) Div(y int1_15) int1_15 { return int1_15(int32(x) << 15 / int32(y)) }

func (x int1_15) String() string {
	const shift, mask = 15, 1<<15 - 1
	return fmt.Sprintf("%d:%05d", int16(x>>shift), int16(x&mask))
}
