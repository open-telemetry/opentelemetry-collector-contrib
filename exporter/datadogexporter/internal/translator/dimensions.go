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

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/translator"

import (
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/translator/utils"
)

const (
	dimensionSeparator = string(byte(0))
)

type metricsDimensions struct {
	name string
	tags []string
	host string
}

// getTags maps an attributeMap into a slice of Datadog tags
func getTags(labels pdata.AttributeMap) []string {
	tags := make([]string, 0, labels.Len())
	labels.Range(func(key string, value pdata.AttributeValue) bool {
		v := value.AsString()
		tags = append(tags, utils.FormatKeyValueTag(key, v))
		return true
	})
	return tags
}

// AddTags to metrics dimensions.
func (m *metricsDimensions) AddTags(tags ...string) metricsDimensions {
	// defensively copy the tags
	newTags := make([]string, 0, len(tags)+len(m.tags))
	newTags = append(newTags, tags...)
	newTags = append(newTags, m.tags...)
	return metricsDimensions{
		name: m.name,
		tags: newTags,
		host: m.host,
	}
}

// WithAttributeMap creates a new metricDimensions struct with additional tags from attributes.
func (m *metricsDimensions) WithAttributeMap(labels pdata.AttributeMap) metricsDimensions {
	return m.AddTags(getTags(labels)...)
}

// WithSuffix creates a new dimensions struct with an extra name suffix.
func (m *metricsDimensions) WithSuffix(suffix string) metricsDimensions {
	return metricsDimensions{
		name: fmt.Sprintf("%s.%s", m.name, suffix),
		host: m.host,
		tags: m.tags,
	}
}

// Uses a logic similar to what is done in the span processor to build metric keys:
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/b2327211df976e0a57ef0425493448988772a16b/processor/spanmetricsprocessor/processor.go#L353-L387
// TODO: make this a public util function?
func concatDimensionValue(metricKeyBuilder *strings.Builder, value string) {
	metricKeyBuilder.WriteString(value)
	metricKeyBuilder.WriteString(dimensionSeparator)
}

// String maps dimensions to a string to use as an identifier.
// The tags order does not matter.
func (m *metricsDimensions) String() string {
	var metricKeyBuilder strings.Builder

	dimensions := make([]string, len(m.tags))
	copy(dimensions, m.tags)

	dimensions = append(dimensions, fmt.Sprintf("name:%s", m.name))
	dimensions = append(dimensions, fmt.Sprintf("host:%s", m.host))
	sort.Strings(dimensions)

	for _, dim := range dimensions {
		concatDimensionValue(&metricKeyBuilder, dim)
	}
	return metricKeyBuilder.String()
}
