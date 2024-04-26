// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/quantile"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics/sketches"
)

var _ metrics.Consumer = (*Consumer)(nil)
var _ metrics.HostConsumer = (*Consumer)(nil)
var _ metrics.TagsConsumer = (*Consumer)(nil)

// Consumer implements metrics.Consumer. It records consumed metrics, sketches and
// APM stats payloads. It provides them to the caller using the All method.
type Consumer struct {
	ms        []datadogV2.MetricSeries
	sl        sketches.SketchSeriesList
	seenHosts map[string]struct{}
	seenTags  map[string]struct{}
}

// NewConsumer creates a new Datadog consumer. It implements metrics.Consumer.
func NewConsumer() *Consumer {
	return &Consumer{
		seenHosts: make(map[string]struct{}),
		seenTags:  make(map[string]struct{}),
	}
}

// toDataType maps translator datatypes to DatadogV2's datatypes.
func (c *Consumer) toDataType(dt metrics.DataType) (out datadogV2.MetricIntakeType) {
	out = datadogV2.METRICINTAKETYPE_UNSPECIFIED

	switch dt {
	case metrics.Count:
		out = datadogV2.METRICINTAKETYPE_COUNT
	case metrics.Gauge:
		out = datadogV2.METRICINTAKETYPE_GAUGE
	}

	return
}

// runningMetrics gets the running metrics for the exporter.
func (c *Consumer) runningMetrics(timestamp uint64, buildInfo component.BuildInfo, metadata metrics.Metadata) (series []datadogV2.MetricSeries) {
	buildTags := TagsFromBuildInfo(buildInfo)
	for host := range c.seenHosts {
		// Report the host as running
		runningMetric := DefaultMetrics("metrics", host, timestamp, buildTags)
		series = append(series, runningMetric...)
	}

	for tag := range c.seenTags {
		runningMetrics := DefaultMetrics("metrics", "", timestamp, buildTags)
		for i := range runningMetrics {
			runningMetrics[i].Tags = append(runningMetrics[i].Tags, tag)
		}
		series = append(series, runningMetrics...)
	}

	for _, lang := range metadata.Languages {
		tags := append(buildTags, "language:"+lang) // nolint
		runningMetric := DefaultMetrics("runtime_metrics", "", timestamp, tags)
		series = append(series, runningMetric...)
	}

	return
}

// All gets all metrics (consumed metrics and running metrics).
func (c *Consumer) All(timestamp uint64, buildInfo component.BuildInfo, tags []string, metadata metrics.Metadata) ([]datadogV2.MetricSeries, sketches.SketchSeriesList) {
	series := c.ms
	series = append(series, c.runningMetrics(timestamp, buildInfo, metadata)...)
	if len(tags) == 0 {
		return series, c.sl
	}
	for i := range series {
		series[i].Tags = append(series[i].Tags, tags...)
	}
	for i := range c.sl {
		c.sl[i].Tags = append(c.sl[i].Tags, tags...)
	}
	return series, c.sl
}

// ConsumeTimeSeries implements the metrics.Consumer interface.
func (c *Consumer) ConsumeTimeSeries(
	_ context.Context,
	dims *metrics.Dimensions,
	typ metrics.DataType,
	timestamp uint64,
	value float64,
) {
	dt := c.toDataType(typ)
	met := NewMetric(dims.Name(), dt, timestamp, value, dims.Tags())
	met.SetResources([]datadogV2.MetricResource{
		{
			Name: datadog.PtrString(dims.Host()),
			Type: datadog.PtrString("host"),
		},
	})

	// datadog-api-client-go does not support `origin_product`, `origin_sub_product` or `origin_product_detail`.
	// We add them as 'AdditionalProperties'. This is undocumented; it adds the fields to the JSON output as-is:
	// https://github.com/DataDog/datadog-api-client-go/blob/f692d3/api/datadogV2/model_metric_origin.go#L153-L155
	// To make things more fun, the `MetricsOrigin` struct has references to deprecated `product` and `service` fields,
	// so to avoid sending these we use `MetricMetadata`'s `AdditionalProperties` field instead of `MetricsOrigin`'s.
	// Should be kept in sync with `DefaultMetrics`
	met.SetMetadata(datadogV2.MetricMetadata{
		AdditionalProperties: map[string]any{
			"origin": map[string]any{
				"origin_product":        int(dims.OriginProduct()),
				"origin_sub_product":    int(dims.OriginSubProduct()),
				"origin_product_detail": int(dims.OriginProductDetail()),
			}},
	})

	c.ms = append(c.ms, met)
}

// ConsumeSketch implements the metrics.Consumer interface.
func (c *Consumer) ConsumeSketch(
	_ context.Context,
	dims *metrics.Dimensions,
	timestamp uint64,
	sketch *quantile.Sketch,
) {
	c.sl = append(c.sl, sketches.SketchSeries{
		Name:     dims.Name(),
		Tags:     dims.Tags(),
		Host:     dims.Host(),
		Interval: 1,
		Points: []sketches.SketchPoint{{
			Ts:     int64(timestamp / 1e9),
			Sketch: sketch,
		}},
	})
}

// ConsumeHost implements the metrics.HostConsumer interface.
func (c *Consumer) ConsumeHost(host string) {
	c.seenHosts[host] = struct{}{}
}

// ConsumeTag implements the metrics.TagsConsumer interface.
func (c *Consumer) ConsumeTag(tag string) {
	c.seenTags[tag] = struct{}{}
}
