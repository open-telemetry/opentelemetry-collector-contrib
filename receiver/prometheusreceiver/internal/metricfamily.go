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

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/pkg/textparse"
	"github.com/prometheus/prometheus/scrape"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type metricFamily struct {
	name                string
	mtype               metricspb.MetricDescriptor_Type
	mc                  MetadataCache
	droppedTimeseries   int
	labelKeys           map[string]bool
	labelKeysOrdered    []string
	metadata            *scrape.MetricMetadata
	groupOrders         map[string]int
	groups              map[string]*metricGroup
	intervalStartTimeMs int64
}

func metadataForMetric(metricName string, mc MetadataCache) (*scrape.MetricMetadata, string) {
	if metadata, ok := internalMetricMetadata[metricName]; ok {
		return metadata, metricName
	}
	if metadata, ok := mc.Metadata(metricName); ok {
		return &metadata, metricName
	}
	// If we didn't find metadata with the original name,
	// try with suffixes trimmed, in-case it is a "merged" metric type.
	normalizedName := normalizeMetricName(metricName)
	if metadata, ok := mc.Metadata(normalizedName); ok {
		if metadata.Type == textparse.MetricTypeCounter {
			return &metadata, metricName
		}
		return &metadata, normalizedName
	}
	// Otherwise, the metric is unknown
	return &scrape.MetricMetadata{
		Metric: metricName,
		Type:   textparse.MetricTypeUnknown,
	}, metricName
}

func newMetricFamily(metricName string, mc MetadataCache, logger *zap.Logger, intervalStartTimeMs int64) *metricFamily {
	metadata, familyName := metadataForMetric(metricName, mc)
	ocaMetricType := convToOCAMetricType(metadata.Type)

	if ocaMetricType == metricspb.MetricDescriptor_UNSPECIFIED {
		logger.Debug(fmt.Sprintf("Unknown-typed metric : %s %+v", metricName, metadata))
	}

	return &metricFamily{
		name:                familyName,
		mtype:               ocaMetricType,
		mc:                  mc,
		droppedTimeseries:   0,
		labelKeys:           make(map[string]bool),
		labelKeysOrdered:    make([]string, 0),
		metadata:            metadata,
		groupOrders:         make(map[string]int),
		groups:              make(map[string]*metricGroup),
		intervalStartTimeMs: intervalStartTimeMs,
	}
}

// internalMetricMetadata allows looking up metadata for internal scrape metrics
var internalMetricMetadata = map[string]*scrape.MetricMetadata{
	scrapeUpMetricName: {
		Metric: scrapeUpMetricName,
		Type:   textparse.MetricTypeGauge,
		Help:   "The scraping was successful",
	},
	"scrape_duration_seconds": {
		Metric: "scrape_duration_seconds",
		Unit:   "seconds",
		Type:   textparse.MetricTypeGauge,
		Help:   "Duration of the scrape",
	},
	"scrape_samples_scraped": {
		Metric: "scrape_samples_scraped",
		Type:   textparse.MetricTypeGauge,
		Help:   "The number of samples the target exposed",
	},
	"scrape_series_added": {
		Metric: "scrape_series_added",
		Type:   textparse.MetricTypeGauge,
		Help:   "The approximate number of new series in this scrape",
	},
	"scrape_samples_post_metric_relabeling": {
		Metric: "scrape_samples_post_metric_relabeling",
		Type:   textparse.MetricTypeGauge,
		Help:   "The number of samples remaining after metric relabeling was applied",
	},
}

// updateLabelKeys is used to store all the label keys of a same metric family in observed order. since prometheus
// receiver removes any label with empty value before feeding it to an appender, in order to figure out all the labels
// from the same metric family we will need to keep track of what labels have ever been observed.
func (mf *metricFamily) updateLabelKeys(ls labels.Labels) {
	for _, l := range ls {
		if isUsefulLabel(mf.mtype, l.Name) {
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
func (mf *metricFamily) includesMetric(metricName string) bool {
	if mf.isCumulativeType() || mf.mtype == metricspb.MetricDescriptor_GAUGE_DISTRIBUTION {
		// If it is a type that can have suffixes removed, then the metric should match the
		// family name when suffixes are trimmed.
		return normalizeMetricName(metricName) == mf.name
	}
	// If it isn't a merged type, the metricName and family name
	// should match
	return metricName == mf.name
}

func (mf *metricFamily) isCumulativeType() bool {
	return mf.mtype == metricspb.MetricDescriptor_CUMULATIVE_DOUBLE ||
		mf.mtype == metricspb.MetricDescriptor_CUMULATIVE_INT64 ||
		mf.mtype == metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION ||
		mf.mtype == metricspb.MetricDescriptor_SUMMARY
}

func (mf *metricFamily) getGroupKey(ls labels.Labels) string {
	mf.updateLabelKeys(ls)
	return dpgSignature(mf.labelKeysOrdered, ls)
}

// getGroups to return groups in insertion order
func (mf *metricFamily) getGroups() []*metricGroup {
	groups := make([]*metricGroup, len(mf.groupOrders))
	for k, v := range mf.groupOrders {
		groups[v] = mf.groups[k]
	}

	return groups
}

func (mf *metricFamily) loadMetricGroupOrCreate(groupKey string, ls labels.Labels, ts int64) *metricGroup {
	mg, ok := mf.groups[groupKey]
	if !ok {
		mg = &metricGroup{
			family:              mf,
			ts:                  ts,
			ls:                  ls,
			complexValue:        make([]*dataPoint, 0),
			intervalStartTimeMs: mf.intervalStartTimeMs,
		}
		mf.groups[groupKey] = mg
		// maintaining data insertion order is helpful to generate stable/reproducible metric output
		mf.groupOrders[groupKey] = len(mf.groupOrders)
	}
	return mg
}

func (mf *metricFamily) getLabelKeys() []*metricspb.LabelKey {
	lks := make([]*metricspb.LabelKey, len(mf.labelKeysOrdered))
	for i, k := range mf.labelKeysOrdered {
		lks[i] = &metricspb.LabelKey{Key: k}
	}
	return lks
}

func (mf *metricFamily) Add(metricName string, ls labels.Labels, t int64, v float64) error {
	groupKey := mf.getGroupKey(ls)
	mg := mf.loadMetricGroupOrCreate(groupKey, ls, t)
	switch mf.mtype {
	case metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION:
		fallthrough
	case metricspb.MetricDescriptor_SUMMARY:
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
			boundary, err := getBoundary(mf.mtype, ls)
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

func (mf *metricFamily) ToMetric() (*metricspb.Metric, int, int) {
	timeseries := make([]*metricspb.TimeSeries, 0, len(mf.groups))
	switch mf.mtype {
	// not supported currently
	// case metricspb.MetricDescriptor_GAUGE_DISTRIBUTION:
	//	return nil
	case metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION:
		for _, mg := range mf.getGroups() {
			tss := mg.toDistributionTimeSeries(mf.labelKeysOrdered)
			if tss != nil {
				timeseries = append(timeseries, tss)
			} else {
				mf.droppedTimeseries++
			}
		}
	case metricspb.MetricDescriptor_SUMMARY:
		for _, mg := range mf.getGroups() {
			tss := mg.toSummaryTimeSeries(mf.labelKeysOrdered)
			if tss != nil {
				timeseries = append(timeseries, tss)
			} else {
				mf.droppedTimeseries++
			}
		}
	default:
		for _, mg := range mf.getGroups() {
			tss := mg.toDoubleValueTimeSeries(mf.labelKeysOrdered)
			if tss != nil {
				timeseries = append(timeseries, tss)
			} else {
				mf.droppedTimeseries++
			}
		}
	}

	// note: the total number of timeseries is the length of timeseries plus the number of dropped timeseries.
	numTimeseries := len(timeseries)
	if numTimeseries != 0 {
		return &metricspb.Metric{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        mf.name,
					Description: mf.metadata.Help,
					Unit:        heuristicalMetricAndKnownUnits(mf.name, mf.metadata.Unit),
					Type:        mf.mtype,
					LabelKeys:   mf.getLabelKeys(),
				},
				Timeseries: timeseries,
			},
			numTimeseries + mf.droppedTimeseries,
			mf.droppedTimeseries
	}
	return nil, mf.droppedTimeseries, mf.droppedTimeseries
}

type dataPoint struct {
	value    float64
	boundary float64
}

// metricGroup, represents a single metric of a metric family. for example a histogram metric is usually represent by
// a couple data complexValue (buckets and count/sum), a group of a metric family always share a same set of tags. for
// simple types like counter and gauge, each data point is a group of itself
type metricGroup struct {
	family              *metricFamily
	ts                  int64
	ls                  labels.Labels
	count               float64
	hasCount            bool
	sum                 float64
	hasSum              bool
	value               float64
	complexValue        []*dataPoint
	intervalStartTimeMs int64
}

func (mg *metricGroup) sortPoints() {
	sort.Slice(mg.complexValue, func(i, j int) bool {
		return mg.complexValue[i].boundary < mg.complexValue[j].boundary
	})
}

func (mg *metricGroup) toDistributionTimeSeries(orderedLabelKeys []string) *metricspb.TimeSeries {
	if !(mg.hasCount) || len(mg.complexValue) == 0 {
		return nil
	}
	mg.sortPoints()
	// for OCAgent Proto, the bounds won't include +inf
	bounds := make([]float64, len(mg.complexValue)-1)
	buckets := make([]*metricspb.DistributionValue_Bucket, len(mg.complexValue))

	for i := 0; i < len(mg.complexValue); i++ {
		if i != len(mg.complexValue)-1 {
			// not need to add +inf as bound to oc proto
			bounds[i] = mg.complexValue[i].boundary
		}
		adjustedCount := mg.complexValue[i].value
		if i != 0 {
			adjustedCount -= mg.complexValue[i-1].value
		}
		buckets[i] = &metricspb.DistributionValue_Bucket{Count: int64(adjustedCount)}
	}

	dv := &metricspb.DistributionValue{
		BucketOptions: &metricspb.DistributionValue_BucketOptions{
			Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
				Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
					Bounds: bounds,
				},
			},
		},
		Count:   int64(mg.count),
		Sum:     mg.sum,
		Buckets: buckets,
		// SumOfSquaredDeviation:  // there's no way to compute this value from prometheus data
	}

	return &metricspb.TimeSeries{
		StartTimestamp: timestampFromMs(mg.ts),
		LabelValues:    populateLabelValues(orderedLabelKeys, mg.ls),
		Points: []*metricspb.Point{
			{
				Timestamp: timestampFromMs(mg.ts),
				Value:     &metricspb.Point_DistributionValue{DistributionValue: dv},
			},
		},
	}
}

func (mg *metricGroup) toSummaryTimeSeries(orderedLabelKeys []string) *metricspb.TimeSeries {
	// expecting count to be provided, however, in the following two cases, they can be missed.
	// 1. data is corrupted
	// 2. ignored by startValue evaluation
	if !(mg.hasCount) {
		return nil
	}
	mg.sortPoints()
	percentiles := make([]*metricspb.SummaryValue_Snapshot_ValueAtPercentile, len(mg.complexValue))
	for i, p := range mg.complexValue {
		percentiles[i] =
			&metricspb.SummaryValue_Snapshot_ValueAtPercentile{Percentile: p.boundary * 100, Value: p.value}
	}

	// allow percentiles to be nil when no data provided from prometheus
	var snapshot *metricspb.SummaryValue_Snapshot
	if len(percentiles) != 0 {
		snapshot = &metricspb.SummaryValue_Snapshot{
			PercentileValues: percentiles,
		}
	}

	// Based on the summary description from https://prometheus.io/docs/concepts/metric_types/#summary
	// the quantiles are calculated over a sliding time window, however, the count is the total count of
	// observations and the corresponding sum is a sum of all observed values, thus the sum and count used
	// at the global level of the metricspb.SummaryValue

	summaryValue := &metricspb.SummaryValue{
		Sum:      &wrapperspb.DoubleValue{Value: mg.sum},
		Count:    &wrapperspb.Int64Value{Value: int64(mg.count)},
		Snapshot: snapshot,
	}
	return &metricspb.TimeSeries{
		StartTimestamp: timestampFromMs(mg.ts),
		LabelValues:    populateLabelValues(orderedLabelKeys, mg.ls),
		Points: []*metricspb.Point{
			{Timestamp: timestampFromMs(mg.ts), Value: &metricspb.Point_SummaryValue{SummaryValue: summaryValue}},
		},
	}
}

func (mg *metricGroup) toDoubleValueTimeSeries(orderedLabelKeys []string) *metricspb.TimeSeries {
	var startTs *timestamppb.Timestamp
	// gauge/undefined types has no start time
	if mg.family.isCumulativeType() {
		startTs = timestampFromMs(mg.ts) // metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
	}

	return &metricspb.TimeSeries{
		StartTimestamp: startTs,
		Points:         []*metricspb.Point{{Timestamp: timestampFromMs(mg.ts), Value: &metricspb.Point_DoubleValue{DoubleValue: mg.value}}},
		LabelValues:    populateLabelValues(orderedLabelKeys, mg.ls),
	}
}

func populateLabelValues(orderedKeys []string, ls labels.Labels) []*metricspb.LabelValue {
	lvs := make([]*metricspb.LabelValue, len(orderedKeys))
	lmap := ls.Map()
	for i, k := range orderedKeys {
		value := lmap[k]
		lvs[i] = &metricspb.LabelValue{Value: value, HasValue: value != ""}
	}
	return lvs
}
