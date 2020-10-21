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

package googlecloudpubsubexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"

import (
	"time"

	"go.opentelemetry.io/collector/model/pdata"
)

type MetricsWatermarkFunc func(metrics pdata.Metrics, processingTime time.Time, allowedDrift time.Duration) time.Time
type LogsWatermarkFunc func(logs pdata.Logs, processingTime time.Time, allowedDrift time.Duration) time.Time
type TracesWatermarkFunc func(traces pdata.Traces, processingTime time.Time, allowedDrift time.Duration) time.Time

type collectFunc func(timestamp pdata.Timestamp) bool

type collector struct {
	processingTime time.Time
	allowedDrift   time.Duration
	calculatedTime time.Time
}

func (c *collector) earliest(timestamp pdata.Timestamp) bool {
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

func currentLogsWatermark(_ pdata.Logs, processingTime time.Time, _ time.Duration) time.Time {
	return processingTime
}

func currentMetricsWatermark(_ pdata.Metrics, processingTime time.Time, _ time.Duration) time.Time {
	return processingTime
}

func earliestMetricsWatermark(metrics pdata.Metrics, processingTime time.Time, allowedDrift time.Duration) time.Time {
	collector := &collector{
		processingTime: processingTime,
		allowedDrift:   allowedDrift,
		calculatedTime: processingTime,
	}
	traverseMetrics(metrics, collector.earliest)
	return collector.calculatedTime
}

func traverseMetrics(metrics pdata.Metrics, collect collectFunc) {
	for rix := 0; rix < metrics.ResourceMetrics().Len(); rix++ {
		r := metrics.ResourceMetrics().At(rix)
		for lix := 0; lix < r.InstrumentationLibraryMetrics().Len(); lix++ {
			l := r.InstrumentationLibraryMetrics().At(lix)
			for dix := 0; dix < l.Metrics().Len(); dix++ {
				d := l.Metrics().At(dix)
				switch d.DataType() {
				case pdata.MetricDataTypeHistogram:
					for pix := 0; pix < d.Histogram().DataPoints().Len(); pix++ {
						p := d.Histogram().DataPoints().At(pix)
						if collect(p.Timestamp()) {
							return
						}
					}
				case pdata.MetricDataTypeExponentialHistogram:
					for pix := 0; pix < d.ExponentialHistogram().DataPoints().Len(); pix++ {
						p := d.ExponentialHistogram().DataPoints().At(pix)
						if collect(p.Timestamp()) {
							return
						}
					}
				case pdata.MetricDataTypeSum:
					for pix := 0; pix < d.Sum().DataPoints().Len(); pix++ {
						p := d.Sum().DataPoints().At(pix)
						if collect(p.Timestamp()) {
							return
						}
					}
				case pdata.MetricDataTypeGauge:
					for pix := 0; pix < d.Gauge().DataPoints().Len(); pix++ {
						p := d.Gauge().DataPoints().At(pix)
						if collect(p.Timestamp()) {
							return
						}
					}
				case pdata.MetricDataTypeSummary:
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

func earliestLogsWatermark(logs pdata.Logs, processingTime time.Time, allowedDrift time.Duration) time.Time {
	c := collector{
		processingTime: processingTime,
		allowedDrift:   allowedDrift,
		calculatedTime: processingTime,
	}
	traverseLogs(logs, c.earliest)
	return c.calculatedTime
}

func traverseLogs(logs pdata.Logs, collect collectFunc) {
	for rix := 0; rix < logs.ResourceLogs().Len(); rix++ {
		r := logs.ResourceLogs().At(rix)
		for lix := 0; lix < r.InstrumentationLibraryLogs().Len(); lix++ {
			l := r.InstrumentationLibraryLogs().At(lix)
			for dix := 0; dix < l.Logs().Len(); dix++ {
				d := l.Logs().At(dix)
				if collect(d.Timestamp()) {
					return
				}
			}
		}
	}
}

func currentTracesWatermark(_ pdata.Traces, processingTime time.Time, _ time.Duration) time.Time {
	return processingTime
}

func earliestTracesWatermark(traces pdata.Traces, processingTime time.Time, allowedDrift time.Duration) time.Time {
	c := collector{
		processingTime: processingTime,
		allowedDrift:   allowedDrift,
		calculatedTime: processingTime,
	}
	traverseTraces(traces, c.earliest)
	return c.calculatedTime
}

func traverseTraces(traces pdata.Traces, collect collectFunc) {
	for rix := 0; rix < traces.ResourceSpans().Len(); rix++ {
		r := traces.ResourceSpans().At(rix)
		for lix := 0; lix < r.InstrumentationLibrarySpans().Len(); lix++ {
			l := r.InstrumentationLibrarySpans().At(lix)
			for dix := 0; dix < l.Spans().Len(); dix++ {
				d := l.Spans().At(dix)
				if collect(d.StartTimestamp()) {
					return
				}
			}
		}
	}
}
