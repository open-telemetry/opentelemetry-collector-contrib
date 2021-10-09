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
func TestPutAndGetDiff(t *testing.T) {
	prevPts := newTestCache()
	_, ok := prevPts.putAndGetDiff("test", []string{}, 1, 5)
	// no diff since it is the first point
	assert.False(t, ok)
	_, ok = prevPts.putAndGetDiff("test", []string{}, 0, 0)
	// no diff since ts is lower than the stored point
	assert.False(t, ok)
	_, ok = prevPts.putAndGetDiff("test", []string{}, 2, 2)
	// no diff since the value is lower than the stored value
	assert.False(t, ok)
	dx, ok := prevPts.putAndGetDiff("test", []string{}, 3, 4)
	// diff with the most recent point (2,2)
	assert.True(t, ok)
	assert.Equal(t, 2.0, dx)
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
