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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"
)

const metricName string = "metric name"

// HostnameProvider gets a hostname
type HostnameProvider interface {
	// Hostname gets the hostname from the machine.
	Hostname(ctx context.Context) (string, error)
}

type Translator struct {
	prevPts                  *TTLCache
	logger                   *zap.Logger
	cfg                      config.MetricsConfig
	buildInfo                component.BuildInfo
	fallbackHostnameProvider HostnameProvider
}

func New(cache *TTLCache, params component.ExporterCreateSettings, cfg config.MetricsConfig, fallbackHostProvider HostnameProvider) *Translator {
	return &Translator{cache, params.Logger, cfg, params.BuildInfo, fallbackHostProvider}
}

// getTags maps an attributeMap into a slice of Datadog tags
func getTags(labels pdata.AttributeMap) []string {
	tags := make([]string, 0, labels.Len())
	labels.Range(func(key string, value pdata.AttributeValue) bool {
		v := value.AsString()
		if v == "" {
			// Tags can't end with ":" so we replace empty values with "n/a"
			v = "n/a"
		}
		tags = append(tags, fmt.Sprintf("%s:%s", key, v))
		return true
	})
	return tags
}

// isCumulativeMonotonic checks if a metric is a cumulative monotonic metric
func isCumulativeMonotonic(md pdata.Metric) bool {
	switch md.DataType() {
	case pdata.MetricDataTypeSum:
		return md.Sum().AggregationTemporality() == pdata.AggregationTemporalityCumulative &&
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
func (t *Translator) mapNumberMetrics(name string, dt metrics.MetricDataType, slice pdata.NumberDataPointSlice, attrTags []string) []datadog.Metric {
	ms := make([]datadog.Metric, 0, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		tags := getTags(p.Attributes())
		tags = append(tags, attrTags...)
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
func (t *Translator) mapNumberMonotonicMetrics(name string, slice pdata.NumberDataPointSlice, attrTags []string) []datadog.Metric {
	ms := make([]datadog.Metric, 0, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		ts := uint64(p.Timestamp())
		tags := getTags(p.Attributes())
		tags = append(tags, attrTags...)

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
// We follow a similar approach to our OpenCensus exporter:
// we report sum and count by default; buckets count can also
// be reported (opt-in), but bounds are ignored.
func (t *Translator) mapHistogramMetrics(name string, slice pdata.HistogramDataPointSlice, attrTags []string) []datadog.Metric {
	// Allocate assuming none are nil and no buckets
	ms := make([]datadog.Metric, 0, 2*slice.Len())
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		ts := uint64(p.Timestamp())
		tags := getTags(p.Attributes())
		tags = append(tags, attrTags...)

		ms = append(ms,
			metrics.NewGauge(fmt.Sprintf("%s.count", name), ts, float64(p.Count()), tags),
		)

		sumName := fmt.Sprintf("%s.sum", name)
		if !t.isSkippable(sumName, p.Sum()) {
			ms = append(ms, metrics.NewGauge(sumName, ts, p.Sum(), tags))
		}

		if t.cfg.Buckets {
			// We have a single metric, 'count_per_bucket', which is tagged with the bucket id. See:
			// https://github.com/DataDog/opencensus-go-exporter-datadog/blob/c3b47f1c6dcf1c47b59c32e8dbb7df5f78162daa/stats.go#L99-L104
			fullName := fmt.Sprintf("%s.count_per_bucket", name)
			for idx, count := range p.BucketCounts() {
				bucketTags := append(tags, fmt.Sprintf("bucket_idx:%d", idx))
				ms = append(ms,
					metrics.NewGauge(fullName, ts, float64(count), bucketTags),
				)
			}
		}
	}
	return ms
}

// getQuantileTag returns the quantile tag for summary types.
// Since the summary type is provided as a compatibility feature, we try to format the float number as close as possible to what
// we do on the Datadog Agent Python OpenMetrics check, which, in turn, tries to
// follow https://github.com/OpenObservability/OpenMetrics/blob/v1.0.0/specification/OpenMetrics.md#considerations-canonical-numbers
func getQuantileTag(quantile float64) string {
	// We handle 0 and 1 separately since they are special
	if quantile == 0 {
		// we do this differently on our check
		return "quantile:0"
	} else if quantile == 1.0 {
		// it needs to have a '.0' added at the end according to the spec
		return "quantile:1.0"
	}
	return fmt.Sprintf("quantile:%s", strconv.FormatFloat(quantile, 'g', -1, 64))
}

// mapSummaryMetrics maps summary datapoints into Datadog metrics
func (t *Translator) mapSummaryMetrics(name string, slice pdata.SummaryDataPointSlice, attrTags []string) []datadog.Metric {
	// Allocate assuming none are nil and no quantiles
	ms := make([]datadog.Metric, 0, 2*slice.Len())
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		ts := uint64(p.Timestamp())
		tags := getTags(p.Attributes())
		tags = append(tags, attrTags...)

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

				quantileTags := append(tags, getQuantileTag(q.Quantile()))
				ms = append(ms,
					metrics.NewGauge(fullName, ts, q.Value(), quantileTags),
				)
			}
		}
	}
	return ms
}

// MapMetrics maps OTLP metrics into the DataDog format
func (t *Translator) MapMetrics(md pdata.Metrics) (series []datadog.Metric) {
	pushTime := uint64(time.Now().UTC().UnixNano())
	rms := md.ResourceMetrics()
	seenHosts := make(map[string]struct{})
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		var attributeTags []string

		// Only fetch attribute tags if they're not already converted into labels.
		// Otherwise some tags would be present twice in a metric's tag list.
		if !t.cfg.ExporterConfig.ResourceAttributesAsTags {
			attributeTags = attributes.TagsFromAttributes(rm.Resource().Attributes())
		}

		host, ok := attributes.HostnameFromAttributes(rm.Resource().Attributes())
		if !ok {
			fallbackHost, err := t.fallbackHostnameProvider.Hostname(context.Background())
			host = ""
			if err == nil {
				host = fallbackHost
			}
		}
		seenHosts[host] = struct{}{}

		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			metricsArray := ilm.Metrics()
			for k := 0; k < metricsArray.Len(); k++ {
				md := metricsArray.At(k)
				var datapoints []datadog.Metric
				switch md.DataType() {
				case pdata.MetricDataTypeGauge:
					datapoints = t.mapNumberMetrics(md.Name(), metrics.Gauge, md.Gauge().DataPoints(), attributeTags)
				case pdata.MetricDataTypeSum:
					switch md.Sum().AggregationTemporality() {
					case pdata.AggregationTemporalityCumulative:
						if t.cfg.SendMonotonic && isCumulativeMonotonic(md) {
							datapoints = t.mapNumberMonotonicMetrics(md.Name(), md.Sum().DataPoints(), attributeTags)
						} else {
							datapoints = t.mapNumberMetrics(md.Name(), metrics.Gauge, md.Sum().DataPoints(), attributeTags)
						}
					case pdata.AggregationTemporalityDelta:
						datapoints = t.mapNumberMetrics(md.Name(), metrics.Count, md.Sum().DataPoints(), attributeTags)
					default: // pdata.AggregationTemporalityUnspecified or any other not supported type
						t.logger.Debug("Unknown or unsupported aggregation temporality",
							zap.String(metricName, md.Name()),
							zap.Any("aggregation temporality", md.Sum().AggregationTemporality()),
						)
						continue
					}
				case pdata.MetricDataTypeHistogram:
					datapoints = t.mapHistogramMetrics(md.Name(), md.Histogram().DataPoints(), attributeTags)
				case pdata.MetricDataTypeSummary:
					datapoints = t.mapSummaryMetrics(md.Name(), md.Summary().DataPoints(), attributeTags)
				default: // pdata.MetricDataTypeNone or any other not supported type
					t.logger.Debug("Unknown or unsupported metric type", zap.String(metricName, md.Name()), zap.Any("data type", md.DataType()))
					continue
				}

				for i := range datapoints {
					datapoints[i].SetHost(host)
				}

				series = append(series, datapoints...)
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
