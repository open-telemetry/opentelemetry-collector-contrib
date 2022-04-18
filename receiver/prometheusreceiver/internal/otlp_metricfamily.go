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
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricFamilyPdata struct {
	mtype             pmetric.MetricDataType
	groups            map[string]*metricGroupPdata
	name              string
	mc                MetadataCache
	droppedTimeseries int
	labelKeys         map[string]bool
	labelKeysOrdered  []string
	metadata          *scrape.MetricMetadata
	groupOrders       map[string]int
}

// metricGroupPdata, represents a single metric of a metric family. for example a histogram metric is usually represent by
// a couple data complexValue (buckets and count/sum), a group of a metric family always share a same set of tags. for
// simple types like counter and gauge, each data point is a group of itself
type metricGroupPdata struct {
	family       *metricFamilyPdata
	ts           int64
	ls           labels.Labels
	count        float64
	hasCount     bool
	sum          float64
	hasSum       bool
	value        float64
	complexValue []*dataPoint
}

var pdataStaleFlags = pmetric.NewMetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue)

func newMetricFamilyPdata(metricName string, mc MetadataCache, logger *zap.Logger) *metricFamilyPdata {
	metadata, familyName := metadataForMetric(metricName, mc)
	mtype := convToPdataMetricType(metadata.Type)
	if mtype == pmetric.MetricDataTypeNone {
		logger.Debug(fmt.Sprintf("Unknown-typed metric : %s %+v", metricName, metadata))
	}

	return &metricFamilyPdata{
		mtype:             mtype,
		groups:            make(map[string]*metricGroupPdata),
		name:              familyName,
		mc:                mc,
		droppedTimeseries: 0,
		labelKeys:         make(map[string]bool),
		labelKeysOrdered:  make([]string, 0),
		metadata:          metadata,
		groupOrders:       make(map[string]int),
	}
}

// updateLabelKeys is used to store all the label keys of a same metric family in observed order. since prometheus
// receiver removes any label with empty value before feeding it to an appender, in order to figure out all the labels
// from the same metric family we will need to keep track of what labels have ever been observed.
func (mf *metricFamilyPdata) updateLabelKeys(ls labels.Labels) {
	for _, l := range ls {
		if isUsefulLabelPdata(mf.mtype, l.Name) {
			if _, ok := mf.labelKeys[l.Name]; !ok {
				mf.labelKeys[l.Name] = true
				// use insertion sort to maintain order
				i := sort.SearchStrings(mf.labelKeysOrdered, l.Name)
				mf.labelKeysOrdered = append(mf.labelKeysOrdered, "")
				copy(mf.labelKeysOrdered[i+1:], mf.labelKeysOrdered[i:])
				mf.labelKeysOrdered[i] = l.Name

			}
		}
	}
}

// includesMetric returns true if the metric is part of the family
func (mf *metricFamilyPdata) includesMetric(metricName string) bool {
	if mf.isCumulativeTypePdata() {
		// If it is a merged family type, then it should match the
		// family name when suffixes are trimmed.
		return normalizeMetricName(metricName) == mf.name
	}
	// If it isn't a merged type, the metricName and family name
	// should match
	return metricName == mf.name
}

func (mf *metricFamilyPdata) getGroupKey(ls labels.Labels) string {
	mf.updateLabelKeys(ls)
	return dpgSignature(mf.labelKeysOrdered, ls)
}

func (mg *metricGroupPdata) sortPoints() {
	sort.Slice(mg.complexValue, func(i, j int) bool {
		return mg.complexValue[i].boundary < mg.complexValue[j].boundary
	})
}

func (mg *metricGroupPdata) toDistributionPoint(orderedLabelKeys []string, dest *pmetric.HistogramDataPointSlice) bool {
	if !mg.hasCount || len(mg.complexValue) == 0 {
		return false
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
		point.SetFlags(pdataStaleFlags)
	} else {
		point.SetCount(uint64(mg.count))
		point.SetSum(mg.sum)
	}

	point.SetExplicitBounds(bounds)
	point.SetBucketCounts(bucketCounts)

	// The timestamp MUST be in retrieved from milliseconds and converted to nanoseconds.
	tsNanos := pdataTimestampFromMs(mg.ts)
	if mg.family.isCumulativeTypePdata() {
		point.SetStartTimestamp(tsNanos) // metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
	}
	point.SetTimestamp(tsNanos)
	populateAttributesPdata(orderedLabelKeys, mg.ls, point.Attributes())

	return true
}

func pdataTimestampFromMs(timeAtMs int64) pcommon.Timestamp {
	secs, ns := timeAtMs/1e3, (timeAtMs%1e3)*1e6
	return pcommon.NewTimestampFromTime(time.Unix(secs, ns))
}

func (mg *metricGroupPdata) toSummaryPoint(orderedLabelKeys []string, dest *pmetric.SummaryDataPointSlice) bool {
	// expecting count to be provided, however, in the following two cases, they can be missed.
	// 1. data is corrupted
	// 2. ignored by startValue evaluation
	if !mg.hasCount {
		return false
	}

	mg.sortPoints()

	point := dest.AppendEmpty()
	pointIsStale := value.IsStaleNaN(mg.sum) || value.IsStaleNaN(mg.count)
	if pointIsStale {
		point.SetFlags(pdataStaleFlags)
	} else {
		point.SetSum(mg.sum)
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
	tsNanos := pdataTimestampFromMs(mg.ts)
	point.SetTimestamp(tsNanos)
	if mg.family.isCumulativeTypePdata() {
		point.SetStartTimestamp(tsNanos) // metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
	}
	populateAttributesPdata(orderedLabelKeys, mg.ls, point.Attributes())

	return true
}

func (mg *metricGroupPdata) toNumberDataPoint(orderedLabelKeys []string, dest *pmetric.NumberDataPointSlice) bool {
	var startTsNanos pcommon.Timestamp
	tsNanos := pdataTimestampFromMs(mg.ts)
	// gauge/undefined types have no start time.
	if mg.family.isCumulativeTypePdata() {
		startTsNanos = tsNanos // metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
	}

	point := dest.AppendEmpty()
	point.SetStartTimestamp(startTsNanos)
	point.SetTimestamp(tsNanos)
	if value.IsStaleNaN(mg.value) {
		point.SetFlags(pdataStaleFlags)
	} else {
		point.SetDoubleVal(mg.value)
	}
	populateAttributesPdata(orderedLabelKeys, mg.ls, point.Attributes())

	return true
}

func populateAttributesPdata(orderedKeys []string, ls labels.Labels, dest pcommon.Map) {
	src := ls.Map()
	for _, key := range orderedKeys {
		if src[key] == "" {
			// empty label values should be omitted
			continue
		}
		dest.InsertString(key, src[key])
	}
}

// Purposefully being referenced to avoid lint warnings about being "unused".
var _ = (*metricFamilyPdata)(nil).updateLabelKeys

func (mf *metricFamilyPdata) isCumulativeTypePdata() bool {
	return mf.mtype == pmetric.MetricDataTypeSum ||
		mf.mtype == pmetric.MetricDataTypeHistogram ||
		mf.mtype == pmetric.MetricDataTypeSummary
}

func (mf *metricFamilyPdata) loadMetricGroupOrCreate(groupKey string, ls labels.Labels, ts int64) *metricGroupPdata {
	mg, ok := mf.groups[groupKey]
	if !ok {
		mg = &metricGroupPdata{
			family:       mf,
			ts:           ts,
			ls:           ls,
			complexValue: make([]*dataPoint, 0),
		}
		mf.groups[groupKey] = mg
		// maintaining data insertion order is helpful to generate stable/reproducible metric output
		mf.groupOrders[groupKey] = len(mf.groupOrders)
	}
	return mg
}

func (mf *metricFamilyPdata) Add(metricName string, ls labels.Labels, t int64, v float64) error {
	groupKey := mf.getGroupKey(ls)
	mg := mf.loadMetricGroupOrCreate(groupKey, ls, t)
	switch mf.mtype {
	case pmetric.MetricDataTypeHistogram, pmetric.MetricDataTypeSummary:
		switch {
		case strings.HasSuffix(metricName, metricsSuffixSum):
			// always use the timestamp from sum (count is ok too), because the startTs from quantiles won't be reliable
			// in cases like remote server restart
			mg.ts = t
			mg.sum = v
			mg.hasSum = true
		case strings.HasSuffix(metricName, metricsSuffixCount):
			mg.count = v
			mg.hasCount = true
		default:
			boundary, err := getBoundaryPdata(mf.mtype, ls)
			if err != nil {
				mf.droppedTimeseries++
				return err
			}
			mg.complexValue = append(mg.complexValue, &dataPoint{value: v, boundary: boundary})
		}
	default:
		mg.value = v
	}

	return nil
}

// getGroups to return groups in insertion order
func (mf *metricFamilyPdata) getGroups() []*metricGroupPdata {
	groups := make([]*metricGroupPdata, len(mf.groupOrders))
	for k, v := range mf.groupOrders {
		groups[v] = mf.groups[k]
	}
	return groups
}

func (mf *metricFamilyPdata) ToMetricPdata(metrics *pmetric.MetricSlice) (int, int) {
	metric := pmetric.NewMetric()
	metric.SetDataType(mf.mtype)
	metric.SetName(mf.name)
	metric.SetDescription(mf.metadata.Help)
	metric.SetUnit(mf.metadata.Unit)

	pointCount := 0

	switch mf.mtype {
	case pmetric.MetricDataTypeHistogram:
		histogram := metric.Histogram()
		histogram.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		hdpL := histogram.DataPoints()
		for _, mg := range mf.getGroups() {
			if !mg.toDistributionPoint(mf.labelKeysOrdered, &hdpL) {
				mf.droppedTimeseries++
			}
		}
		pointCount = hdpL.Len()

	case pmetric.MetricDataTypeSummary:
		summary := metric.Summary()
		sdpL := summary.DataPoints()
		for _, mg := range mf.getGroups() {
			if !mg.toSummaryPoint(mf.labelKeysOrdered, &sdpL) {
				mf.droppedTimeseries++
			}
		}
		pointCount = sdpL.Len()

	case pmetric.MetricDataTypeSum:
		sum := metric.Sum()
		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		sum.SetIsMonotonic(true)
		sdpL := sum.DataPoints()
		for _, mg := range mf.getGroups() {
			if !mg.toNumberDataPoint(mf.labelKeysOrdered, &sdpL) {
				mf.droppedTimeseries++
			}
		}
		pointCount = sdpL.Len()

	default: // Everything else should be set to a Gauge.
		metric.SetDataType(pmetric.MetricDataTypeGauge)
		gauge := metric.Gauge()
		gdpL := gauge.DataPoints()
		for _, mg := range mf.getGroups() {
			if !mg.toNumberDataPoint(mf.labelKeysOrdered, &gdpL) {
				mf.droppedTimeseries++
			}
		}
		pointCount = gdpL.Len()
	}

	if pointCount == 0 {
		return mf.droppedTimeseries, mf.droppedTimeseries
	}

	metric.CopyTo(metrics.AppendEmpty())

	// note: the total number of points is the number of points+droppedTimeseries.
	return pointCount + mf.droppedTimeseries, mf.droppedTimeseries
}
