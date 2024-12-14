package periph_test

import (
	"bytes"
	"io"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/clktmr/n64/drivers/carts/isviewer"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

// Used as a reference implementation, should have the same behaviour as
// periph.Device.
type bytesReadWriter struct {
	bytes.Reader
	buf []byte
}

func newBytesReadWriter(b []byte) *bytesReadWriter {
	return &bytesReadWriter{
		Reader: *bytes.NewReader(b),
		buf:    b,
	}
}

func (b *bytesReadWriter) WriteAt(p []byte, offset int64) (n int, err error) {
	n = copy(b.buf[offset:], p)
	if n < len(p) {
		err = periph.ErrEndOfDevice
	}
	return
}

// Use end of ISViewer buffer for testing
var dut = periph.NewDevice(0x13ff_fe00, 64)
var ref = newBytesReadWriter(make([]byte, 64, 64))
var initBytes = make([]byte, 64, 64)

func TestReaderWriterAt(t *testing.T) {
	if isviewer.Probe() == nil {
		t.Skip("needs ISViewer")
	}

	for i, _ := range initBytes {
		initBytes[i] = byte(i + 0x30)
	}

	var (
		even     = []byte("evenlenght")
		odd      = []byte("oddlenght")
		evenLong = []byte("text longer than a cacheline with even length.")
		oddLong  = []byte("text longer than a cacheline with odd length.")
	)

	// Define testcases
	tests := map[string]params{
		"noop":               {0, []byte{}},
		"paddedEven":         {0, cpu.CopyPaddedSlice(even)},
		"paddedOdd":          {0, cpu.CopyPaddedSlice(odd)},
		"unpaddedEven":       {0, even},
		"unpaddedOdd":        {0, odd},
		"paddedEvenLong":     {0, cpu.CopyPaddedSlice(evenLong)},
		"paddedOddLong":      {0, cpu.CopyPaddedSlice(oddLong)},
		"unpaddedEvenLong":   {0, evenLong},
		"unpaddedOddLong":    {0, oddLong},
		"noCacheAlignEven":   {0, cpu.CopyPaddedSlice(evenLong)[4:]},
		"noCacheAlignOdd":    {0, cpu.CopyPaddedSlice(oddLong)[4:]},
		"noPIBusAlignEven":   {0, cpu.CopyPaddedSlice(oddLong)[3:]},
		"noPIBusAlignOdd":    {0, cpu.CopyPaddedSlice(evenLong)[3:]},
		"paddedEvenOffset":   {1, cpu.CopyPaddedSlice(even)},
		"paddedOddOffset":    {1, cpu.CopyPaddedSlice(odd)},
		"unpaddedEvenOffset": {1, even},
		"unpaddedOddOffset":  {1, odd},
	}

	// Run all testcases
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resultRef := testWriterAt(t, tc, ref)
			resultDut := testWriterAt(t, tc, dut)

			if bytes.Compare(resultRef.Bytes(), resultDut.Bytes()) != 0 {
				t.Error("write not equal")
				t.Log("Ref:", resultRef.String())
				t.Log("Dut:", resultDut.String())
			}

			resultRef = testReaderAt(t, tc, ref)
			resultDut = testReaderAt(t, tc, dut)

			if bytes.Compare(resultRef.Bytes(), resultDut.Bytes()) != 0 {
				t.Error("read not equal")
				t.Log("Ref:", resultRef.String())
				t.Log("Dut:", resultDut.String())
			}
		})
	}
}

type params struct {
	offset int64
	data   []byte
}

func testWriterAt(t *testing.T, tc params, dut io.WriterAt) *bytes.Buffer {
	n, err := dut.WriteAt(initBytes, 0)
	if err != nil {
		t.Error("copy init:", err, n)
	}

	dut.WriteAt(tc.data, tc.offset)

	result := bytes.NewBuffer(make([]byte, 64, 64))
	nc, err := io.Copy(result, io.NewSectionReader(dut.(io.ReaderAt), 0, 64))
	if err != nil {
		t.Error("copy result:", err, nc)
	}

	return result
}

func testReaderAt(t *testing.T, tc params, dut io.ReaderAt) *bytes.Buffer {
	n, err := dut.(io.WriterAt).WriteAt(initBytes, 0)
	if err != nil {
		t.Error("copy init:", err, n)
	}

	dut.ReadAt(tc.data, tc.offset)

	result := bytes.NewBuffer(make([]byte, 64, 64))
	nc, err := io.Copy(result, bytes.NewReader(tc.data))
	if err != nil {
		t.Error("copy result:", err, nc)
	}
	return result
}

func TestReadWriteIO(t *testing.T) {
	if isviewer.Probe() == nil {
		t.Skip("needs ISViewer")
	}

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

				periph.WriteIO(0x13ff_fe00, initBytes)

				tx := txbuf[sliceAlign : sliceAlign+sliceLen]
				copy(tx, testdata)
				periph.WriteIO(0x13ff_fe00+cpu.Addr(busAlign), tx)

				rx := rxbuf[sliceAlign : sliceAlign+sliceLen]
				periph.ReadIO(0x13ff_fe00+cpu.Addr(busAlign), rx)

				if !bytes.Equal(tx, rx) {
					t.Logf("tx %q", string(tx))
					t.Logf("rx %q", string(rx))
					t.Error("mismatch at ", busAlign, sliceAlign, sliceLen)
				}

				periph.ReadIO(0x13ff_fe00, rxbuf)
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

const lorem = `Lorem ipsum dolor sit amet, consectetur adipisici elit, sed
eiusmod tempor incidunt ut labore et dolore magna aliqua. Ut enim ad
minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquid
ex ea commodi consequat. Quis aute iure reprehenderit in voluptate velit
esse cillum dolore eu fugiat nulla pariatur. Excepteur sint obcaecat
cupiditat non proident, sunt in culpa qui officia deserunt mollit anim
id est laborum.`

func TestConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	const devSize = 1024
	devs := [...]*periph.Device{
		periph.NewDevice(0x13fffc00, devSize),
		periph.NewDevice(0x13fff800, devSize),
		periph.NewDevice(0x13fff400, devSize),
		periph.NewDevice(0x13fff000, devSize),
	}

	var wg sync.WaitGroup
	for _, dev := range devs {
		dev := dev
		wg.Add(1)
		go func() {
			timer := time.NewTimer(5 * time.Second)
			exit := false
			for !exit {
				offset := int64(rand.Intn(devSize - len(lorem)))
				_, err := dev.WriteAt([]byte(lorem), offset)
				if err != nil {
					t.Error(err)
				}
				buf := make([]byte, len(lorem))
				_, err = dev.ReadAt(buf, offset)
				if err != nil {
					t.Error(err)
				}
				if !bytes.Equal(buf, []byte(lorem)) {
					t.Error("read unexpected data")
				}

				select {
				case <-timer.C:
					exit = true
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
