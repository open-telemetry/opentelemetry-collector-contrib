// Copyright  The OpenTelemetry Authors
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

package translator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestWithAttributeMap(t *testing.T) {
	attributes := pdata.NewMapFromRaw(map[string]interface{}{
		"key1": "val1",
		"key2": "val2",
		"key3": "",
	})

	dims := Dimensions{}
	assert.ElementsMatch(t,
		dims.WithAttributeMap(attributes).tags,
		[...]string{"key1:val1", "key2:val2", "key3:n/a"},
	)
}

func TestMetricDimensionsString(t *testing.T) {
	getKey := func(name string, tags []string, host string) string {
		dims := Dimensions{name: name, tags: tags, host: host}
		return dims.String()
	}
	metricName := "metric.name"
	hostOne := "host-one"
	hostTwo := "host-two"
	noTags := getKey(metricName, []string{}, hostOne)
	someTags := getKey(metricName, []string{"key1:val1", "key2:val2"}, hostOne)
	sameTags := getKey(metricName, []string{"key2:val2", "key1:val1"}, hostOne)
	diffTags := getKey(metricName, []string{"key3:val3"}, hostOne)
	diffHost := getKey(metricName, []string{"key1:val1", "key2:val2"}, hostTwo)

	assert.NotEqual(t, noTags, someTags)
	assert.NotEqual(t, someTags, diffTags)
	assert.Equal(t, someTags, sameTags)
	assert.NotEqual(t, someTags, diffHost)
}

func TestMetricDimensionsStringNoTagsChange(t *testing.T) {
	// The original metricDimensionsToMapKey had an issue where:
	// - if the capacity of the tags array passed to it was higher than its length
	// - and the metric name is earlier (in alphabetical order) than one of the tags
	// then the original tag array would be modified (without a reallocation, since there is enough capacity),
	// and would contain a tag labeled as the metric name, while the final tag (in alphabetical order)
	// would get left out.
	// This test checks that this doesn't happen anymore.

	originalTags := make([]string, 2, 3)
	originalTags[0] = "key1:val1"
	originalTags[1] = "key2:val2"

	dims := Dimensions{
		name: "a.metric.name",
		tags: originalTags,
	}

	_ = dims.String()
	assert.Equal(t, []string{"key1:val1", "key2:val2"}, originalTags)

}

var testDims = Dimensions{
	name: "test.metric",
	tags: []string{"key:val"},
	host: "host",
}

func TestWithSuffix(t *testing.T) {
	dimsSuf1 := testDims.WithSuffix("suffixOne")
	dimsSuf2 := testDims.WithSuffix("suffixTwo")

	assert.Equal(t, "test.metric", testDims.name)
	assert.Equal(t, "test.metric.suffixOne", dimsSuf1.name)
	assert.Equal(t, "test.metric.suffixTwo", dimsSuf2.name)
}

func TestAddTags(t *testing.T) {
	dimsWithTags := testDims.AddTags("key1:val1", "key2:val2")
	assert.ElementsMatch(t, []string{"key:val", "key1:val1", "key2:val2"}, dimsWithTags.tags)
	assert.ElementsMatch(t, []string{"key:val"}, testDims.tags)
}

func TestAllFieldsAreCopied(t *testing.T) {
	dims := &Dimensions{
		name:     "example.name",
		host:     "hostname",
		tags:     []string{"tagOne:a", "tagTwo:b"},
		originID: "origin_id",
	}

	newDims := dims.
		AddTags("tagThree:c").
		WithSuffix("suffix").
		WithAttributeMap(pdata.NewMapFromRaw(map[string]interface{}{
			"tagFour": "d",
		}))

	assert.Equal(t, "example.name.suffix", newDims.Name())
	assert.Equal(t, "hostname", newDims.Host())
	assert.ElementsMatch(t, []string{"tagOne:a", "tagTwo:b", "tagThree:c", "tagFour:d"}, newDims.Tags())
	assert.Equal(t, "origin_id", newDims.OriginID())
}
