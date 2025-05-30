//go:build !mips64

#include "go_asm.h"
#include "funcdata.h"
#include "textflag.h"

// func Writeback(addr uintptr, length uint)
TEXT ·Writeback(SB),NOSPLIT|NOFRAME,$0-16
	RET


// func Invalidate(addr uintptr, length uint)
TEXT ·Invalidate(SB),NOSPLIT|NOFRAME,$0-16
	RET
