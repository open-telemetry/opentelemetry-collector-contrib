// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

const (
	traceIDKey = "trace_id"
	spanIDKey  = "span_id"
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
	mtype        pmetric.MetricType
	ts           int64
	ls           labels.Labels
	count        float64
	hasCount     bool
	sum          float64
	hasSum       bool
	created      float64
	value        float64
	complexValue []*dataPoint
	exemplars    pmetric.ExemplarSlice
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

	// for OTLP the bounds won't include +inf
	bounds := make([]float64, len(mg.complexValue)-1)
	bucketCounts := make([]uint64, len(mg.complexValue))

	pointIsStale := value.IsStaleNaN(mg.sum) || value.IsStaleNaN(mg.count)

	for i := 0; i < len(mg.complexValue); i++ {
		if i != len(mg.complexValue)-1 {
			// not need to add +inf as OTLP assumes it
			bounds[i] = mg.complexValue[i].boundary
		} else if mg.complexValue[i].boundary != math.Inf(1) {
			// This histogram is missing the +Inf bucket, and isn't a complete prometheus histogram.
			return
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
	if mg.created != 0 {
		point.SetStartTimestamp(timestampFromFloat64(mg.created))
	} else {
		// metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
		point.SetStartTimestamp(tsNanos)
	}
	point.SetTimestamp(tsNanos)
	populateAttributes(pmetric.MetricTypeHistogram, mg.ls, point.Attributes())
	mg.setExemplars(point.Exemplars())
}

func (mg *metricGroup) setExemplars(exemplars pmetric.ExemplarSlice) {
	if mg == nil {
		return
	}
	if mg.exemplars.Len() > 0 {
		mg.exemplars.MoveAndAppendTo(exemplars)
	}
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
	if mg.created != 0 {
		point.SetStartTimestamp(timestampFromFloat64(mg.created))
	} else {
		// metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
		point.SetStartTimestamp(tsNanos)
	}
	populateAttributes(pmetric.MetricTypeSummary, mg.ls, point.Attributes())
}

func (mg *metricGroup) toNumberDataPoint(dest pmetric.NumberDataPointSlice) {
	tsNanos := timestampFromMs(mg.ts)
	point := dest.AppendEmpty()
	// gauge/undefined types have no start time.
	if mg.mtype == pmetric.MetricTypeSum {
		if mg.created != 0 {
			point.SetStartTimestamp(timestampFromFloat64(mg.created))
		} else {
			// metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
			point.SetStartTimestamp(tsNanos)
		}
	}
	point.SetTimestamp(tsNanos)
	if value.IsStaleNaN(mg.value) {
		point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	} else {
		point.SetDoubleValue(mg.value)
	}
	populateAttributes(pmetric.MetricTypeGauge, mg.ls, point.Attributes())
	mg.setExemplars(point.Exemplars())
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
			mtype:     mf.mtype,
			ts:        ts,
			ls:        ls,
			exemplars: pmetric.NewExemplarSlice(),
		}
		mf.groups[groupKey] = mg
		// maintaining data insertion order is helpful to generate stable/reproducible metric output
		mf.groupOrders = append(mf.groupOrders, mg)
	}
	return mg
}

func (mf *metricFamily) addSeries(seriesRef uint64, metricName string, ls labels.Labels, t int64, v float64) error {
	mg := mf.loadMetricGroupOrCreate(seriesRef, ls, t)
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
		case strings.HasSuffix(metricName, metricSuffixCreated):
			mg.created = v
		default:
			boundary, err := getBoundary(mf.mtype, ls)
			if err != nil {
				return err
			}
			mg.complexValue = append(mg.complexValue, &dataPoint{value: v, boundary: boundary})
		}
	case pmetric.MetricTypeSum:
		if strings.HasSuffix(metricName, metricSuffixCreated) {
			mg.created = v
		} else {
			mg.value = v
		}
	case pmetric.MetricTypeEmpty, pmetric.MetricTypeGauge, pmetric.MetricTypeExponentialHistogram:
		fallthrough
	default:
		mg.value = v
	}

	return nil
}

func (mf *metricFamily) appendMetric(metrics pmetric.MetricSlice, normalizer *prometheus.Normalizer) {
	metric := pmetric.NewMetric()
	// Trims type's and unit's suffixes from metric name
	metric.SetName(normalizer.TrimPromSuffixes(mf.name, mf.mtype, mf.metadata.Unit))
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

	case pmetric.MetricTypeEmpty, pmetric.MetricTypeGauge, pmetric.MetricTypeExponentialHistogram:
		fallthrough
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

func (mf *metricFamily) addExemplar(seriesRef uint64, e exemplar.Exemplar) {
	mg := mf.groups[seriesRef]
	if mg == nil {
		return
	}
	es := mg.exemplars
	convertExemplar(e, es.AppendEmpty())
}

func convertExemplar(pe exemplar.Exemplar, e pmetric.Exemplar) {
	e.SetTimestamp(timestampFromMs(pe.Ts))
	e.SetDoubleValue(pe.Value)
	e.FilteredAttributes().EnsureCapacity(len(pe.Labels))
	for _, lb := range pe.Labels {
		switch strings.ToLower(lb.Name) {
		case traceIDKey:
			var tid [16]byte
			err := decodeAndCopyToLowerBytes(tid[:], []byte(lb.Value))
			if err == nil {
				e.SetTraceID(tid)
			} else {
				e.FilteredAttributes().PutStr(lb.Name, lb.Value)
			}
		case spanIDKey:
			var sid [8]byte
			err := decodeAndCopyToLowerBytes(sid[:], []byte(lb.Value))
			if err == nil {
				e.SetSpanID(sid)
			} else {
				e.FilteredAttributes().PutStr(lb.Name, lb.Value)
			}
		default:
			e.FilteredAttributes().PutStr(lb.Name, lb.Value)
		}
	}
}

/*
	decodeAndCopyToLowerBytes copies src to dst on lower bytes instead of higher

1. If len(src) > len(dst) -> copy first len(dst) bytes as it is. Example -> src = []byte{0xab,0xcd,0xef,0xgh,0xij}, dst = [2]byte, result dst = [2]byte{0xab, 0xcd}
2. If len(src) = len(dst) -> copy src to dst as it is
3. If len(src) < len(dst) -> prepend required 0s and then add src to dst. Example -> src = []byte{0xab, 0xcd}, dst = [8]byte, result dst = [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xab, 0xcd}
*/
func decodeAndCopyToLowerBytes(dst []byte, src []byte) error {
	var err error
	decodedLen := hex.DecodedLen(len(src))
	if decodedLen >= len(dst) {
		_, err = hex.Decode(dst, src[:hex.EncodedLen(len(dst))])
	} else {
		_, err = hex.Decode(dst[len(dst)-decodedLen:], src)
	}
	return err
}
