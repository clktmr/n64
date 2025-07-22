package fixed

import "fmt"

func Int6_10U(i int) Int6_10     { return Int6_10(i << 10) }
func Int6_10F(f float32) Int6_10 { return Int6_10(f * (1 << 10)) }

func (x Int6_10) Floor() int            { return int(x >> 10) }
func (x Int6_10) Ceil() int             { return int(int32(x) + (1<<10-1)>>10) }
func (x Int6_10) Mul(y Int6_10) Int6_10 { return Int6_10((int32(x) * int32(y)) >> 10) }
func (x Int6_10) Div(y Int6_10) Int6_10 { return Int6_10(int32(x) << 10 / int32(y)) }

func (x Int6_10) String() string {
	const shift, mask = 10, 1<<10 - 1
	return fmt.Sprintf("%d:%04d", int32(x>>shift), int32(x&mask))
}
