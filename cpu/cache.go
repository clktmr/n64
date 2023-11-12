// The CPU accesses RAM through a cache and in general assumes that there are no
// other readers or writers.  Since the stored value in the cache can divert
// from the stored value in RAM for a limited amount of time, we need to sync
// both before other comoponents are involved.
//
// All operations in this package refer to the data cache.  Instruction cache
// won't be changed.
package cpu

// Keep in mind that cache operations always affect a whole cache line.  To
// avoid invalidating unrelated data in a cache line, pad structs with
// CacheLineSize at the beginning and end.
const CacheLineSize = 16
const cacheLineMask = ^(CacheLineSize - 1)

// Causes the cache to be written back to RAM.  Call this before requesting
// another component to read from this address range.  If the specified address
// is currently not cached, this is a no-op.
func Writeback(addr uintptr, length int)

// Causes the cache to be read from RAM before next access.  Call this before
// reading the address range if it was written by another component.  If the
// specified address is currently not cached, this is a no-op.
func Invalidate(addr uintptr, length int)
