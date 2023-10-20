#include "go_asm.h"
#include "funcdata.h"
#include "textflag.h"

// func Writeback(addr uintptr, length uint)
TEXT ·Writeback(SB),NOSPLIT|NOFRAME,$0-16
	MOVV  addr+0(FP), R4
	MOVV  length+8(FP), R5
	ADDU  R5, R4, R8
	AND   $const_cacheLineMask, R4

loop:
	SUB   R4, R8, R9
	BLEZ  R9, done
	BREAK R25, 0(R4) // asm generates cache op
	ADDU  $const_CacheLineSize, R4
	JMP   loop

done:
	RET


// func Invalidate(addr uintptr, length uint)
TEXT ·Invalidate(SB),NOSPLIT|NOFRAME,$0-16
	MOVV  addr+0(FP), R4
	MOVV  length+8(FP), R5
	ADDU  R5, R4, R8
	AND   $const_cacheLineMask, R4

loop:
	SUB   R4, R8, R9
	BLEZ  R9, done
	BREAK R17, 0(R4) // asm generates cache op
	ADDU  $const_CacheLineSize, R4
	JMP   loop

done:
	RET
