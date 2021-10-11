// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translator

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/DataDog/datadog-agent/pkg/quantile"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/instrumentationlibrary"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/sketches"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/translator/utils"
)

const metricName string = "metric name"

type Translator struct {
	prevPts   *ttlCache
	logger    *zap.Logger
	cfg       translatorConfig
	buildInfo component.BuildInfo
}

func New(params component.ExporterCreateSettings, options ...Option) (*Translator, error) {
	cfg := translatorConfig{
		HistMode:                             HistogramModeDistributions,
		SendCountSum:                         false,
		Quantiles:                            false,
		SendMonotonic:                        true,
		ResourceAttributesAsTags:             false,
		InstrumentationLibraryMetadataAsTags: false,
		sweepInterval:                        1800,
		deltaTTL:                             3600,
		fallbackHostnameProvider:             &noHostProvider{},
	}

	for _, opt := range options {
		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	if cfg.HistMode == HistogramModeNoBuckets && !cfg.SendCountSum {
		return nil, fmt.Errorf("no buckets mode and no send count sum are incompatible")
	}

	cache := newTTLCache(cfg.sweepInterval, cfg.deltaTTL)
	return &Translator{cache, params.Logger, cfg, params.BuildInfo}, nil
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

// isCumulativeMonotonic checks if a metric is a cumulative monotonic metric
func isCumulativeMonotonic(md pdata.Metric) bool {
	switch md.DataType() {
	case pdata.MetricDataTypeSum:
		return md.Sum().AggregationTemporality() == pdata.MetricAggregationTemporalityCumulative &&
			md.Sum().IsMonotonic()
	}
	return false
}

// isSkippable checks if a value can be skipped (because it is not supported by the backend).
// It logs that the value is unsupported for debugging since this sometimes means there is a bug.
func (t *Translator) isSkippable(name string, v float64) bool {
	skippable := math.IsInf(v, 0) || math.IsNaN(v)
	if skippable {
		t.logger.Debug("Unsupported metric value", zap.String(metricName, name), zap.Float64("value", v))
	}
	return skippable
}

// mapNumberMetrics maps double datapoints into Datadog metrics
func (t *Translator) mapNumberMetrics(name string, dt metrics.MetricDataType, slice pdata.NumberDataPointSlice, additionalTags []string) []datadog.Metric {
	ms := make([]datadog.Metric, 0, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		tags := getTags(p.Attributes())
		tags = append(tags, additionalTags...)
		var val float64
		switch p.Type() {
		case pdata.MetricValueTypeDouble:
			val = p.DoubleVal()
		case pdata.MetricValueTypeInt:
			val = float64(p.IntVal())
		}

		if t.isSkippable(name, val) {
			continue
		}

		ms = append(ms,
			metrics.NewMetric(name, dt, uint64(p.Timestamp()), val, tags),
		)
	}
	return ms
}

// mapNumberMonotonicMetrics maps monotonic datapoints into Datadog metrics
func (t *Translator) mapNumberMonotonicMetrics(name string, slice pdata.NumberDataPointSlice, additionalTags []string) []datadog.Metric {
	ms := make([]datadog.Metric, 0, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		ts := uint64(p.Timestamp())
		tags := getTags(p.Attributes())
		tags = append(tags, additionalTags...)

		var val float64
		switch p.Type() {
		case pdata.MetricValueTypeDouble:
			val = p.DoubleVal()
		case pdata.MetricValueTypeInt:
			val = float64(p.IntVal())
		}

		if t.isSkippable(name, val) {
			continue
		}

		if dx, ok := t.prevPts.putAndGetDiff(name, tags, ts, val); ok {
			ms = append(ms, metrics.NewCount(name, ts, dx, tags))
		}
	}
	return ms
}

func getBounds(p pdata.HistogramDataPoint, idx int) (lowerBound float64, upperBound float64) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/v0.10.0/opentelemetry/proto/metrics/v1/metrics.proto#L427-L439
	lowerBound = math.Inf(-1)
	upperBound = math.Inf(1)
	if idx > 0 {
		lowerBound = p.ExplicitBounds()[idx-1]
	}
	if idx < len(p.ExplicitBounds()) {
		upperBound = p.ExplicitBounds()[idx]
	}
	return
}

func (t *Translator) getSketchBuckets(name string, ts uint64, p pdata.HistogramDataPoint, delta bool, tags []string) sketches.SketchSeries {
	as := &quantile.Agent{}
	for j := range p.BucketCounts() {
		lowerBound, upperBound := getBounds(p, j)
		// InsertInterpolate doesn't work with an infinite bound; insert in to the bucket that contains the non-infinite bound
		// https://github.com/DataDog/datadog-agent/blob/7.31.0/pkg/aggregator/check_sampler.go#L107-L111
		if math.IsInf(upperBound, 1) {
			upperBound = lowerBound
		} else if math.IsInf(lowerBound, -1) {
			lowerBound = upperBound
		}

		count := p.BucketCounts()[j]
		if delta {
			as.InsertInterpolate(lowerBound, upperBound, uint(count))
		} else if dx, ok := t.prevPts.putAndGetDiff(name, tags, ts, float64(count)); ok {
			as.InsertInterpolate(lowerBound, upperBound, uint(dx))
		}

	}
	return sketches.SketchSeries{
		Name:     name,
		Tags:     tags,
		Interval: 1,
		Points: []sketches.SketchPoint{{
			Ts:     int64(p.Timestamp() / 1e9),
			Sketch: as.Finish(),
		}},
	}
}

func (t *Translator) getLegacyBuckets(name string, p pdata.HistogramDataPoint, delta bool, tags []string) []datadog.Metric {
	// We have a single metric, 'bucket', which is tagged with the bucket bounds. See:
	// https://github.com/DataDog/integrations-core/blob/7.30.1/datadog_checks_base/datadog_checks/base/checks/openmetrics/v2/transformers/histogram.py
	ms := make([]datadog.Metric, 0, len(p.BucketCounts()))
	fullName := fmt.Sprintf("%s.bucket", name)
	for idx, val := range p.BucketCounts() {
		lowerBound, upperBound := getBounds(p, idx)
		bucketTags := []string{
			fmt.Sprintf("lower_bound:%s", formatFloat(lowerBound)),
			fmt.Sprintf("upper_bound:%s", formatFloat(upperBound)),
		}
		bucketTags = append(bucketTags, tags...)

		count := float64(val)
		ts := uint64(p.Timestamp())
		if delta {
			ms = append(ms, metrics.NewCount(fullName, ts, count, bucketTags))
		} else if dx, ok := t.prevPts.putAndGetDiff(fullName, bucketTags, ts, count); ok {
			ms = append(ms, metrics.NewCount(fullName, ts, dx, bucketTags))
		}
	}
	return ms
}

// mapHistogramMetrics maps double histogram metrics slices to Datadog metrics
//
// A Histogram metric has:
// - The count of values in the population
// - The sum of values in the population
// - A number of buckets, each of them having
//    - the bounds that define the bucket
//    - the count of the number of items in that bucket
//    - a sample value from each bucket
//
// We follow a similar approach to our OpenMetrics check:
// we report sum and count by default; buckets count can also
// be reported (opt-in) tagged by lower bound.
func (t *Translator) mapHistogramMetrics(name string, slice pdata.HistogramDataPointSlice, delta bool, additionalTags []string) (ms []datadog.Metric, sl sketches.SketchSeriesList) {
	// Allocate assuming none are nil and no buckets
	ms = make([]datadog.Metric, 0, 2*slice.Len())
	if t.cfg.HistMode == HistogramModeDistributions {
		sl = make(sketches.SketchSeriesList, 0, slice.Len())
	}
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		ts := uint64(p.Timestamp())
		tags := getTags(p.Attributes())
		tags = append(tags, additionalTags...)

		if t.cfg.SendCountSum {
			count := float64(p.Count())
			countName := fmt.Sprintf("%s.count", name)
			if delta {
				ms = append(ms, metrics.NewCount(countName, ts, count, tags))
			} else if dx, ok := t.prevPts.putAndGetDiff(countName, tags, ts, count); ok {
				ms = append(ms, metrics.NewCount(countName, ts, dx, tags))
			}
		}

		if t.cfg.SendCountSum {
			sum := p.Sum()
			sumName := fmt.Sprintf("%s.sum", name)
			if !t.isSkippable(sumName, p.Sum()) {
				if delta {
					ms = append(ms, metrics.NewCount(sumName, ts, sum, tags))
				} else if dx, ok := t.prevPts.putAndGetDiff(sumName, tags, ts, sum); ok {
					ms = append(ms, metrics.NewCount(sumName, ts, dx, tags))
				}
			}
		}

		switch t.cfg.HistMode {
		case HistogramModeCounters:
			ms = append(ms, t.getLegacyBuckets(name, p, delta, tags)...)
		case HistogramModeDistributions:
			sl = append(sl, t.getSketchBuckets(name, ts, p, true, tags))
		}
	}
	return
}

// formatFloat formats a float number as close as possible to what
// we do on the Datadog Agent Python OpenMetrics check, which, in turn, tries to
// follow https://github.com/OpenObservability/OpenMetrics/blob/v1.0.0/specification/OpenMetrics.md#considerations-canonical-numbers
func formatFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "inf"
	} else if math.IsInf(f, -1) {
		return "-inf"
	} else if math.IsNaN(f) {
		return "nan"
	} else if f == 0 {
		return "0"
	}

	// Add .0 to whole numbers
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if f == math.Floor(f) {
		s = s + ".0"
	}
	return s
}

// getQuantileTag returns the quantile tag for summary types.
func getQuantileTag(quantile float64) string {
	return fmt.Sprintf("quantile:%s", formatFloat(quantile))
}

// mapSummaryMetrics maps summary datapoints into Datadog metrics
func (t *Translator) mapSummaryMetrics(name string, slice pdata.SummaryDataPointSlice, additionalTags []string) []datadog.Metric {
	// Allocate assuming none are nil and no quantiles
	ms := make([]datadog.Metric, 0, 2*slice.Len())
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		ts := uint64(p.Timestamp())
		tags := getTags(p.Attributes())
		tags = append(tags, additionalTags...)

		// count and sum are increasing; we treat them as cumulative monotonic sums.
		{
			countName := fmt.Sprintf("%s.count", name)
			if dx, ok := t.prevPts.putAndGetDiff(countName, tags, ts, float64(p.Count())); ok && !t.isSkippable(countName, dx) {
				ms = append(ms, metrics.NewCount(countName, ts, dx, tags))
			}
		}

		{
			sumName := fmt.Sprintf("%s.sum", name)
			if !t.isSkippable(sumName, p.Sum()) {
				if dx, ok := t.prevPts.putAndGetDiff(sumName, tags, ts, p.Sum()); ok {
					ms = append(ms, metrics.NewCount(sumName, ts, dx, tags))
				}
			}
		}

		if t.cfg.Quantiles {
			fullName := fmt.Sprintf("%s.quantile", name)
			quantiles := p.QuantileValues()
			for i := 0; i < quantiles.Len(); i++ {
				q := quantiles.At(i)

				if t.isSkippable(fullName, q.Value()) {
					continue
				}

				quantileTags := []string{getQuantileTag(q.Quantile())}
				quantileTags = append(quantileTags, tags...)
				ms = append(ms,
					metrics.NewGauge(fullName, ts, q.Value(), quantileTags),
				)
			}
		}
	}
	return ms
}

// MapMetrics maps OTLP metrics into the DataDog format
func (t *Translator) MapMetrics(ctx context.Context, md pdata.Metrics) (series []datadog.Metric, sl sketches.SketchSeriesList, err error) {
	pushTime := uint64(time.Now().UTC().UnixNano())
	rms := md.ResourceMetrics()
	seenHosts := make(map[string]struct{})
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		var attributeTags []string

		// Only fetch attribute tags if they're not already converted into labels.
		// Otherwise some tags would be present twice in a metric's tag list.
		if !t.cfg.ResourceAttributesAsTags {
			attributeTags = attributes.TagsFromAttributes(rm.Resource().Attributes())
		}

		host, ok := attributes.HostnameFromAttributes(rm.Resource().Attributes())
		if !ok {
			host, err = t.cfg.fallbackHostnameProvider.Hostname(context.Background())
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get fallback host: %w", err)
			}
		}
		seenHosts[host] = struct{}{}

		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			metricsArray := ilm.Metrics()

			var additionalTags []string
			if t.cfg.InstrumentationLibraryMetadataAsTags {
				additionalTags = append(attributeTags, instrumentationlibrary.TagsFromInstrumentationLibraryMetadata(ilm.InstrumentationLibrary())...)
			}

			for k := 0; k < metricsArray.Len(); k++ {
				md := metricsArray.At(k)
				var datapoints []datadog.Metric
				var sketchesPoints sketches.SketchSeriesList
				switch md.DataType() {
				case pdata.MetricDataTypeGauge:
					datapoints = t.mapNumberMetrics(md.Name(), metrics.Gauge, md.Gauge().DataPoints(), additionalTags)
				case pdata.MetricDataTypeSum:
					switch md.Sum().AggregationTemporality() {
					case pdata.MetricAggregationTemporalityCumulative:
						if t.cfg.SendMonotonic && isCumulativeMonotonic(md) {
							datapoints = t.mapNumberMonotonicMetrics(md.Name(), md.Sum().DataPoints(), additionalTags)
						} else {
							datapoints = t.mapNumberMetrics(md.Name(), metrics.Gauge, md.Sum().DataPoints(), additionalTags)
						}
					case pdata.MetricAggregationTemporalityDelta:
						datapoints = t.mapNumberMetrics(md.Name(), metrics.Count, md.Sum().DataPoints(), additionalTags)
					default: // pdata.MetricAggregationTemporalityUnspecified or any other not supported type
						t.logger.Debug("Unknown or unsupported aggregation temporality",
							zap.String(metricName, md.Name()),
							zap.Any("aggregation temporality", md.Sum().AggregationTemporality()),
						)
						continue
					}
				case pdata.MetricDataTypeHistogram:
					switch md.Histogram().AggregationTemporality() {
					case pdata.MetricAggregationTemporalityCumulative, pdata.MetricAggregationTemporalityDelta:
						delta := md.Histogram().AggregationTemporality() == pdata.MetricAggregationTemporalityDelta
						datapoints, sketchesPoints = t.mapHistogramMetrics(md.Name(), md.Histogram().DataPoints(), delta, additionalTags)
					default: // pdata.MetricAggregationTemporalityUnspecified or any other not supported type
						t.logger.Debug("Unknown or unsupported aggregation temporality",
							zap.String("metric name", md.Name()),
							zap.Any("aggregation temporality", md.Histogram().AggregationTemporality()),
						)
						continue
					}
				case pdata.MetricDataTypeSummary:
					datapoints = t.mapSummaryMetrics(md.Name(), md.Summary().DataPoints(), additionalTags)
				default: // pdata.MetricDataTypeNone or any other not supported type
					t.logger.Debug("Unknown or unsupported metric type", zap.String(metricName, md.Name()), zap.Any("data type", md.DataType()))
					continue
				}

				for i := range datapoints {
					datapoints[i].SetHost(host)
				}

				for i := range sl {
					sl[i].Host = host
				}

				series = append(series, datapoints...)
				sl = append(sl, sketchesPoints...)
			}
		}
	}

	for host := range seenHosts {
		// Report the host as running
		runningMetric := metrics.DefaultMetrics("metrics", host, pushTime, t.buildInfo)
		series = append(series, runningMetric...)
	}

	return
}
