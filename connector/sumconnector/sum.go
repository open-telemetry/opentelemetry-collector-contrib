// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/sumconnector"

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

var noAttributes = [16]byte{}

func newSummer[K any](metricDefs map[string]metricDef[K]) *summer[K] {
	return &summer[K]{
		metricDefs: metricDefs,
		sums:       make(map[string]map[[16]byte]*attrSummer, len(metricDefs)),
		timestamp:  time.Now(),
	}
}

type summer[K any] struct {
	metricDefs map[string]metricDef[K]
	sums       map[string]map[[16]byte]*attrSummer
	timestamp  time.Time
}

type attrSummer struct {
	attrs pcommon.Map
	sum   float64
}

func (c *summer[K]) update(ctx context.Context, attrs pcommon.Map, tCtx K) error {
	var multiError error
	for name, md := range c.metricDefs {
		sourceAttribute := md.sourceAttr
		sumAttrs := pcommon.NewMap()
		var sumVal float64

		// Get source attribute value
		if sourceAttrVal, ok := attrs.Get(sourceAttribute); ok {
			switch {
			case sourceAttrVal.Str() != "":
				sumVal, _ = strconv.ParseFloat(sourceAttrVal.Str(), 64)
			case sourceAttrVal.Double() != 0:
				sumVal = sourceAttrVal.Double()
			case sourceAttrVal.Int() != 0:
				sumVal = float64(sourceAttrVal.Int())
			}
		}

		// Get attribute values to include otherwise use default value
		for _, attr := range md.attrs {
			if attrVal, ok := attrs.Get(attr.Key); ok {
				switch {
				case attrVal.Str() != "":
					sumAttrs.PutStr(attr.Key, attrVal.Str())
				case attrVal.Double() != 0:
					sumAttrs.PutStr(attr.Key, fmt.Sprintf("%v", attrVal.Double()))
				case attrVal.Int() != 0:
					sumAttrs.PutStr(attr.Key, fmt.Sprintf("%v", attrVal.Int()))
				}
			} else if attr.DefaultValue != nil {
				switch v := attr.DefaultValue.(type) {
				case string:
					if v != "" {
						sumAttrs.PutStr(attr.Key, v)
					}
				case int:
					if v != 0 {
						sumAttrs.PutInt(attr.Key, int64(v))
					}
				case float64:
					if v != 0 {
						sumAttrs.PutDouble(attr.Key, float64(v))
					}
				}
			}
		}

		// Missing necessary attributes
		if sumAttrs.Len() != len(md.attrs) {
			continue
		}

		// Perform condition matching or not
		if md.condition == nil {
			multiError = errors.Join(multiError, c.increment(name, sumVal, sumAttrs))
			continue
		}

		if match, err := md.condition.Eval(ctx, tCtx); err != nil {
			multiError = errors.Join(multiError, err)
		} else if match {
			multiError = errors.Join(multiError, c.increment(name, sumVal, sumAttrs))
		}
	}
	return multiError
}

func (c *summer[K]) increment(metricName string, sumVal float64, attrs pcommon.Map) error {
	if _, ok := c.sums[metricName]; !ok {
		c.sums[metricName] = make(map[[16]byte]*attrSummer)
	}

	key := noAttributes
	if attrs.Len() > 0 {
		key = pdatautil.MapHash(attrs)
	}

	if _, ok := c.sums[metricName][key]; !ok {
		c.sums[metricName][key] = &attrSummer{attrs: attrs}
	}

	for strings := range c.sums[metricName][key].attrs.AsRaw() {
		if _, ok := c.sums[metricName][key].attrs.Get(strings); ok {
			c.sums[metricName][key].sum += sumVal
		}
	}

	if attrs.Len() == 0 {
		c.sums[metricName][key].sum += sumVal
	}

	return nil
}

func (c *summer[K]) appendMetricsTo(metricSlice pmetric.MetricSlice) {
	for name, md := range c.metricDefs {
		if len(c.sums[name]) == 0 {
			continue
		}
		sumMetric := metricSlice.AppendEmpty()
		sumMetric.SetName(name)
		sumMetric.SetDescription(md.desc)
		sum := sumMetric.SetEmptySum()
		// The delta value is always positive, so a value accumulated downstream is monotonic
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		for _, dpSum := range c.sums[name] {
			dp := sum.DataPoints().AppendEmpty()
			dpSum.attrs.CopyTo(dp.Attributes())
			dp.SetDoubleValue(dpSum.sum)
			dp.SetTimestamp(pcommon.NewTimestampFromTime(c.timestamp))
		}
	}
}
