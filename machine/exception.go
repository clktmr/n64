package machine

//go:nosplit
func Exception(cause, epc, status, badvaddr, sp uint64) {
	var buf [16]byte
	DefaultWrite(0, []byte("UNHANDLED EXCEPTION"))
	DefaultWrite(0, []byte("\ncause    0x"))
	DefaultWrite(0, itoa(buf[:], cause))
	DefaultWrite(0, []byte("\nepc      0x"))
	DefaultWrite(0, itoa(buf[:], epc))
	DefaultWrite(0, []byte("\nstatus   0x"))
	DefaultWrite(0, itoa(buf[:], status))
	DefaultWrite(0, []byte("\nbadvaddr 0x"))
	DefaultWrite(0, itoa(buf[:], badvaddr))
	DefaultWrite(0, []byte("\nsp       0x"))
	DefaultWrite(0, itoa(buf[:], sp))
}

//go:nosplit
func itoa(buf []byte, num uint64) []byte {
	for i := range 16 {
		char := byte(num>>(60-(4*i))) & 0xf
		if char > 9 {
			char += 'a' - 10
		} else {
			char += '0'
		}
		buf[i] = char
	}
	return buf
}
