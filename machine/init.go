//go:build n64

package machine

import (
	"embedded/arch/r4000/systim"

	"github.com/clktmr/n64/rcp/cpu"
)

func init() {
	systim.Setup(cpu.ClockSpeed)
}
