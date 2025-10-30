// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package countconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	utilattri "github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

var noAttributes = [16]byte{}

func newCounter[K any](metricDefs map[string]metricDef[K]) *counter[K] {
	return &counter[K]{
		metricDefs: metricDefs,
		counts:     make(map[string]map[[16]byte]*attrCounter, len(metricDefs)),
	}
}

type counter[K any] struct {
	metricDefs map[string]metricDef[K]
	counts     map[string]map[[16]byte]*attrCounter
	startTime  pcommon.Timestamp
	endTime    pcommon.Timestamp
}

type attrCounter struct {
	attrs pcommon.Map
	count uint64
}

func (c *counter[K]) update(ctx context.Context, attrs, scopeAttrs, resourceAttrs pcommon.Map, tCtx K) error {
	var multiError error
	for name, md := range c.metricDefs {
		countAttrs := pcommon.NewMap()
		for _, attr := range md.attrs {
			dimension := utilattri.Dimension{
				Name: attr.Key,
				Value: func() *pcommon.Value {
					if attr.DefaultValue != nil {
						switch v := attr.DefaultValue.(type) {
						case string:
							if v != "" {
								strV := pcommon.NewValueStr(v)
								return &strV
							}
						case int:
							if v != 0 {
								intV := pcommon.NewValueInt(int64(v))
								return &intV
							}
						case float64:
							if v != 0 {
								floatV := pcommon.NewValueDouble(v)
								return &floatV
							}
						}
					}

					return nil
				}(),
			}
			value, ok := utilattri.GetDimensionValue(dimension, attrs, scopeAttrs, resourceAttrs)
			if ok {
				switch value.Type() {
				case pcommon.ValueTypeInt:
					countAttrs.PutInt(attr.Key, value.Int())
				case pcommon.ValueTypeDouble:
					countAttrs.PutDouble(attr.Key, value.Double())
				default:
					countAttrs.PutStr(attr.Key, value.Str())
				}
			}
		}

		// Missing necessary attributes to be counted
		if countAttrs.Len() != len(md.attrs) {
			continue
		}

		// No conditions, so match all.
		if md.condition == nil {
			multiError = errors.Join(multiError, c.increment(name, countAttrs))
			continue
		}

		if match, err := md.condition.Eval(ctx, tCtx); err != nil {
			multiError = errors.Join(multiError, err)
		} else if match {
			multiError = errors.Join(multiError, c.increment(name, countAttrs))
		}
	}
	return multiError
}

// updateTimestamp updates the start and end timestamps based on the provided timestamp
func (c *counter[K]) updateTimestamp(timestamp pcommon.Timestamp) {
	if timestamp != 0 {
		if c.startTime == 0 {
			c.endTime = timestamp
			c.startTime = timestamp
		} else {
			if timestamp < c.startTime {
				c.startTime = timestamp
			}
			if timestamp > c.endTime {
				c.endTime = timestamp
			}
		}
	}
}

// getTimestamps either gets the valid start and end timestamps or returns the current time
func (c *counter[K]) getTimestamps() (pcommon.Timestamp, pcommon.Timestamp) {
	if c.startTime != 0 {
		return c.startTime, c.endTime
	}
	now := pcommon.NewTimestampFromTime(time.Now())
	return now, now
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
			startTime, endTime := c.getTimestamps()
			dp.SetStartTimestamp(startTime)
			dp.SetTimestamp(endTime)
		}
	}
}
