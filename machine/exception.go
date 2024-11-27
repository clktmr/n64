package machine

var excNames = [32]string{
	0:  "Interrupt",
	1:  "TLB Modification",
	2:  "TLB Miss (load)",
	3:  "TLB Miss (store)",
	4:  "Address Error (load)",
	5:  "Address Error (store)",
	6:  "Bus Error (instruction)",
	7:  "Bus Error (data)",
	8:  "Syscall",
	9:  "Breakpoint",
	10: "Reserved Instruction",
	11: "Coprocessor Unusable",
	12: "Arithmetic Overflow",
	13: "Trap",
	15: "Floating-Point",
	23: "Watch",
}

//go:nosplit
func Exception(cause, epc, status, badvaddr, ra uint64) {
	var buf [16]byte
	DefaultWrite(0, []byte("Unhandled "))
	DefaultWrite(0, []byte(excNames[cause>>2&31]))
	DefaultWrite(0, []byte(" Exception"))

	DefaultWrite(0, []byte("\ncause    0x"))
	DefaultWrite(0, itoa(buf[:], cause))
	DefaultWrite(0, []byte("\nepc      0x"))
	DefaultWrite(0, itoa(buf[:], epc))
	DefaultWrite(0, []byte("\nstatus   0x"))
	DefaultWrite(0, itoa(buf[:], status))
	DefaultWrite(0, []byte("\nbadvaddr 0x"))
	DefaultWrite(0, itoa(buf[:], badvaddr))
	DefaultWrite(0, []byte("\nra       0x"))
	DefaultWrite(0, itoa(buf[:], ra))
	DefaultWrite(0, []byte("\n"))
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
