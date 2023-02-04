// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package countconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const scopeName = "otelcol/countconnector"

// count can count spans, data points, or log records and emit
// the count onto a metrics pipeline.
type count struct {
	cfg             Config
	metricsConsumer consumer.Metrics
	component.StartFunc
	component.ShutdownFunc
}

func (c *count) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *count) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	now := time.Now()
	countMetrics := pmetric.NewMetrics()
	countMetrics.ResourceMetrics().EnsureCapacity(td.ResourceSpans().Len())
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceSpan.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countResource.ScopeMetrics().EnsureCapacity(resourceSpan.ScopeSpans().Len())
		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		var count uint64
		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			count += uint64(resourceSpan.ScopeSpans().At(j).Spans().Len())
		}
		setCountMetric(countScope.Metrics().AppendEmpty(), c.cfg.Traces, count, now)
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}

func (c *count) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	now := time.Now()
	countMetrics := pmetric.NewMetrics()
	countMetrics.ResourceMetrics().EnsureCapacity(md.ResourceMetrics().Len())
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceMetric.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countResource.ScopeMetrics().EnsureCapacity(resourceMetric.ScopeMetrics().Len())
		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		var count uint64
		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			countMetrics := resourceMetric.ScopeMetrics().At(j).Metrics()
			for k := 0; k < countMetrics.Len(); k++ {
				countMetric := countMetrics.At(k)
				switch countMetric.Type() {
				case pmetric.MetricTypeGauge:
					count += uint64(countMetric.Gauge().DataPoints().Len())
				case pmetric.MetricTypeSum:
					count += uint64(countMetric.Sum().DataPoints().Len())
				case pmetric.MetricTypeSummary:
					count += uint64(countMetric.Summary().DataPoints().Len())
				case pmetric.MetricTypeHistogram:
					count += uint64(countMetric.Histogram().DataPoints().Len())
				case pmetric.MetricTypeExponentialHistogram:
					count += uint64(countMetric.ExponentialHistogram().DataPoints().Len())
				}
			}
		}
		setCountMetric(countScope.Metrics().AppendEmpty(), c.cfg.Metrics, count, now)
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}

func (c *count) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	now := time.Now()
	countMetrics := pmetric.NewMetrics()
	countMetrics.ResourceMetrics().EnsureCapacity(ld.ResourceLogs().Len())
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLog := ld.ResourceLogs().At(i)
		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceLog.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countResource.ScopeMetrics().EnsureCapacity(resourceLog.ScopeLogs().Len())
		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		var count uint64
		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			count += uint64(resourceLog.ScopeLogs().At(j).LogRecords().Len())
		}
		setCountMetric(countScope.Metrics().AppendEmpty(), c.cfg.Logs, count, now)
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}

func setCountMetric(countMetric pmetric.Metric, metricType MetricInfo, count uint64, now time.Time) {
	countMetric.SetName(metricType.Name)
	countMetric.SetDescription(metricType.Description)
	sum := countMetric.SetEmptySum()
	// The delta value is always positive, so a value accumulated downstream is monotonic
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetIntValue(int64(count))
	// TODO determine appropriate start time
	dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
}
