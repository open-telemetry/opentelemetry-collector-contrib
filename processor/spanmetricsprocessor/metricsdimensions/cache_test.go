// Copyright -c Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metricsdimensions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDimensionCache(t *testing.T) {
	dc := NewCache()
	assert.True(t, dc.Empty())

	// Insert dimensions into the trie.
	// Exercise inserting dimensions from both the trie as well as trie nodes.
	d := dc.InsertDimensions(DimensionKeyValue{"a", "foo"})
	d = d.InsertDimensions(DimensionKeyValue{"b", "bar"})

	// After inserting all the dimensions, verify that no cached key is present.
	assert.False(t, d.HasCachedMetricKey())

	// Store a JSON-serialized key in cache.
	jsonKey := `{"a": "foo", "b": "bar"}`
	d.SetCachedMetricKey(jsonKey)

	// Simulate building the metrics dimension key again.
	d = dc.InsertDimensions([]DimensionKeyValue{
		{"a", "foo"},
		{"b", "bar"},
	}...)

	// Verify the key was found in cache.
	assert.True(t, d.HasCachedMetricKey())
	assert.Equal(t, jsonKey, d.GetCachedMetricKey())
}
