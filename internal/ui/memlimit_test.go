package ui

import (
	"math"
	"testing"
)

func TestApplyDefaultMemoryLimitSetsDefaultWhenUnset(t *testing.T) {
	limit := int64(math.MaxInt64) // runtime sentinel for "no limit set"
	applyDefaultMemoryLimit(
		func() int64 { return limit },
		func(n int64) int64 { prev := limit; limit = n; return prev },
	)
	if limit != softMemoryLimitDefault {
		t.Fatalf("default not applied: got %d, want %d", limit, softMemoryLimitDefault)
	}
}

func TestApplyDefaultMemoryLimitLeavesOperatorLimitUntouched(t *testing.T) {
	const operatorLimit = 64 * bytesPerMiB
	limit := int64(operatorLimit)
	setCalled := false
	applyDefaultMemoryLimit(
		func() int64 { return limit },
		func(n int64) int64 { setCalled = true; prev := limit; limit = n; return prev },
	)
	if setCalled {
		t.Fatalf("default overrode operator limit; setLimit called, limit now %d", limit)
	}
	if limit != operatorLimit {
		t.Fatalf("operator limit changed: got %d, want %d", limit, operatorLimit)
	}
}
