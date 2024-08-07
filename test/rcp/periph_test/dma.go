package periph_test

import (
	"bytes"
	"io"
	"testing"

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

func (b *bytesReadWriter) Write(p []byte) (n int, err error) {
	offset, err := b.Reader.Seek(0, io.SeekCurrent)
	n = copy(b.buf[offset:], p)
	if n < len(p) {
		err = io.ErrShortWrite
	}
	b.Reader.Seek(int64(n), io.SeekCurrent)
	return
}

// Use end of ISViewer buffer for testing
var dut = periph.NewDevice(0x13ff_fe00, 64)
var ref = newBytesReadWriter(make([]byte, 64, 64))
var initReader *bytes.Reader

func TestReadWriteSeeker(t *testing.T) {
	var initBytes = make([]byte, 64, 64)
	for i, _ := range initBytes {
		initBytes[i] = byte(i + 0x30)
	}
	initReader = bytes.NewReader(initBytes)

	var (
		even     = []byte("evenlenght")
		odd      = []byte("oddlenght")
		evenLong = []byte("text longer than a cacheline with even length.")
		oddLong  = []byte("text longer than a cacheline with odd length.")
	)

	// Define testcases
	tests := map[string]params{
		"noop":                {0, 0, io.SeekStart, []byte{}},
		"paddedEven":          {1, 0, io.SeekStart, cpu.CopyPaddedSlice(even)},
		"paddedOdd":           {1, 0, io.SeekStart, cpu.CopyPaddedSlice(odd)},
		"unpaddedEven":        {1, 0, io.SeekStart, even},
		"unpaddedOdd":         {1, 0, io.SeekStart, odd},
		"paddedEvenLong":      {1, 0, io.SeekStart, cpu.CopyPaddedSlice(evenLong)},
		"paddedOddLong":       {1, 0, io.SeekStart, cpu.CopyPaddedSlice(oddLong)},
		"unpaddedEvenLong":    {1, 0, io.SeekStart, evenLong},
		"unpaddedOddLong":     {1, 0, io.SeekStart, oddLong},
		"noCacheAlignEven":    {1, 0, io.SeekStart, cpu.CopyPaddedSlice(evenLong)[4:]},
		"noCacheAlignOdd":     {1, 0, io.SeekStart, cpu.CopyPaddedSlice(oddLong)[4:]},
		"noPIBusAlignEven":    {1, 0, io.SeekStart, cpu.CopyPaddedSlice(oddLong)[3:]},
		"noPIBusAlignOdd":     {1, 0, io.SeekStart, cpu.CopyPaddedSlice(evenLong)[3:]},
		"paddedEvenSeekPos":   {4, 1, io.SeekCurrent, cpu.CopyPaddedSlice(even)},
		"paddedOddSeekPos":    {4, 1, io.SeekCurrent, cpu.CopyPaddedSlice(odd)},
		"unpaddedEvenSeekPos": {4, 1, io.SeekCurrent, even},
		"unpaddedOddSeekPos":  {4, 1, io.SeekCurrent, odd},
		"paddedEvenSeekNeg":   {4, -1, io.SeekCurrent, cpu.CopyPaddedSlice(even)},
		"paddedOddSeekNeg":    {4, -1, io.SeekCurrent, cpu.CopyPaddedSlice(odd)},
		"unpaddedEvenSeekNeg": {4, -1, io.SeekCurrent, even},
		"unpaddedOddSeekNeg":  {4, -1, io.SeekCurrent, odd},
		"paddedEvenSeekEnd":   {4, -31, io.SeekEnd, cpu.CopyPaddedSlice(even)},
		"paddedOddSeekEnd":    {4, -31, io.SeekEnd, cpu.CopyPaddedSlice(odd)},
		"unpaddedEvenSeekEnd": {4, -31, io.SeekEnd, even},
		"unpaddedOddSeekEnd":  {4, -31, io.SeekEnd, odd},
		"eof":                 {4, -1, io.SeekEnd, cpu.CopyPaddedSlice(evenLong)},
		"eofnoop":             {4, 0, io.SeekEnd, cpu.CopyPaddedSlice(evenLong)},
	}

	// Run all testcases
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resultRef := testWriteSeeker(t, tc, ref)
			resultDut := testWriteSeeker(t, tc, dut)

			if bytes.Compare(resultRef.buf, resultDut.buf) != 0 {
				t.Error("write not equal")
				t.Log("Ref:", string(resultRef.buf))
				t.Log("Dut:", string(resultDut.buf))
			}

			resultRef = testReadSeeker(t, tc, ref)
			resultDut = testReadSeeker(t, tc, dut)

			if bytes.Compare(resultRef.buf, resultDut.buf) != 0 {
				t.Error("read not equal")
				t.Log("Ref:", string(resultRef.buf))
				t.Log("Dut:", string(resultDut.buf))
			}
		})
	}
}

type params struct {
	repeat int
	offset int64
	whence int
	data   []byte
}

func testWriteSeeker(t *testing.T, tc params, dut io.ReadWriteSeeker) *bytesReadWriter {
	initReader.Seek(0, io.SeekStart)
	dut.Seek(0, io.SeekStart)
	n, err := io.Copy(dut, initReader)
	if n != 64 || err != nil {
		t.Error("copy init:", err, n)
	}

	dut.Seek(0, io.SeekStart)
	for range tc.repeat {
		dut.Seek(tc.offset, tc.whence)
		dut.Write(tc.data)
	}

	result := newBytesReadWriter(make([]byte, 64, 64))
	dut.Seek(0, io.SeekStart)
	n, err = io.Copy(result, dut)
	if n != 64 || err != nil {
		t.Error("copy result:", err, n)
	}

	return result
}

func testReadSeeker(t *testing.T, tc params, dut io.ReadWriteSeeker) *bytesReadWriter {
	initReader.Seek(0, io.SeekStart)
	dut.Seek(0, io.SeekStart)
	n, err := io.Copy(dut, initReader)
	if n != 64 || err != nil {
		t.Error("copy init:", err, n)
	}

	result := newBytesReadWriter(make([]byte, 64, 64))

	dut.Seek(0, io.SeekStart)
	for range tc.repeat {
		dut.Seek(tc.offset, tc.whence)
		dut.Read(tc.data)
		result.Write(tc.data)
	}

	return result
}
