package rcp_test

import (
	"bytes"
	"embedded/mmio"
	"testing"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
	n64testing "github.com/clktmr/n64/testing"
)

func TestMain(m *testing.M) { n64testing.TestMain(m) }

func TestReadWriteIO(t *testing.T) {
	testdata := []byte("Hello everybody, I'm Bonzo!")
	initBytes := cpu.MakePaddedSliceAligned[byte](64, 4)
	for i := range initBytes {
		initBytes[i] = byte(i+0x30) % 64
	}

	for busAlign := 0; busAlign < 7; busAlign += 1 {
		for sliceAlign := 0; sliceAlign < 3; sliceAlign += 1 {
			for sliceLen := 0; sliceLen < len(testdata); sliceLen += 1 {
				txbuf := cpu.MakePaddedSliceAligned[byte](64, 4)
				rxbuf := cpu.MakePaddedSliceAligned[byte](64, 4)

				rcp.WriteIO[*mmio.U32](0x0400_0000, initBytes)

				tx := txbuf[sliceAlign : sliceAlign+sliceLen]
				copy(tx, testdata)
				rcp.WriteIO[*mmio.U32](0x0400_0000+cpu.Addr(busAlign), tx)

				rx := rxbuf[sliceAlign : sliceAlign+sliceLen]
				rcp.ReadIO[*mmio.U32](0x0400_0000+cpu.Addr(busAlign), rx)

				if !bytes.Equal(tx, rx) {
					t.Logf("tx %q", string(tx))
					t.Logf("rx %q", string(rx))
					t.Error("mismatch at ", busAlign, sliceAlign, sliceLen)
				}

				rcp.ReadIO[*mmio.U32](0x0400_0000, rxbuf)
				start := busAlign
				if !bytes.Equal(rxbuf[:start], initBytes[:start]) {
					t.Logf("got      %q", string(rxbuf[:start]))
					t.Logf("expected %q", string(initBytes[:start]))
					t.Error("modified preceding data", busAlign, sliceAlign, sliceLen)
				}
				end := busAlign + sliceLen
				if !bytes.Equal(rxbuf[end:], initBytes[end:]) {
					t.Logf("got      %q", string(rxbuf[end:]))
					t.Logf("expected %q", string(initBytes[end:]))
					t.Error("modified succeeding data", busAlign, sliceAlign, sliceLen)
				}
				if t.Failed() {
					t.Fatal()
				}
			}
		}
	}
}
