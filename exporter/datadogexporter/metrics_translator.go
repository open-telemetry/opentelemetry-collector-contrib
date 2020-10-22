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

package datadogexporter

import (
	"fmt"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
)

const (
	// Gauge is the Datadog Gauge metric type
	Gauge string = "gauge"
)

// newGauge creates a new Datadog Gauge metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func newGauge(name string, ts uint64, value float64, tags []string) datadog.Metric {
	// Transform UnixNano timestamp into Unix timestamp
	// 1 second = 1e9 ns
	timestamp := float64(ts / 1e9)

	gauge := datadog.Metric{
		Points: []datadog.DataPoint{[2]*float64{&timestamp, &value}},
		Tags:   tags,
	}
	gauge.SetMetric(name)
	gauge.SetType(Gauge)
	return gauge
}

// getTags maps a stringMap into a slice of Datadog tags
func getTags(labels pdata.StringMap) []string {
	tags := make([]string, 0, labels.Len())
	labels.ForEach(func(key string, v pdata.StringValue) {
		value := v.Value()
		if value == "" {
			// Tags can't end with ":" so we replace empty values with "n/a"
			value = "n/a"
		}
		tags = append(tags, fmt.Sprintf("%s:%s", key, value))
	})
	return tags
}

// mapIntMetrics maps int datapoints into Datadog metrics
func mapIntMetrics(name string, slice pdata.IntDataPointSlice) []datadog.Metric {
	// Allocate assuming none are nil
	metrics := make([]datadog.Metric, 0, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		if p.IsNil() {
			continue
		}
		metrics = append(metrics,
			newGauge(name, uint64(p.Timestamp()), float64(p.Value()), getTags(p.LabelsMap())),
		)
	}
	return metrics
}

// mapDoubleMetrics maps double datapoints into Datadog metrics
func mapDoubleMetrics(name string, slice pdata.DoubleDataPointSlice) []datadog.Metric {
	// Allocate assuming none are nil
	metrics := make([]datadog.Metric, 0, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		if p.IsNil() {
			continue
		}
		metrics = append(metrics,
			newGauge(name, uint64(p.Timestamp()), float64(p.Value()), getTags(p.LabelsMap())),
		)
	}
	return metrics
}

// mapIntHistogramMetrics maps histogram metrics slices to Datadog metrics
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
func mapIntHistogramMetrics(name string, slice pdata.IntHistogramDataPointSlice, buckets bool) []datadog.Metric {
	// Allocate assuming none are nil and no buckets
	metrics := make([]datadog.Metric, 0, 2*slice.Len())
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		if p.IsNil() {
			continue
		}
		ts := uint64(p.Timestamp())
		tags := getTags(p.LabelsMap())

		metrics = append(metrics,
			newGauge(fmt.Sprintf("%s.count", name), ts, float64(p.Count()), tags),
			newGauge(fmt.Sprintf("%s.sum", name), ts, float64(p.Sum()), tags),
		)

		if buckets {
			// We have a single metric, 'count_per_bucket', which is tagged with the bucket id. See:
			// https://github.com/DataDog/opencensus-go-exporter-datadog/blob/c3b47f1c6dcf1c47b59c32e8dbb7df5f78162daa/stats.go#L99-L104
			fullName := fmt.Sprintf("%s.count_per_bucket", name)
			for idx, count := range p.BucketCounts() {
				bucketTags := append(tags, fmt.Sprintf("bucket_idx:%d", idx))
				metrics = append(metrics,
					newGauge(fullName, ts, float64(count), bucketTags),
				)
			}
		}
	}
	return metrics
}

// mapIntHistogramMetrics maps double histogram metrics slices to Datadog metrics
//
// see mapIntHistogramMetrics docs for further details.
func mapDoubleHistogramMetrics(name string, slice pdata.DoubleHistogramDataPointSlice, buckets bool) []datadog.Metric {
	// Allocate assuming none are nil and no buckets
	metrics := make([]datadog.Metric, 0, 2*slice.Len())
	for i := 0; i < slice.Len(); i++ {
		p := slice.At(i)
		if p.IsNil() {
			continue
		}
		ts := uint64(p.Timestamp())
		tags := getTags(p.LabelsMap())

		metrics = append(metrics,
			newGauge(fmt.Sprintf("%s.count", name), ts, float64(p.Count()), tags),
			newGauge(fmt.Sprintf("%s.sum", name), ts, float64(p.Sum()), tags),
		)

		if buckets {
			// We have a single metric, 'count_per_bucket', which is tagged with the bucket id. See:
			// https://github.com/DataDog/opencensus-go-exporter-datadog/blob/c3b47f1c6dcf1c47b59c32e8dbb7df5f78162daa/stats.go#L99-L104
			fullName := fmt.Sprintf("%s.count_per_bucket", name)
			for idx, count := range p.BucketCounts() {
				bucketTags := append(tags, fmt.Sprintf("bucket_idx:%d", idx))
				metrics = append(metrics,
					newGauge(fullName, ts, float64(count), bucketTags),
				)
			}
		}
	}
	return metrics
}

// MapMetrics maps OTLP metrics into the DataDog format
func MapMetrics(logger *zap.Logger, cfg config.MetricsConfig, md pdata.Metrics) (series []datadog.Metric, droppedTimeSeries int) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		if rm.IsNil() {
			continue
		}
		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			if ilm.IsNil() {
				continue
			}
			metrics := ilm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				md := metrics.At(k)
				if md.IsNil() {
					continue
				}
				var datapoints []datadog.Metric
				switch md.DataType() {
				case pdata.MetricDataTypeNone:
					continue
				case pdata.MetricDataTypeIntGauge:
					datapoints = mapIntMetrics(md.Name(), md.IntGauge().DataPoints())
				case pdata.MetricDataTypeDoubleGauge:
					datapoints = mapDoubleMetrics(md.Name(), md.DoubleGauge().DataPoints())
				case pdata.MetricDataTypeIntSum:
					// Ignore aggregation temporality; report raw values
					datapoints = mapIntMetrics(md.Name(), md.IntSum().DataPoints())
				case pdata.MetricDataTypeDoubleSum:
					// Ignore aggregation temporality; report raw values
					datapoints = mapDoubleMetrics(md.Name(), md.DoubleSum().DataPoints())
				case pdata.MetricDataTypeIntHistogram:
					datapoints = mapIntHistogramMetrics(md.Name(), md.IntHistogram().DataPoints(), cfg.Buckets)
				case pdata.MetricDataTypeDoubleHistogram:
					datapoints = mapDoubleHistogramMetrics(md.Name(), md.DoubleHistogram().DataPoints(), cfg.Buckets)
				}
				series = append(series, datapoints...)
			}
		}
	}
	return
}
