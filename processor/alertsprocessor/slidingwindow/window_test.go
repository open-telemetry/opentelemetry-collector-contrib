// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package slidingwindow

import (
	"testing"
	"time"
)

func stamp[T any](ts time.Time, v T) tm[T] { return tm[T]{at: ts, v: v} }

func TestCrop_TimeTrim(t *testing.T) {
	now := time.Now()

	buf := []tm[int]{
		stamp(now.Add(-10*time.Second), 1), // trimmed
		stamp(now.Add(-6*time.Second), 2),  // trimmed
		stamp(now.Add(-4*time.Second), 3),  // keep
		stamp(now.Add(-1*time.Second), 4),  // keep
	}

	cut := now.Add(-5 * time.Second)
	out := crop(buf, cut, 0, "ring_buffer")

	if len(out) != 2 || out[0].v != 3 || out[1].v != 4 {
		t.Fatalf("unexpected result after time trim: %+v", out)
	}
}

func TestCrop_SizeTrim_RingBuffer(t *testing.T) {
	now := time.Now()
	buf := []tm[int]{
		stamp(now.Add(-5*time.Second), 1),
		stamp(now.Add(-4*time.Second), 2),
		stamp(now.Add(-3*time.Second), 3),
		stamp(now.Add(-2*time.Second), 4),
		stamp(now.Add(-1*time.Second), 5),
	}
	out := crop(buf, now.Add(-6*time.Second), 3, "ring_buffer")
	if len(out) != 3 || out[0].v != 3 || out[1].v != 4 || out[2].v != 5 {
		t.Fatalf("unexpected ring buffer output: %+v", out)
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
	out := crop(buf, now.Add(-6*time.Second), 3, "drop_new")
	if len(out) != 3 || out[0].v != 1 || out[1].v != 2 || out[2].v != 3 {
		t.Fatalf("unexpected drop_new output: %+v", out)
	}
}

func TestCrop_TimeThenSize(t *testing.T) {
	now := time.Now()
	buf := []tm[string]{
		stamp(now.Add(-9*time.Second), "old1"),
		stamp(now.Add(-8*time.Second), "old2"),
		stamp(now.Add(-4*time.Second), "a"),
		stamp(now.Add(-3*time.Second), "b"),
		stamp(now.Add(-2*time.Second), "c"),
		stamp(now.Add(-1*time.Second), "d"),
	}
	out := crop(buf, now.Add(-5*time.Second), 2, "ring_buffer")
	if len(out) != 2 || out[0].v != "c" || out[1].v != "d" {
		t.Fatalf("unexpected final output: %+v", out)
	}
}
