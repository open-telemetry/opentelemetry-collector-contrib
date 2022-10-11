// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"fmt"
	"sort"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricFamily struct {
	mtype pmetric.MetricType
	// isMonotonic only applies to sums
	isMonotonic bool
	groups      map[uint64]*metricGroup
	name        string
	metadata    *scrape.MetricMetadata
	groupOrders []*metricGroup
}

// metricGroup, represents a single metric of a metric family. for example a histogram metric is usually represent by
// a couple data complexValue (buckets and count/sum), a group of a metric family always share a same set of tags. for
// simple types like counter and gauge, each data point is a group of itself
type metricGroup struct {
	family       *metricFamily
	ts           int64
	ls           labels.Labels
	count        float64
	hasCount     bool
	sum          float64
	hasSum       bool
	value        float64
	complexValue []*dataPoint
}

func newMetricFamily(metricName string, mc scrape.MetricMetadataStore, logger *zap.Logger) *metricFamily {
	metadata, familyName := metadataForMetric(metricName, mc)
	mtype, isMonotonic := convToMetricType(metadata.Type)
	if mtype == pmetric.MetricTypeEmpty {
		logger.Debug(fmt.Sprintf("Unknown-typed metric : %s %+v", metricName, metadata))
	}

	return &metricFamily{
		mtype:       mtype,
		isMonotonic: isMonotonic,
		groups:      make(map[uint64]*metricGroup),
		name:        familyName,
		metadata:    metadata,
	}
}

// includesMetric returns true if the metric is part of the family
func (mf *metricFamily) includesMetric(metricName string) bool {
	if mf.mtype != pmetric.MetricTypeGauge {
		// If it is a merged family type, then it should match the
		// family name when suffixes are trimmed.
		return normalizeMetricName(metricName) == mf.name
	}
	// If it isn't a merged type, the metricName and family name should match
	return metricName == mf.name
}

func (mf *metricFamily) getGroupKey(ls labels.Labels) uint64 {
	bytes := make([]byte, 0, 2048)
	hash, _ := ls.HashWithoutLabels(bytes, getSortedNotUsefulLabels(mf.mtype)...)
	return hash
}

func (mg *metricGroup) sortPoints() {
	sort.Slice(mg.complexValue, func(i, j int) bool {
		return mg.complexValue[i].boundary < mg.complexValue[j].boundary
	})
}

func (mg *metricGroup) toDistributionPoint(dest pmetric.HistogramDataPointSlice) {
	if !mg.hasCount || len(mg.complexValue) == 0 {
		return
	}

	mg.sortPoints()

	// for OCAgent Proto, the bounds won't include +inf
	// TODO: (@odeke-em) should we also check OpenTelemetry Pdata for bucket bounds?
	bounds := make([]float64, len(mg.complexValue)-1)
	bucketCounts := make([]uint64, len(mg.complexValue))

	pointIsStale := value.IsStaleNaN(mg.sum) || value.IsStaleNaN(mg.count)

	for i := 0; i < len(mg.complexValue); i++ {
		if i != len(mg.complexValue)-1 {
			// not need to add +inf as bound to oc proto
			bounds[i] = mg.complexValue[i].boundary
		}
		adjustedCount := mg.complexValue[i].value
		// Buckets still need to be sent to know to set them as stale,
		// but a staleness NaN converted to uint64 would be an extremely large number.
		// Setting to 0 instead.
		if pointIsStale {
			adjustedCount = 0
		} else if i != 0 {
			adjustedCount -= mg.complexValue[i-1].value
		}
		bucketCounts[i] = uint64(adjustedCount)
	}

	point := dest.AppendEmpty()

	if pointIsStale {
		point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	} else {
		point.SetCount(uint64(mg.count))
		if mg.hasSum {
			point.SetSum(mg.sum)
		}
	}

	point.ExplicitBounds().FromRaw(bounds)
	point.BucketCounts().FromRaw(bucketCounts)

	// The timestamp MUST be in retrieved from milliseconds and converted to nanoseconds.
	tsNanos := timestampFromMs(mg.ts)
	point.SetStartTimestamp(tsNanos) // metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
	point.SetTimestamp(tsNanos)
	populateAttributes(pmetric.MetricTypeHistogram, mg.ls, point.Attributes())
}

func (mg *metricGroup) toSummaryPoint(dest pmetric.SummaryDataPointSlice) {
	// expecting count to be provided, however, in the following two cases, they can be missed.
	// 1. data is corrupted
	// 2. ignored by startValue evaluation
	if !mg.hasCount {
		return
	}

	mg.sortPoints()

	point := dest.AppendEmpty()
	pointIsStale := value.IsStaleNaN(mg.sum) || value.IsStaleNaN(mg.count)
	if pointIsStale {
		point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	} else {
		if mg.hasSum {
			point.SetSum(mg.sum)
		}
		point.SetCount(uint64(mg.count))
	}

	quantileValues := point.QuantileValues()
	for _, p := range mg.complexValue {
		quantile := quantileValues.AppendEmpty()
		// Quantiles still need to be sent to know to set them as stale,
		// but a staleness NaN converted to uint64 would be an extremely large number.
		// By not setting the quantile value, it will default to 0.
		if !pointIsStale {
			quantile.SetValue(p.value)
		}
		quantile.SetQuantile(p.boundary)
	}

	// Based on the summary description from https://prometheus.io/docs/concepts/metric_types/#summary
	// the quantiles are calculated over a sliding time window, however, the count is the total count of
	// observations and the corresponding sum is a sum of all observed values, thus the sum and count used
	// at the global level of the metricspb.SummaryValue
	// The timestamp MUST be in retrieved from milliseconds and converted to nanoseconds.
	tsNanos := timestampFromMs(mg.ts)
	point.SetTimestamp(tsNanos)
	point.SetStartTimestamp(tsNanos) // metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
	populateAttributes(pmetric.MetricTypeSummary, mg.ls, point.Attributes())
}

func (mg *metricGroup) toNumberDataPoint(dest pmetric.NumberDataPointSlice) {
	tsNanos := timestampFromMs(mg.ts)
	point := dest.AppendEmpty()
	// gauge/undefined types have no start time.
	if mg.family.mtype == pmetric.MetricTypeSum {
		point.SetStartTimestamp(tsNanos) // metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
	}
	point.SetTimestamp(tsNanos)
	if value.IsStaleNaN(mg.value) {
		point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	} else {
		point.SetDoubleValue(mg.value)
	}
	populateAttributes(pmetric.MetricTypeGauge, mg.ls, point.Attributes())
}

func populateAttributes(mType pmetric.MetricType, ls labels.Labels, dest pcommon.Map) {
	dest.EnsureCapacity(ls.Len())
	names := getSortedNotUsefulLabels(mType)
	j := 0
	for i := range ls {
		for j < len(names) && names[j] < ls[i].Name {
			j++
		}
		if j < len(names) && ls[i].Name == names[j] {
			continue
		}
		if ls[i].Value == "" {
			// empty label values should be omitted
			continue
		}
		dest.PutStr(ls[i].Name, ls[i].Value)
	}
}

func (mf *metricFamily) loadMetricGroupOrCreate(groupKey uint64, ls labels.Labels, ts int64) *metricGroup {
	mg, ok := mf.groups[groupKey]
	if !ok {
		mg = &metricGroup{
			family: mf,
			ts:     ts,
			ls:     ls,
		}
		mf.groups[groupKey] = mg
		// maintaining data insertion order is helpful to generate stable/reproducible metric output
		mf.groupOrders = append(mf.groupOrders, mg)
	}
	return mg
}

func (mf *metricFamily) Add(metricName string, ls labels.Labels, t int64, v float64) error {
	groupKey := mf.getGroupKey(ls)
	mg := mf.loadMetricGroupOrCreate(groupKey, ls, t)
	if mg.ts != t {
		return fmt.Errorf("inconsistent timestamps on metric points for metric %v", metricName)
	}
	switch mf.mtype {
	case pmetric.MetricTypeHistogram, pmetric.MetricTypeSummary:
		switch {
		case strings.HasSuffix(metricName, metricsSuffixSum):
			mg.sum = v
			mg.hasSum = true
		case strings.HasSuffix(metricName, metricsSuffixCount):
			// always use the timestamp from count, because is the only required field for histograms and summaries.
			mg.ts = t
			mg.count = v
			mg.hasCount = true
		default:
			boundary, err := getBoundary(mf.mtype, ls)
			if err != nil {
				return err
			}
			mg.complexValue = append(mg.complexValue, &dataPoint{value: v, boundary: boundary})
		}
	default:
		mg.value = v
	}

	return nil
}

func (mf *metricFamily) appendMetric(metrics pmetric.MetricSlice) {
	metric := pmetric.NewMetric()
	metric.SetName(mf.name)
	metric.SetDescription(mf.metadata.Help)
	metric.SetUnit(mf.metadata.Unit)

	pointCount := 0

	switch mf.mtype {
	case pmetric.MetricTypeHistogram:
		histogram := metric.SetEmptyHistogram()
		histogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		hdpL := histogram.DataPoints()
		for _, mg := range mf.groupOrders {
			mg.toDistributionPoint(hdpL)
		}
		pointCount = hdpL.Len()

	case pmetric.MetricTypeSummary:
		summary := metric.SetEmptySummary()
		sdpL := summary.DataPoints()
		for _, mg := range mf.groupOrders {
			mg.toSummaryPoint(sdpL)
		}
		pointCount = sdpL.Len()

	case pmetric.MetricTypeSum:
		sum := metric.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum.SetIsMonotonic(mf.isMonotonic)
		sdpL := sum.DataPoints()
		for _, mg := range mf.groupOrders {
			mg.toNumberDataPoint(sdpL)
		}
		pointCount = sdpL.Len()

	default: // Everything else should be set to a Gauge.
		gauge := metric.SetEmptyGauge()
		gdpL := gauge.DataPoints()
		for _, mg := range mf.groupOrders {
			mg.toNumberDataPoint(gdpL)
		}
		pointCount = gdpL.Len()
	}

	if pointCount == 0 {
		return
	}

	metric.MoveTo(metrics.AppendEmpty())
}
