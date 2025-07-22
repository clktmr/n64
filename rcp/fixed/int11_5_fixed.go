package fixed

import "fmt"

func Int11_5U(i int) Int11_5     { return Int11_5(i << 5) }
func Int11_5F(f float32) Int11_5 { return Int11_5(f * (1 << 5)) }

func (x Int11_5) Floor() int            { return int(x >> 5) }
func (x Int11_5) Ceil() int             { return int(int32(x) + (1<<5-1)>>5) }
func (x Int11_5) Mul(y Int11_5) Int11_5 { return Int11_5((int32(x) * int32(y)) >> 5) }
func (x Int11_5) Div(y Int11_5) Int11_5 { return Int11_5(int32(x) << 5 / int32(y)) }

func (x Int11_5) String() string {
	const shift, mask = 5, 1<<5 - 1
	return fmt.Sprintf("%d:%02d", int32(x>>shift), int32(x&mask))
}
