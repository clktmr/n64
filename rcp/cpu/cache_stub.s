//go:build !mips64

#include "go_asm.h"
#include "funcdata.h"
#include "textflag.h"

// func writeback(addr uintptr, length uint)
TEXT ·writeback(SB),NOSPLIT|NOFRAME,$0-16
	RET


// func invalidate(addr uintptr, length uint)
TEXT ·invalidate(SB),NOSPLIT|NOFRAME,$0-16
	RET
