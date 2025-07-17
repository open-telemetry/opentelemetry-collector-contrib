// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"context"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/quantile"
	"go.opentelemetry.io/collector/component"
	zorkian "gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics/sketches"
)

var (
	_ metrics.Consumer     = (*ZorkianConsumer)(nil)
	_ metrics.HostConsumer = (*ZorkianConsumer)(nil)
	_ metrics.TagsConsumer = (*ZorkianConsumer)(nil)
)

// ZorkianConsumer implements metrics.Consumer. It records consumed metrics, sketches and
// APM stats payloads. It provides them to the caller using the All method.
type ZorkianConsumer struct {
	ms        []zorkian.Metric
	sl        sketches.SketchSeriesList
	seenHosts map[string]struct{}
	seenTags  map[string]struct{}
}

// NewZorkianConsumer creates a new ZorkianConsumer. It implements metrics.Consumer.
func NewZorkianConsumer() *ZorkianConsumer {
	return &ZorkianConsumer{
		seenHosts: make(map[string]struct{}),
		seenTags:  make(map[string]struct{}),
	}
}

// toDataType maps translator datatypes to Zorkian's datatypes.
func (c *ZorkianConsumer) toDataType(dt metrics.DataType) (out MetricType) {
	out = MetricType("unknown")

	switch dt {
	case metrics.Count:
		out = Count
	case metrics.Gauge:
		out = Gauge
	}

	return
}

// runningMetrics gets the running metrics for the exporter.
func (c *ZorkianConsumer) runningMetrics(timestamp uint64, buildInfo component.BuildInfo) (series []zorkian.Metric) {
	for host := range c.seenHosts {
		// Report the host as running
		runningMetric := DefaultZorkianMetrics("metrics", host, timestamp, buildInfo)
		series = append(series, runningMetric...)
	}

	for tag := range c.seenTags {
		runningMetrics := DefaultZorkianMetrics("metrics", "", timestamp, buildInfo)
		for i := range runningMetrics {
			runningMetrics[i].Tags = append(runningMetrics[i].Tags, tag)
		}
		series = append(series, runningMetrics...)
	}

	return
}

// All gets all metrics (consumed metrics and running metrics).
func (c *ZorkianConsumer) All(timestamp uint64, buildInfo component.BuildInfo, tags []string) ([]zorkian.Metric, sketches.SketchSeriesList) {
	series := c.ms
	series = append(series, c.runningMetrics(timestamp, buildInfo)...)
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
func (c *ZorkianConsumer) ConsumeTimeSeries(
	_ context.Context,
	dims *metrics.Dimensions,
	typ metrics.DataType,
	timestamp uint64,
	value float64,
) {
	dt := c.toDataType(typ)
	met := NewZorkianMetric(dims.Name(), dt, timestamp, value, dims.Tags())
	met.SetHost(dims.Host())
	c.ms = append(c.ms, met)
}

// ConsumeSketch implements the metrics.Consumer interface.
func (c *ZorkianConsumer) ConsumeSketch(
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
func (c *ZorkianConsumer) ConsumeHost(host string) {
	c.seenHosts[host] = struct{}{}
}

// ConsumeTag implements the metrics.TagsConsumer interface.
func (c *ZorkianConsumer) ConsumeTag(tag string) {
	c.seenTags[tag] = struct{}{}
}
