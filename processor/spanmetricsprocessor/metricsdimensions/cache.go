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

// Cache is based on the typical use case of a Trie data structure that stores a dictionary
// of words where each node represents a letter in the word and a boolean flag in each node of the Trie
// to signal the end of a word. Using this analogy for Cache:
// - word = metric
// - letter = dimension
// - end word flag = nullable string value for the metric key. A nil value means there is no cached metric key.
//
// The motivation for a cache is to reduce the expensive and, likely, unnecessary serialization and
// deserialization cost of metrics keys, particularly given that (at least well-designed metrics
// with low cardinality dimensions) new metrics are created at a logarithmic rate such that the benefits
// of cached metrics will be realized more over time from an increasing number of cache hits.
//
// Cache assumes that list of dimension values passed into each call of InsertDimensions is in the same order.
// Each subsequent call to InsertDimensions will traverse down a level of the Trie which is associated with the
// dimension name such as "serviceName" or "operation".
// This assumption is appropriate for the spanmetricsprocessor because the dimensions consist of a known hard-coded
// list of serviceName, operation, spanKind and statusCode along with statically configured additional dimensions.
//
// BenchmarkProcessorConsumeTraces was used to measure the performance gains of using Cache, with a
// 30% improvement on average on the ConsumeTraces call at 100% cache hits, and interestingly a much lower
// standard deviation implying a more stable performance footprint.
// Naturally, this is a trade-off with additional memory used to store the Trie. It is the responsibility of users
// to configure sensible dimensions with low-cardinality, which is a good practice for metrics in general.
type Cache struct {
	root *CacheNode
}

// CacheNode represents a node in the Cache Trie.
type CacheNode struct {
	children  map[string]*CacheNode
	metricKey *string
}

// DimensionKeyValue stores the a dimension's key value pair such as: serviceName -> myService.
type DimensionKeyValue struct {
	Key   string
	Value string
}

// NewCache creates a new Cache.
func NewCache() *Cache {
	return &Cache{
		root: &CacheNode{children: make(map[string]*CacheNode)},
	}
}

// Empty returns whether if any metrics dimensions exist.
func (c *Cache) Empty() bool {
	return len(c.root.children) == 0
}

// InsertDimensions inserts dimension key-value pairs from the Cache starting from the root CacheNode.
func (c *Cache) InsertDimensions(keyValues ...DimensionKeyValue) *CacheNode {
	return c.root.InsertDimensions(keyValues...)
}

// InsertDimensions inserts the list of dimension values into the Cache returning the last dimension inserted
// into the metric. The last metric is where the serialized representation of the metric's dimensions can be
// saved via a call to SetCachedMetricKey.
func (n *CacheNode) InsertDimensions(keyValues ...DimensionKeyValue) *CacheNode {
	child := n
	for _, k := range keyValues {
		child = child.insertNextDimension(k)
	}
	return child
}

// insertNextDimension inserts the given DimensionKeyValue's value into the cache.
// insertNextDimension will return the CacheNode representing the given DimensionKeyValue.
func (n *CacheNode) insertNextDimension(keyValue DimensionKeyValue) *CacheNode {
	if _, ok := n.children[keyValue.Value]; !ok {
		n.children[keyValue.Value] = &CacheNode{children: make(map[string]*CacheNode)}
	}
	return n.children[keyValue.Value]
}

// HasCachedMetricKey returns whether if a cached metric key exists on this CacheNode.
// That is, if this is the last dimension of a metric, and hence forms the complete metric.
func (n *CacheNode) HasCachedMetricKey() bool {
	return n.metricKey != nil
}

// GetCachedMetricKey gets the cached metric key.
func (n *CacheNode) GetCachedMetricKey() string {
	return *n.metricKey
}

// SetCachedMetricKey sets the cached metric key on this CacheNode.
func (n *CacheNode) SetCachedMetricKey(metricKey string) {
	n.metricKey = &metricKey
}
