package carts

import (
	"io"

	"github.com/clktmr/n64/drivers/carts/everdrive64"
	"github.com/clktmr/n64/drivers/carts/isviewer"
	"github.com/clktmr/n64/drivers/carts/summercart64"
)

type Cart interface {
	io.Writer
}

func ProbeAll() (c Cart) {
	if sc64 := summercart64.Probe(); sc64 != nil {
		c = sc64
	} else if ed64 := everdrive64.Probe(); ed64 != nil {
		// TODO should return the cart
		c = everdrive64.NewUNFLoader(ed64)
	} else if isv := isviewer.Probe(); isv != nil {
		c = isv
	}
	return
}
