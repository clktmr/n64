#include "go_asm.h"
#include "funcdata.h"
#include "textflag.h"

#define C0_INDEX        0               /* Index of TLB Entry */
#define C0_ENTRYLO0     2               /* TLB entry's first PFN */
#define C0_ENTRYLO1     3               /* TLB entry's second PFN */
#define C0_PAGEMASK     5               /* Size of TLB Entries */
#define C0_COUNT        9               /* Timer Count Register */
#define C0_ENTRYHI      10              /* VPN and ASID of two TLB entry */
#define C0_COMPARE      11              /* Timer Compare Register */
#define C0_SR           12              /* Status Register */
#define C0_CAUSE        13              /* last exception description */
#define C0_EPC          14              /* Exception error address */
#define C0_PRID         15              /* Processor Revision ID */
#define C0_CONFIG       16              /* CPU configuration */
#define C0_WATCHLO      18              /* Watchpoint */

#define SR_CU1          0x20000000      /* Mark CP1 as usable */
#define SR_FR           0x04000000      /* Enable MIPS III FP registers */
#define SR_BEV          0x00400000      /* Controls location of exception vectors */
#define SR_PE           0x00100000      /* Mark soft reset (clear parity error) */


#define DEFAULT_C0_SR SR_CU1|SR_PE|SR_FR

#define TLBWI WORD $0x42000002

TEXT machine·rt0(SB),NOSPLIT|NOFRAME,$0
	// Watchpoints have been proven to persist across resets and even with
	// the console being off. Zero it as early as possible, to avoid it
	// triggering during boot. This should really be done at the start IPL3.
	MOVW R0, M(C0_WATCHLO)

	JAL  ·rt0_tlb(SB)

	MOVW (0x80000318), R8 // memory size
	MOVV $0x10, R9
	SUBV R9, R8, R29 // init stack pointer
	MOVV $0, RSB // init data pointer
	MOVW $8, R2
	MOVW R2, (0xbfc007fc) // magic N64 hardware init

	// a bit from libgloss so we start at a known state
	MOVW $DEFAULT_C0_SR, R2
	MOVW R2, M(C0_SR)
	MOVW $0, M(C0_CAUSE)

	// Check if PI DMA transfer is required, knowing that IPL3 loads 1 MiB
	// of ROM to RAM, and __libdragon_text_start is located right after the
	// ROM header where this 1 MiB starts.
	// TODO test again with TLB mappings configured
	MOVW $_rt0_mips64_noos(SB), R4
	MOVW $runtime·edata(SB), R5
	MOVW $0x100000, R8 // stock IPL3 load size (1 MiB)
	SUBU R4, R5, R6	// calculate data size
	SUB  R8, R6, R6 // calculate remaining data size
	BLEZ R6, skip_dma // skip PI DMA if data is already loaded

	// Copy code and data via DMA
	MOVW $0x10001000, R5 // address in rom
	ADDU R8, R4, R4	// skip over loaded data
	ADDU R8, R5, R5				

	// Start PI DMA transfer
	MOVW $0xA4600000, R8
	MOVW R4, 0x00(R8) // PI_DRAM_ADDR
	MOVW R5, 0x04(R8) // PI_CART_ADDR
	ADD  $-1, R6
	MOVW R6, 0x0C(R8) // PI_WR_LEN

skip_dma:
	// Wait for DMA transfer to be finished
	MOVW $0xA4600000, R8
wait_dma_end:
	MOVW 0x10(R8), R9 // PI_STATUS
	AND  $3, R9 // PI_STATUS_DMA_BUSY | PI_STATUS_IO_BUSY
	BGTZ R9, wait_dma_end

	JMP runtime·_rt0_mips64_noos1(SB)

// Maps KSEG0, KSEG1 and ROM to the beginning of the virtual address space.
// This saves us from sign-extending pointers correctly, as we avoid pointers
// upwards of 0x80000000.
// The only problems encountered with falsely sign-extended pointers were symbol
// addresses loaded from the ELF.  Otherwise running code in KSEG0 seems to be
// no problem in general.
// The correct way of doing this would probably involve defining a new 64-bit
// architecture with PtrSize = 4, but I have ran into some issues that weren't
// worth fixing at the moment.
// TODO currently only 16 MiB of cartridge is mapped
TEXT ·rt0_tlb(SB),NOSPLIT|NOFRAME,$0
	MOVV $0, R8
	MOVV R8, M(C0_INDEX)
	MOVV $0xfff << 13, R8
	MOVV R8, M(C0_PAGEMASK)
	MOVV $(0x00000000 >> 6) | 0x7, R8
	MOVV R8, M(C0_ENTRYLO0)
	MOVV $(0x01000000 >> 6) | 0x7, R8
	MOVV R8, M(C0_ENTRYLO1)
	MOVV $0x00000000, R8
	MOVV R8, M(C0_ENTRYHI)
	NOOP // avert CP0 hazards
	TLBWI

	MOVV $1, R8
	MOVV R8, M(C0_INDEX)
	MOVV $0xfff << 13, R8
	MOVV R8, M(C0_PAGEMASK)
	MOVV $(0x10000000 >> 6) | (2<<3) |  0x3, R8
	MOVV R8, M(C0_ENTRYLO0)
	MOVV $(0x11000000 >> 6) | (2<<3) |  0x3, R8
	MOVV R8, M(C0_ENTRYLO1)
	MOVV $0x10000000, R8
	MOVV R8, M(C0_ENTRYHI)
	NOOP // avert CP0 hazards
	TLBWI

	MOVV $2, R8
	MOVV R8, M(C0_INDEX)
	MOVV $0xfff << 13, R8
	MOVV R8, M(C0_PAGEMASK)
	MOVV $(0x00000000 >> 6) | (2<<3) | 0x7, R8
	MOVV R8, M(C0_ENTRYLO0)
	MOVV $(0x01000000 >> 6) | (2<<3) | 0x7, R8
	MOVV R8, M(C0_ENTRYLO1)
	MOVV $0x20000000, R8
	MOVV R8, M(C0_ENTRYHI)
	NOOP // avert CP0 hazards
	TLBWI

	MOVV $0x7fffffff, R8
	AND  R8, R31 // return to the tlb mapped address
	RET

