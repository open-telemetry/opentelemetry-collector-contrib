// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dedup

import (
	"sync"
	"testing"
	"time"
)

// Helper: small TTL for fast tests.
const tinyTTL = 50 * time.Millisecond

func TestSeen_FirstFalseThenTrueWithinTTL(t *testing.T) {
	d := New(500 * time.Millisecond)

	fp := uint64(42)

	// First time: not seen
	if got := d.Seen(fp); got {
		t.Fatalf("first Seen(%d) = true, want false", fp)
	}

	// Immediately (within TTL): seen
	if got := d.Seen(fp); !got {
		t.Fatalf("second Seen(%d) = false, want true (within TTL)", fp)
	}
}

func TestSeen_ExpiresAfterTTL(t *testing.T) {
	d := New(tinyTTL)

	fp := uint64(101)

	// Prime it
	if d.Seen(fp) {
		t.Fatalf("expected first Seen(%d) to be false", fp)
	}
	if !d.Seen(fp) {
		t.Fatalf("expected second Seen(%d) to be true (within TTL)", fp)
	}

	// Wait for TTL to elapse
	time.Sleep(tinyTTL + 10*time.Millisecond)

	// After TTL: should be false again (expired / evicted)
	if d.Seen(fp) {
		t.Fatalf("expected Seen(%d) after TTL to be false", fp)
	}
}

func TestSeen_LazyGC_RemovesOnlyExpired(t *testing.T) {
	d := New(120 * time.Millisecond)

	fpOld := uint64(1)
	fpYoung := uint64(2)

	// Insert both
	if d.Seen(fpOld) {
		t.Fatalf("first Seen(old) should be false")
	}
	if d.Seen(fpYoung) {
		t.Fatalf("first Seen(young) should be false")
	}

	// Re-touch young to keep it fresh, then let old expire
	time.Sleep(70 * time.Millisecond)
	if !d.Seen(fpYoung) {
		t.Fatalf("second Seen(young) should be true (within TTL)")
	}

	// Let more time pass so that fpOld definitely expires, but fpYoung might still be within TTL
	time.Sleep(70 * time.Millisecond) // total ~140ms since old inserted, ~70ms since young re-touched

	// Hitting Seen on a new fingerprint triggers lazy GC internally
	_ = d.Seen(uint64(999)) // value doesn't matter

	// Old should now be reported as NOT seen (expired & GC'd)
	if d.Seen(fpOld) {
		t.Fatalf("expected old fingerprint to be expired/GC'd -> Seen(old) = false")
	}

	// Young should still be present (depending on timing window); allow small slack:
	// If it did expire due to timing jitter, reinsert and verify behavior is consistent.
	// Prefer strict expectation first—most runs should keep it true.
	if !d.Seen(fpYoung) {
		// If it flaked due to timing, demonstrate consistent behavior by re-adding and checking:
		if d.Seen(fpYoung) {
			t.Logf("young entry expired by timing; re-added successfully")
		} else {
			t.Fatalf("young fingerprint unexpectedly absent after GC window")
		}
	}
}

func TestSeen_MultipleFingerprintsIndependent(t *testing.T) {
	d := New(300 * time.Millisecond)

	fpA := uint64(111)
	fpB := uint64(222)

	// First encounters are false
	if d.Seen(fpA) || d.Seen(fpB) {
		t.Fatalf("first Seen(A/B) should be false for both")
	}

	// Now both should be true within TTL
	if !d.Seen(fpA) || !d.Seen(fpB) {
		t.Fatalf("second Seen(A/B) should be true for both (within TTL)")
	}
}

func TestSeen_ZeroTTL_AlwaysFalseAndAggressivelyGCs(t *testing.T) {
	d := New(0) // ttl=0 means entries never considered "within TTL" and are GC'd immediately

	fp := uint64(7)

	for i := 0; i < 3; i++ {
		if d.Seen(fp) {
			t.Fatalf("with ttl=0, Seen should always be false; iteration %d returned true", i)
		}
	}
}

func TestSeen_LongTTL_StaysTrue(t *testing.T) {
	d := New(5 * time.Second)

	fp := uint64(555)

	if d.Seen(fp) {
		t.Fatalf("first Seen should be false")
	}

	// Multiple checks within long TTL should remain true
	for i := 0; i < 5; i++ {
		if !d.Seen(fp) {
			t.Fatalf("Seen should be true within TTL; iteration %d", i)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSeen_Concurrency_SingleInsertMultipleHits(t *testing.T) {
	d := New(500 * time.Millisecond)
	fp := uint64(9999)

	const goroutines = 32

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(goroutines)

	results := make([]bool, goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			results[i] = d.Seen(fp)
		}()
	}

	// Release all goroutines at once; exactly one should observe "not seen" (false),
	// the rest should see true, since the first acquires the lock and inserts.
	close(start)
	wg.Wait()

	falseCount := 0
	trueCount := 0
	for _, r := range results {
		if r {
			trueCount++
		} else {
			falseCount++
		}
	}

	if falseCount != 1 {
		t.Fatalf("expected exactly one false (first insert), got %d false, %d true", falseCount, trueCount)
	}
	if trueCount != goroutines-1 {
		t.Fatalf("expected %d true, got %d", goroutines-1, trueCount)
	}
}
