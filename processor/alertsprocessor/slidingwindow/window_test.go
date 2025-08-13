package slidingwindow

import (
	"testing"
	"time"
)

// helper to build a tm[T]
func stamp[T any](ts time.Time, v T) tm[T] { return tm[T]{at: ts, v: v} }

func TestCrop_TimeTrim(t *testing.T) {
	now := time.Now()

	buf := []tm[int]{
		stamp(now.Add(-10*time.Second), 1), // should be trimmed
		stamp(now.Add(-6*time.Second), 2),  // should be trimmed
		stamp(now.Add(-4*time.Second), 3),  // keep
		stamp(now.Add(-1*time.Second), 4),  // keep
	}

	cut := now.Add(-5 * time.Second)
	out := crop(buf, cut, 0, "ring_buffer") // no size limit

	if len(out) != 2 {
		t.Fatalf("expected 2 items after time trim, got %d", len(out))
	}
	if out[0].v != 3 || out[1].v != 4 {
		t.Fatalf("unexpected remaining values: %v, %v (want 3,4)", out[0].v, out[1].v)
	}
}

func TestCrop_SizeTrim_RingBuffer(t *testing.T) {
	now := time.Now()

	// ascending timestamps; 5 items total
	buf := []tm[int]{
		stamp(now.Add(-5*time.Second), 1),
		stamp(now.Add(-4*time.Second), 2),
		stamp(now.Add(-3*time.Second), 3),
		stamp(now.Add(-2*time.Second), 4),
		stamp(now.Add(-1*time.Second), 5),
	}

	// keep all by time
	cut := now.Add(-6 * time.Second)

	out := crop(buf, cut, 3, "ring_buffer")
	if len(out) != 3 {
		t.Fatalf("expected 3 items after ring buffer trim, got %d", len(out))
	}
	// ring_buffer keeps the most recent N
	if out[0].v != 3 || out[1].v != 4 || out[2].v != 5 {
		t.Fatalf("unexpected order/values after ring buffer: %v,%v,%v (want 3,4,5)", out[0].v, out[1].v, out[2].v)
	}
}

func TestCrop_SizeTrim_DropNew(t *testing.T) {
	now := time.Now()

	buf := []tm[int]{
		stamp(now.Add(-5*time.Second), 1),
		stamp(now.Add(-4*time.Second), 2),
		stamp(now.Add(-3*time.Second), 3),
		stamp(now.Add(-2*time.Second), 4),
		stamp(now.Add(-1*time.Second), 5),
	}

	cut := now.Add(-6 * time.Second)

	out := crop(buf, cut, 3, "drop_new")
	if len(out) != 3 {
		t.Fatalf("expected 3 items after drop_new trim, got %d", len(out))
	}
	// drop_new keeps the earliest N (new items would be dropped)
	if out[0].v != 1 || out[1].v != 2 || out[2].v != 3 {
		t.Fatalf("unexpected order/values after drop_new: %v,%v,%v (want 1,2,3)", out[0].v, out[1].v, out[2].v)
	}
}

func TestCrop_TimeThenSize_BothApplied(t *testing.T) {
	now := time.Now()

	buf := []tm[string]{
		stamp(now.Add(-9*time.Second), "old1"), // trimmed by time
		stamp(now.Add(-8*time.Second), "old2"), // trimmed by time
		stamp(now.Add(-4*time.Second), "a"),
		stamp(now.Add(-3*time.Second), "b"),
		stamp(now.Add(-2*time.Second), "c"),
		stamp(now.Add(-1*time.Second), "d"),
	}

	// Trim everything older than 5s; leaves [a,b,c,d]
	cut := now.Add(-5 * time.Second)

	out := crop(buf, cut, 2, "ring_buffer")
	if len(out) != 2 {
		t.Fatalf("expected 2 items after combined trim, got %d", len(out))
	}
	// After time trim we have [a,b,c,d]; ring buffer keeps the last 2 -> [c,d]
	if out[0].v != "c" || out[1].v != "d" {
		t.Fatalf("unexpected final values: %q,%q (want c,d)", out[0].v, out[1].v)
	}
}
