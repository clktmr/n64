package fixed

import "fmt"

func UInt14_2U(i int) UInt14_2     { return UInt14_2(i << 2) }
func UInt14_2F(f float32) UInt14_2 { return UInt14_2(f * (1 << 2)) }

func (x UInt14_2) Floor() int              { return int(x >> 2) }
func (x UInt14_2) Ceil() int               { return int(uint32(x) + (1<<2-1)>>2) }
func (x UInt14_2) Mul(y UInt14_2) UInt14_2 { return UInt14_2((uint32(x) * uint32(y)) >> 2) }
func (x UInt14_2) Div(y UInt14_2) UInt14_2 { return UInt14_2(uint32(x) << 2 / uint32(y)) }

func (x UInt14_2) String() string {
	const shift, mask = 2, 1<<2 - 1
	return fmt.Sprintf("%d:%01d", uint32(x>>shift), uint32(x&mask))
}
