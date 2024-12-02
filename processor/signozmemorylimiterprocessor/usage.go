package signozmemorylimiterprocessor

import "runtime"

type memUsageChecker struct {
	memAllocLimit uint64
}

func newFixedMemUsageChecker(memAllocLimit uint64) *memUsageChecker {
	return &memUsageChecker{
		memAllocLimit: memAllocLimit,
	}
}

func (d memUsageChecker) aboveLimit(ms *runtime.MemStats) bool {
	return ms.Alloc >= d.memAllocLimit
}
