package carts

import (
	"io"
	"n64/drivers/carts/everdrive64"
	"n64/drivers/carts/isviewer"
	"n64/drivers/carts/summercart64"
)

type Cart interface {
	io.Writer
}

func ProbeAll() (c Cart) {
	if isv := isviewer.Probe(); isv != nil {
		c = isv
	} else if ed64 := everdrive64.Probe(); ed64 != nil {
		// TODO should return the cart
		c = everdrive64.NewUNFLoader(ed64)
	} else if sc64 := summercart64.Probe(); sc64 != nil {
		c = sc64
	}
	return
}
