// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type metricsWatermarkFunc func(metrics pmetric.Metrics, processingTime time.Time, allowedDrift time.Duration) time.Time
type logsWatermarkFunc func(logs plog.Logs, processingTime time.Time, allowedDrift time.Duration) time.Time
type tracesWatermarkFunc func(traces ptrace.Traces, processingTime time.Time, allowedDrift time.Duration) time.Time

type collectFunc func(timestamp pcommon.Timestamp) bool

// collector helps traverse the OTLP tree to calculate the final time to set to the ce-time attribute
type collector struct {
	// the current system clock time, set at the start of the tree traversal
	processingTime time.Time
	// maximum allowed difference for the processingTime
	allowedDrift time.Duration
	// calculated time, that can be set each time a timestamp is given to a calculation function
	calculatedTime time.Time
}

// add a new timestamp, and set the calculated time if it's earlier then the current calculated,
// taking into account the allowedDrift
func (c *collector) earliest(timestamp pcommon.Timestamp) bool {
	t := timestamp.AsTime()
	if t.Before(c.calculatedTime) {
		min := c.processingTime.Add(-c.allowedDrift)
		if t.Before(min) {
			c.calculatedTime = min
			return true
		}
		c.calculatedTime = t
	}
	return false
}

// function that doesn't traverse the metric data, return the processingTime
func currentMetricsWatermark(_ pmetric.Metrics, processingTime time.Time, _ time.Duration) time.Time {
	return processingTime
}

// function that traverse the metric data, and returns the earliest timestamp (within limits of the allowedDrift)
func earliestMetricsWatermark(metrics pmetric.Metrics, processingTime time.Time, allowedDrift time.Duration) time.Time {
	collector := &collector{
		processingTime: processingTime,
		allowedDrift:   allowedDrift,
		calculatedTime: processingTime,
	}
	traverseMetrics(metrics, collector.earliest)
	return collector.calculatedTime
}

// traverse the metric data, with a collectFunc
func traverseMetrics(metrics pmetric.Metrics, collect collectFunc) {
	for rix := 0; rix < metrics.ResourceMetrics().Len(); rix++ {
		r := metrics.ResourceMetrics().At(rix)
		for lix := 0; lix < r.ScopeMetrics().Len(); lix++ {
			l := r.ScopeMetrics().At(lix)
			for dix := 0; dix < l.Metrics().Len(); dix++ {
				d := l.Metrics().At(dix)
				switch d.Type() {
				case pmetric.MetricTypeHistogram:
					for pix := 0; pix < d.Histogram().DataPoints().Len(); pix++ {
						p := d.Histogram().DataPoints().At(pix)
						if collect(p.Timestamp()) {
							return
						}
					}
				case pmetric.MetricTypeExponentialHistogram:
					for pix := 0; pix < d.ExponentialHistogram().DataPoints().Len(); pix++ {
						p := d.ExponentialHistogram().DataPoints().At(pix)
						if collect(p.Timestamp()) {
							return
						}
					}
				case pmetric.MetricTypeSum:
					for pix := 0; pix < d.Sum().DataPoints().Len(); pix++ {
						p := d.Sum().DataPoints().At(pix)
						if collect(p.Timestamp()) {
							return
						}
					}
				case pmetric.MetricTypeGauge:
					for pix := 0; pix < d.Gauge().DataPoints().Len(); pix++ {
						p := d.Gauge().DataPoints().At(pix)
						if collect(p.Timestamp()) {
							return
						}
					}
				case pmetric.MetricTypeSummary:
					for pix := 0; pix < d.Summary().DataPoints().Len(); pix++ {
						p := d.Summary().DataPoints().At(pix)
						if collect(p.Timestamp()) {
							return
						}
					}
				}
			}
		}
	}
}

// function that doesn't traverse the log data, return the processingTime
func currentLogsWatermark(_ plog.Logs, processingTime time.Time, _ time.Duration) time.Time {
	return processingTime
}

// function that traverse the log data, and returns the earliest timestamp (within limits of the allowedDrift)
func earliestLogsWatermark(logs plog.Logs, processingTime time.Time, allowedDrift time.Duration) time.Time {
	c := collector{
		processingTime: processingTime,
		allowedDrift:   allowedDrift,
		calculatedTime: processingTime,
	}
	traverseLogs(logs, c.earliest)
	return c.calculatedTime
}

// traverse the log data, with a collectFunc
func traverseLogs(logs plog.Logs, collect collectFunc) {
	for rix := 0; rix < logs.ResourceLogs().Len(); rix++ {
		r := logs.ResourceLogs().At(rix)
		for lix := 0; lix < r.ScopeLogs().Len(); lix++ {
			l := r.ScopeLogs().At(lix)
			for dix := 0; dix < l.LogRecords().Len(); dix++ {
				d := l.LogRecords().At(dix)
				if collect(d.Timestamp()) {
					return
				}
			}
		}
	}
}

// function that doesn't traverse the trace data, return the processingTime
func currentTracesWatermark(_ ptrace.Traces, processingTime time.Time, _ time.Duration) time.Time {
	return processingTime
}

// function that traverse the trace data, and returns the earliest timestamp (within limits of the allowedDrift)
func earliestTracesWatermark(traces ptrace.Traces, processingTime time.Time, allowedDrift time.Duration) time.Time {
	c := collector{
		processingTime: processingTime,
		allowedDrift:   allowedDrift,
		calculatedTime: processingTime,
	}
	traverseTraces(traces, c.earliest)
	return c.calculatedTime
}

// traverse the trace data, with a collectFunc
func traverseTraces(traces ptrace.Traces, collect collectFunc) {
	for rix := 0; rix < traces.ResourceSpans().Len(); rix++ {
		r := traces.ResourceSpans().At(rix)
		for lix := 0; lix < r.ScopeSpans().Len(); lix++ {
			l := r.ScopeSpans().At(lix)
			for dix := 0; dix < l.Spans().Len(); dix++ {
				d := l.Spans().At(dix)
				if collect(d.StartTimestamp()) {
					return
				}
			}
		}
	}
}
