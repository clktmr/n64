package summercart64_test

import (
	"bytes"
	"testing"

	"github.com/clktmr/n64/drivers/carts/summercart64"
	"github.com/clktmr/n64/rcp/cpu"
)

func mustSC64(t *testing.T) (sc64 *summercart64.SummerCart64) {
	sc64 = summercart64.Probe()
	if sc64 == nil {
		t.Skip("needs SummerCart64")
	}
	return
}

func TestUSBRead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	sc64 := mustSC64(t)
	buf := cpu.MakePaddedSlice[byte](7)

	tests := map[string]struct {
		testdata []byte
	}{
		"short": {[]byte("foo\n")},
		"fit":   {[]byte("barbaz\n")},
		"long":  {[]byte("summercart64\n")},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var err error
			var n int
			for n = -1; n != 0; n, err = sc64.Read(buf) {
				// discard, make we start with empty buffer
			}

			t.Logf("Please type \"%v\"", string(tc.testdata[:len(tc.testdata)-1]))

			for len(tc.testdata) > 0 {
				n, err = sc64.Read(buf)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue
				}
				if n != min(len(tc.testdata), len(buf)) {
					t.Fatalf("length: %v", n)
				}
				if !bytes.Equal(buf[:n], tc.testdata[:n]) {
					t.Fatalf("data: exptected %v, got %v", tc.testdata[:n], buf[:n])
				}
				tc.testdata = tc.testdata[n:]
			}
		})
	}
}
