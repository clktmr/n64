#define C0_INDEX        0               /* Index of TLB Entry */
#define C0_ENTRYLO0     2               /* TLB entry's first PFN */
#define C0_ENTRYLO1     3               /* TLB entry's second PFN */
#define C0_PAGEMASK     5               /* Size of TLB Entries */
#define C0_BADVADDR     8               /* Address that occurred an error */
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


// Prepend NOOP to avert CP0 hazards
#define TLBWI NOOP; WORD $0x42000002

