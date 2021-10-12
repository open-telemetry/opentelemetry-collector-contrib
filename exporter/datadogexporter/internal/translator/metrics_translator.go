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

	"github.com/DataDog/datadog-agent/pkg/quantile"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/instrumentationlibrary"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/translator/utils"
)

const metricName string = "metric name"

// Translator is a metrics translator.
type Translator struct {
	prevPts *ttlCache
	logger  *zap.Logger
	cfg     translatorConfig
}

// New creates a new translator with given options.
func New(logger *zap.Logger, options ...Option) (*Translator, error) {
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
	return &Translator{cache, logger, cfg}, nil
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
func (t *Translator) mapNumberMetrics(
	ctx context.Context,
	consumer TimeSeriesConsumer,
	name string,
	dt MetricDataType,
	slice pdata.NumberDataPointSlice,
	additionalTags []string,
	host string,
) {

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

		consumer.ConsumeTimeSeries(ctx, name, dt, uint64(p.Timestamp()), val, tags, host)
	}
}

// mapNumberMonotonicMetrics maps monotonic datapoints into Datadog metrics
func (t *Translator) mapNumberMonotonicMetrics(
	ctx context.Context,
	consumer TimeSeriesConsumer,
	name string,
	slice pdata.NumberDataPointSlice,
	additionalTags []string,
	host string,
) {
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
			consumer.ConsumeTimeSeries(ctx, name, Count, ts, dx, tags, host)
		}
	}
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

func (t *Translator) getSketchBuckets(
	ctx context.Context,
	consumer SketchConsumer,
	name string,
	ts uint64,
	p pdata.HistogramDataPoint,
	delta bool,
	tags []string,
	host string,
) {
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

	consumer.ConsumeSketch(ctx, name, ts, as.Finish(), tags, host)
}

func (t *Translator) getLegacyBuckets(
	ctx context.Context,
	consumer TimeSeriesConsumer,
	name string,
	p pdata.HistogramDataPoint,
	delta bool,
	tags []string,
	host string,
) {
	// We have a single metric, 'bucket', which is tagged with the bucket bounds. See:
	// https://github.com/DataDog/integrations-core/blob/7.30.1/datadog_checks_base/datadog_checks/base/checks/openmetrics/v2/transformers/histogram.py
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
			consumer.ConsumeTimeSeries(ctx, fullName, Count, ts, count, bucketTags, host)
		} else if dx, ok := t.prevPts.putAndGetDiff(fullName, bucketTags, ts, count); ok {
			consumer.ConsumeTimeSeries(ctx, fullName, Count, ts, dx, bucketTags, host)
		}
	}
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
func (t *Translator) mapHistogramMetrics(
	ctx context.Context,
	consumer Consumer,
	name string,
	slice pdata.HistogramDataPointSlice,
	delta bool,
	additionalTags []string,
	host string,
) {
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		ts := uint64(p.Timestamp())
		tags := getTags(p.Attributes())
		tags = append(tags, additionalTags...)

		if t.cfg.SendCountSum {
			count := float64(p.Count())
			countName := fmt.Sprintf("%s.count", name)
			if delta {
				consumer.ConsumeTimeSeries(ctx, countName, Count, ts, count, tags, host)
			} else if dx, ok := t.prevPts.putAndGetDiff(countName, tags, ts, count); ok {
				consumer.ConsumeTimeSeries(ctx, countName, Count, ts, dx, tags, host)
			}
		}

		if t.cfg.SendCountSum {
			sum := p.Sum()
			sumName := fmt.Sprintf("%s.sum", name)
			if !t.isSkippable(sumName, p.Sum()) {
				if delta {
					consumer.ConsumeTimeSeries(ctx, sumName, Count, ts, sum, tags, host)
				} else if dx, ok := t.prevPts.putAndGetDiff(sumName, tags, ts, sum); ok {
					consumer.ConsumeTimeSeries(ctx, sumName, Count, ts, dx, tags, host)
				}
			}
		}

		switch t.cfg.HistMode {
		case HistogramModeCounters:
			t.getLegacyBuckets(ctx, consumer, name, p, delta, tags, host)
		case HistogramModeDistributions:
			t.getSketchBuckets(ctx, consumer, name, ts, p, true, tags, host)
		}
	}
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
func (t *Translator) mapSummaryMetrics(
	ctx context.Context,
	consumer TimeSeriesConsumer,
	name string,
	slice pdata.SummaryDataPointSlice,
	additionalTags []string,
	host string,
) {

	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		ts := uint64(p.Timestamp())
		tags := getTags(p.Attributes())
		tags = append(tags, additionalTags...)

		// count and sum are increasing; we treat them as cumulative monotonic sums.
		{
			countName := fmt.Sprintf("%s.count", name)
			if dx, ok := t.prevPts.putAndGetDiff(countName, tags, ts, float64(p.Count())); ok && !t.isSkippable(countName, dx) {
				consumer.ConsumeTimeSeries(ctx, countName, Count, ts, dx, tags, host)
			}
		}

		{
			sumName := fmt.Sprintf("%s.sum", name)
			if !t.isSkippable(sumName, p.Sum()) {
				if dx, ok := t.prevPts.putAndGetDiff(sumName, tags, ts, p.Sum()); ok {
					consumer.ConsumeTimeSeries(ctx, sumName, Count, ts, dx, tags, host)
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
				consumer.ConsumeTimeSeries(ctx, fullName, Gauge, ts, q.Value(), quantileTags, host)
			}
		}
	}
}

// MapMetrics maps OTLP metrics into the DataDog format
func (t *Translator) MapMetrics(ctx context.Context, md pdata.Metrics, consumer Consumer) error {
	rms := md.ResourceMetrics()
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
			var err error
			host, err = t.cfg.fallbackHostnameProvider.Hostname(context.Background())
			if err != nil {
				return fmt.Errorf("failed to get fallback host: %w", err)
			}
		}

		// Track hosts if the consumer is a HostConsumer.
		if c, ok := consumer.(HostConsumer); ok {
			c.ConsumeHost(host)
		}

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
				switch md.DataType() {
				case pdata.MetricDataTypeGauge:
					t.mapNumberMetrics(ctx, consumer, md.Name(), Gauge, md.Gauge().DataPoints(), additionalTags, host)
				case pdata.MetricDataTypeSum:
					switch md.Sum().AggregationTemporality() {
					case pdata.MetricAggregationTemporalityCumulative:
						if t.cfg.SendMonotonic && isCumulativeMonotonic(md) {
							t.mapNumberMonotonicMetrics(ctx, consumer, md.Name(), md.Sum().DataPoints(), additionalTags, host)
						} else {
							t.mapNumberMetrics(ctx, consumer, md.Name(), Gauge, md.Sum().DataPoints(), additionalTags, host)
						}
					case pdata.MetricAggregationTemporalityDelta:
						t.mapNumberMetrics(ctx, consumer, md.Name(), Count, md.Sum().DataPoints(), additionalTags, host)
					default: // pdata.AggregationTemporalityUnspecified or any other not supported type
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
						t.mapHistogramMetrics(ctx, consumer, md.Name(), md.Histogram().DataPoints(), delta, additionalTags, host)
					default: // pdata.AggregationTemporalityUnspecified or any other not supported type
						t.logger.Debug("Unknown or unsupported aggregation temporality",
							zap.String("metric name", md.Name()),
							zap.Any("aggregation temporality", md.Histogram().AggregationTemporality()),
						)
						continue
					}
				case pdata.MetricDataTypeSummary:
					t.mapSummaryMetrics(ctx, consumer, md.Name(), md.Summary().DataPoints(), additionalTags, host)
				default: // pdata.MetricDataTypeNone or any other not supported type
					t.logger.Debug("Unknown or unsupported metric type", zap.String(metricName, md.Name()), zap.Any("data type", md.DataType()))
					continue
				}
			}
		}
	}
	return nil
}
