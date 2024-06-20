#include "go_asm.h"
#include "funcdata.h"
#include "textflag.h"

#include "asm_mips64.h"

TEXT machine·unhandledException(SB),NOSPLIT|NOFRAME,$0
	MOVV  M(C0_CAUSE), R26
	MOVV  R26, 8(R29)
	MOVV  M(C0_EPC), R26
	MOVV  R26, 16(R29)
	MOVV  M(C0_SR), R26
	MOVV  R26, 24(R29)
	MOVV  M(C0_BADVADDR), R26
	MOVV  R26, 32(R29)
	MOVV  R31, 40(R29)

	JAL ·Exception(SB)
	NOOP
	JMP -1(PC)
