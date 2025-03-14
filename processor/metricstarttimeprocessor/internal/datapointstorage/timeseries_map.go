// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapointstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/datapointstorage"

import (
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// AttributeHash is used to store a hash of attributes for a metric. See pdatautil.MapHash for more details.
type AttributeHash [16]byte

// TimeseriesInfo contains the information necessary to adjust from the initial point and to detect resets.
type TimeseriesInfo struct {
	Mark bool

	Number               pmetric.NumberDataPoint
	Histogram            pmetric.HistogramDataPoint
	ExponentialHistogram pmetric.ExponentialHistogramDataPoint
	Summary              pmetric.SummaryDataPoint
}

type TimeseriesKey struct {
	Name           string
	Attributes     [16]byte
	AggTemporality pmetric.AggregationTemporality
}

// TimeseriesMap maps from a timeseries instance (metric * label values) to the timeseries info for
// the instance.
type TimeseriesMap struct {
	sync.RWMutex
	// The mutex is used to protect access to the member fields. It is acquired for the entirety of
	// AdjustMetricSlice() and also acquired by gc().

	Mark   bool
	TsiMap map[TimeseriesKey]*TimeseriesInfo
}

// Get the TimeseriesInfo for the timeseries associated with the metric and label values.
func (tsm *TimeseriesMap) Get(metric pmetric.Metric, kv pcommon.Map) (*TimeseriesInfo, bool) {
	// This should only be invoked be functions called (directly or indirectly) by AdjustMetricSlice().
	// The lock protecting tsm.tsiMap is acquired there.
	name := metric.Name()
	key := TimeseriesKey{
		Name:       name,
		Attributes: getAttributesSignature(kv),
	}
	switch metric.Type() {
	case pmetric.MetricTypeHistogram:
		// There are 2 types of Histograms whose aggregation temporality needs distinguishing:
		// * CumulativeHistogram
		// * GaugeHistogram
		key.AggTemporality = metric.Histogram().AggregationTemporality()
	case pmetric.MetricTypeExponentialHistogram:
		// There are 2 types of ExponentialHistograms whose aggregation temporality needs distinguishing:
		// * CumulativeHistogram
		// * GaugeHistogram
		key.AggTemporality = metric.ExponentialHistogram().AggregationTemporality()
	}

	tsm.Mark = true
	tsi, ok := tsm.TsiMap[key]
	if !ok {
		tsi = &TimeseriesInfo{}
		tsm.TsiMap[key] = tsi
	}
	tsi.Mark = true
	return tsi, ok
}

// Create a unique string signature for attributes values sorted by attribute keys.
func getAttributesSignature(m pcommon.Map) AttributeHash {
	// TODO(#38621): Investigate whether we should treat empty labels differently.
	clearedMap := pcommon.NewMap()
	m.Range(func(k string, attrValue pcommon.Value) bool {
		value := attrValue.Str()
		if value != "" {
			clearedMap.PutStr(k, value)
		}
		return true
	})
	return pdatautil.MapHash(clearedMap)
}

// Remove timeseries that have aged out.
func (tsm *TimeseriesMap) GC() {
	tsm.Lock()
	defer tsm.Unlock()
	for ts, tsi := range tsm.TsiMap {
		if !tsi.Mark {
			delete(tsm.TsiMap, ts)
		} else {
			tsi.Mark = false
		}
	}
	tsm.Mark = false
}

func newTimeseriesMap() *TimeseriesMap {
	return &TimeseriesMap{Mark: true, TsiMap: map[TimeseriesKey]*TimeseriesInfo{}}
}
