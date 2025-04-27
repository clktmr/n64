// Package carts provides probing for various n64 flashcarts.
//
// These are not required to run ROMs on the flashcarts. They provide access to
// additional features of the carts like usb logging.
//
// See the subdirectories for supported flashcarts.
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
