// Copyright The OpenTelemetry Authors
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

package countconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

var noAttributes = [16]byte{}

func newCounter[K any](metricDefs map[string]metricDef[K]) *counter[K] {
	return &counter[K]{
		metricDefs: metricDefs,
		counts:     make(map[string]map[[16]byte]*attrCounter, len(metricDefs)),
		timestamp:  time.Now(),
	}
}

type counter[K any] struct {
	metricDefs map[string]metricDef[K]
	counts     map[string]map[[16]byte]*attrCounter
	timestamp  time.Time
}

type attrCounter struct {
	attrs pcommon.Map
	count uint64
}

func (c *counter[K]) update(ctx context.Context, attrs pcommon.Map, tCtx K) error {
	var errors error

	for name, md := range c.metricDefs {
		countAttrs := pcommon.NewMap()
		for _, attr := range md.attrs {
			attrVal, ok := attrs.Get(attr.Key)
			if !ok && attr.MaxDepth > 1 {
				// couldn't find the attribute so check if it is nested when any_depth is true
				attrVal, ok = getNestedAttribute(attr.Key, attr.MaxDepth, attrs) // shadows the OK from above on purpose
			}
			if ok {
				countAttrs.PutStr(attr.Key, attrVal.Str())
			} else if attr.DefaultValue != "" {
				countAttrs.PutStr(attr.Key, attr.DefaultValue)
			}
		}

		// Missing necessary attributes to be counted
		if countAttrs.Len() != len(md.attrs) {
			continue
		}

		// No conditions, so match all.
		if md.condition == nil {
			errors = multierr.Append(errors, c.increment(name, countAttrs))
			continue
		}

		if match, err := md.condition.Eval(ctx, tCtx); err != nil {
			errors = multierr.Append(errors, err)
		} else if match {
			errors = multierr.Append(errors, c.increment(name, countAttrs))
		}
	}
	return errors
}

func (c *counter[K]) increment(metricName string, attrs pcommon.Map) error {
	if _, ok := c.counts[metricName]; !ok {
		c.counts[metricName] = make(map[[16]byte]*attrCounter)
	}

	key := noAttributes
	if attrs.Len() > 0 {
		key = pdatautil.MapHash(attrs)
	}

	if _, ok := c.counts[metricName][key]; !ok {
		c.counts[metricName][key] = &attrCounter{attrs: attrs}
	}

	c.counts[metricName][key].count++
	return nil
}

func (c *counter[K]) appendMetricsTo(metricSlice pmetric.MetricSlice) {
	for name, md := range c.metricDefs {
		if len(c.counts[name]) == 0 {
			continue
		}
		countMetric := metricSlice.AppendEmpty()
		countMetric.SetName(name)
		countMetric.SetDescription(md.desc)
		sum := countMetric.SetEmptySum()
		// The delta value is always positive, so a value accumulated downstream is monotonic
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		for _, dpCount := range c.counts[name] {
			dp := sum.DataPoints().AppendEmpty()
			dpCount.attrs.CopyTo(dp.Attributes())
			dp.SetIntValue(int64(dpCount.count))
			// TODO determine appropriate start time
			dp.SetTimestamp(pcommon.NewTimestampFromTime(c.timestamp))
		}
	}
}

func getNestedAttribute(attr string, depth int, attributes pcommon.Map) (pcommon.Value, bool) {
	logger := zap.NewExample()
	defer logger.Sync()
	left, right, _ := strings.Cut(attr, ".")
	av, ok := attributes.Get(left)
	if ok && depth > 0 {
		if av.Type().String() == "Slice" {
			logger.Warn("[connector/count] cannot use Type Slice as an attribute source", zap.String("attribute", attr))
			return pcommon.Value{}, false
		} else if len(right) == 0 {
			return av, ok
		}
	}

	if !ok {
		return pcommon.Value{}, false
	}

	return getNestedAttribute(right, depth-1, av.Map())
}
