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

func TestMonotonicDiffUnknownStart(t *testing.T) {
	startTs := uint64(0) // equivalent to start being unset
	prevPts := newTestCache()
	_, ok := prevPts.MonotonicDiff("test", []string{}, startTs, 1, 5)
	assert.False(t, ok, "expected no diff: first point")
	_, ok = prevPts.MonotonicDiff("test", []string{}, startTs, 0, 0)
	assert.False(t, ok, "expected no diff: old point")
	_, ok = prevPts.MonotonicDiff("test", []string{}, startTs, 2, 2)
	assert.False(t, ok, "expected no diff: new < old")
	dx, ok := prevPts.MonotonicDiff("test", []string{}, startTs, 3, 4)
	assert.True(t, ok, "expected diff: no startTs, old >= new")
	assert.Equal(t, 2.0, dx, "expected diff 2.0 with (0,2,2) value")
}

func TestDiffUnknownStart(t *testing.T) {
	startTs := uint64(0) // equivalent to start being unset
	prevPts := newTestCache()
	_, ok := prevPts.Diff("test", []string{}, startTs, 1, 5)
	assert.False(t, ok, "expected no diff: first point")
	_, ok = prevPts.Diff("test", []string{}, startTs, 0, 0)
	assert.False(t, ok, "expected no diff: old point")
	dx, ok := prevPts.Diff("test", []string{}, startTs, 2, 2)
	assert.True(t, ok, "expected diff: no startTs, not monotonic")
	assert.Equal(t, -3.0, dx, "expected diff -3.0 with (0,1,5) value")
	dx, ok = prevPts.Diff("test", []string{}, startTs, 3, 4)
	assert.True(t, ok, "expected diff: no startTs, old >= new")
	assert.Equal(t, 2.0, dx, "expected diff 2.0 with (0,2,2) value")
}

func TestMonotonicDiffKnownStart(t *testing.T) {
	startTs := uint64(1)
	prevPts := newTestCache()
	_, ok := prevPts.MonotonicDiff("test", []string{}, startTs, 1, 5)
	assert.False(t, ok, "expected no diff: first point")
	_, ok = prevPts.MonotonicDiff("test", []string{}, startTs, 0, 0)
	assert.False(t, ok, "expected no diff: old point")
	_, ok = prevPts.MonotonicDiff("test", []string{}, startTs, 2, 2)
	assert.False(t, ok, "expected no diff: new < old")
	dx, ok := prevPts.MonotonicDiff("test", []string{}, startTs, 3, 4)
	assert.True(t, ok, "expected diff: same startTs, old >= new")
	assert.Equal(t, 2.0, dx, "expected diff 2.0 with (0,2,2) value")

	startTs = uint64(4) // simulate reset with startTs = ts
	_, ok = prevPts.MonotonicDiff("test", []string{}, startTs, startTs, 8)
	assert.False(t, ok, "expected no diff: reset with unknown start")
	dx, ok = prevPts.MonotonicDiff("test", []string{}, startTs, 5, 9)
	assert.True(t, ok, "expected diff: same startTs, old >= new")
	assert.Equal(t, 1.0, dx, "expected diff 1.0 with (4,4,8) value")

	startTs = uint64(6)
	_, ok = prevPts.MonotonicDiff("test", []string{}, startTs, 7, 1)
	assert.False(t, ok, "expected no diff: reset with known start")
	dx, ok = prevPts.MonotonicDiff("test", []string{}, startTs, 8, 10)
	assert.True(t, ok, "expected diff: same startTs, old >= new")
	assert.Equal(t, 9.0, dx, "expected diff 9.0 with (6,7,1) value")
}

func TestDiffKnownStart(t *testing.T) {
	startTs := uint64(1)
	prevPts := newTestCache()
	_, ok := prevPts.Diff("test", []string{}, startTs, 1, 5)
	assert.False(t, ok, "expected no diff: first point")
	_, ok = prevPts.Diff("test", []string{}, startTs, 0, 0)
	assert.False(t, ok, "expected no diff: old point")
	dx, ok := prevPts.Diff("test", []string{}, startTs, 2, 2)
	assert.True(t, ok, "expected diff: same startTs, not monotonic")
	assert.Equal(t, -3.0, dx, "expected diff -3.0 with (1,1,5) point")
	dx, ok = prevPts.Diff("test", []string{}, startTs, 3, 4)
	assert.True(t, ok, "expected diff: same startTs, not monotonic")
	assert.Equal(t, 2.0, dx, "expected diff 2.0 with (0,2,2) value")

	startTs = uint64(4) // simulate reset with startTs = ts
	_, ok = prevPts.Diff("test", []string{}, startTs, startTs, 8)
	assert.False(t, ok, "expected no diff: reset with unknown start")
	dx, ok = prevPts.Diff("test", []string{}, startTs, 5, 9)
	assert.True(t, ok, "expected diff: same startTs, not monotonic")
	assert.Equal(t, 1.0, dx, "expected diff 1.0 with (4,4,8) value")

	startTs = uint64(6)
	_, ok = prevPts.Diff("test", []string{}, startTs, 7, 1)
	assert.False(t, ok, "expected no diff: reset with known start")
	dx, ok = prevPts.Diff("test", []string{}, startTs, 8, 10)
	assert.True(t, ok, "expected diff: same startTs, not monotonic")
	assert.Equal(t, 9.0, dx, "expected diff 9.0 with (6,7,1) value")
}

func TestMetricDimensionsToMapKey(t *testing.T) {
	metricName := "metric.name"
	c := newTestCache()
	noTags := c.metricDimensionsToMapKey(metricName, []string{})
	someTags := c.metricDimensionsToMapKey(metricName, []string{"key1:val1", "key2:val2"})
	sameTags := c.metricDimensionsToMapKey(metricName, []string{"key2:val2", "key1:val1"})
	diffTags := c.metricDimensionsToMapKey(metricName, []string{"key3:val3"})

	assert.NotEqual(t, noTags, someTags)
	assert.NotEqual(t, someTags, diffTags)
	assert.Equal(t, someTags, sameTags)
}

func TestMetricDimensionsToMapKeyNoTagsChange(t *testing.T) {
	// The original metricDimensionsToMapKey had an issue where:
	// - if the capacity of the tags array passed to it was higher than its length
	// - and the metric name is earlier (in alphabetical order) than one of the tags
	// then the original tag array would be modified (without a reallocation, since there is enough capacity),
	// and would contain a tag labeled as the metric name, while the final tag (in alphabetical order)
	// would get left out.
	// This test checks that this doesn't happen anymore.

	metricName := "a.metric.name"
	c := newTestCache()

	originalTags := make([]string, 2, 3)
	originalTags[0] = "key1:val1"
	originalTags[1] = "key2:val2"
	c.metricDimensionsToMapKey(metricName, originalTags)
	assert.Equal(t, []string{"key1:val1", "key2:val2"}, originalTags)

}
