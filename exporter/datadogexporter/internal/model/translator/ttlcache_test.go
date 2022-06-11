// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestCache() *ttlCache {
	cache := newTTLCache(1800, 3600)
	return cache
}

var dims = &Dimensions{name: "test"}

func TestMonotonicDiffUnknownStart(t *testing.T) {
	startTs := uint64(0) // equivalent to start being unset
	prevPts := newTestCache()
	_, ok := prevPts.MonotonicDiff(dims, startTs, 1, 5)
	assert.False(t, ok, "expected no diff: first point")
	_, ok = prevPts.MonotonicDiff(dims, startTs, 0, 0)
	assert.False(t, ok, "expected no diff: old point")
	_, ok = prevPts.MonotonicDiff(dims, startTs, 2, 2)
	assert.False(t, ok, "expected no diff: new < old")
	dx, ok := prevPts.MonotonicDiff(dims, startTs, 3, 4)
	assert.True(t, ok, "expected diff: no startTs, old >= new")
	assert.Equal(t, 2.0, dx, "expected diff 2.0 with (0,2,2) value")
}

func TestDiffUnknownStart(t *testing.T) {
	startTs := uint64(0) // equivalent to start being unset
	prevPts := newTestCache()
	_, ok := prevPts.Diff(dims, startTs, 1, 5)
	assert.False(t, ok, "expected no diff: first point")
	_, ok = prevPts.Diff(dims, startTs, 0, 0)
	assert.False(t, ok, "expected no diff: old point")
	dx, ok := prevPts.Diff(dims, startTs, 2, 2)
	assert.True(t, ok, "expected diff: no startTs, not monotonic")
	assert.Equal(t, -3.0, dx, "expected diff -3.0 with (0,1,5) value")
	dx, ok = prevPts.Diff(dims, startTs, 3, 4)
	assert.True(t, ok, "expected diff: no startTs, old >= new")
	assert.Equal(t, 2.0, dx, "expected diff 2.0 with (0,2,2) value")
}

func TestMonotonicDiffKnownStart(t *testing.T) {
	startTs := uint64(1)
	prevPts := newTestCache()
	_, ok := prevPts.MonotonicDiff(dims, startTs, 1, 5)
	assert.False(t, ok, "expected no diff: first point")
	_, ok = prevPts.MonotonicDiff(dims, startTs, 0, 0)
	assert.False(t, ok, "expected no diff: old point")
	_, ok = prevPts.MonotonicDiff(dims, startTs, 2, 2)
	assert.False(t, ok, "expected no diff: new < old")
	dx, ok := prevPts.MonotonicDiff(dims, startTs, 3, 4)
	assert.True(t, ok, "expected diff: same startTs, old >= new")
	assert.Equal(t, 2.0, dx, "expected diff 2.0 with (0,2,2) value")

	startTs = uint64(4) // simulate reset with startTs = ts
	_, ok = prevPts.MonotonicDiff(dims, startTs, startTs, 8)
	assert.False(t, ok, "expected no diff: reset with unknown start")
	dx, ok = prevPts.MonotonicDiff(dims, startTs, 5, 9)
	assert.True(t, ok, "expected diff: same startTs, old >= new")
	assert.Equal(t, 1.0, dx, "expected diff 1.0 with (4,4,8) value")

	startTs = uint64(6)
	_, ok = prevPts.MonotonicDiff(dims, startTs, 7, 1)
	assert.False(t, ok, "expected no diff: reset with known start")
	dx, ok = prevPts.MonotonicDiff(dims, startTs, 8, 10)
	assert.True(t, ok, "expected diff: same startTs, old >= new")
	assert.Equal(t, 9.0, dx, "expected diff 9.0 with (6,7,1) value")
}

func TestDiffKnownStart(t *testing.T) {
	startTs := uint64(1)
	prevPts := newTestCache()
	_, ok := prevPts.Diff(dims, startTs, 1, 5)
	assert.False(t, ok, "expected no diff: first point")
	_, ok = prevPts.Diff(dims, startTs, 0, 0)
	assert.False(t, ok, "expected no diff: old point")
	dx, ok := prevPts.Diff(dims, startTs, 2, 2)
	assert.True(t, ok, "expected diff: same startTs, not monotonic")
	assert.Equal(t, -3.0, dx, "expected diff -3.0 with (1,1,5) point")
	dx, ok = prevPts.Diff(dims, startTs, 3, 4)
	assert.True(t, ok, "expected diff: same startTs, not monotonic")
	assert.Equal(t, 2.0, dx, "expected diff 2.0 with (0,2,2) value")

	startTs = uint64(4) // simulate reset with startTs = ts
	_, ok = prevPts.Diff(dims, startTs, startTs, 8)
	assert.False(t, ok, "expected no diff: reset with unknown start")
	dx, ok = prevPts.Diff(dims, startTs, 5, 9)
	assert.True(t, ok, "expected diff: same startTs, not monotonic")
	assert.Equal(t, 1.0, dx, "expected diff 1.0 with (4,4,8) value")

	startTs = uint64(6)
	_, ok = prevPts.Diff(dims, startTs, 7, 1)
	assert.False(t, ok, "expected no diff: reset with known start")
	dx, ok = prevPts.Diff(dims, startTs, 8, 10)
	assert.True(t, ok, "expected diff: same startTs, not monotonic")
	assert.Equal(t, 9.0, dx, "expected diff 9.0 with (6,7,1) value")
}
