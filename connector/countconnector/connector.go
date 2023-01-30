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
	Config
	consumer.Metrics
	component.StartFunc
	component.ShutdownFunc
}

func (c *count) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *count) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	countMetrics := pmetric.NewMetrics()
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceSpan.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		var count int
		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			count += resourceSpan.ScopeSpans().At(j).Spans().Len()
		}
		setCountMetric(countScope.Metrics().AppendEmpty(), c.Config.Traces, count)
	}
	return c.Metrics.ConsumeMetrics(ctx, countMetrics)
}

func (c *count) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	countMetrics := pmetric.NewMetrics()
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceMetric.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		count := 0
		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			countMetrics := resourceMetric.ScopeMetrics().At(j).Metrics()
			for k := 0; k < countMetrics.Len(); k++ {
				countMetric := countMetrics.At(k)
				switch countMetric.Type() {
				case pmetric.MetricTypeGauge:
					count += countMetric.Gauge().DataPoints().Len()
				case pmetric.MetricTypeSum:
					count += countMetric.Sum().DataPoints().Len()
				case pmetric.MetricTypeSummary:
					count += countMetric.Summary().DataPoints().Len()
				case pmetric.MetricTypeHistogram:
					count += countMetric.Histogram().DataPoints().Len()
				case pmetric.MetricTypeExponentialHistogram:
					count += countMetric.ExponentialHistogram().DataPoints().Len()
				}
			}
		}
		setCountMetric(countScope.Metrics().AppendEmpty(), c.Config.Metrics, count)
	}
	return c.Metrics.ConsumeMetrics(ctx, countMetrics)
}

func (c *count) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	countMetrics := pmetric.NewMetrics()
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLog := ld.ResourceLogs().At(i)
		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceLog.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		count := 0
		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			count += resourceLog.ScopeLogs().At(j).LogRecords().Len()
		}
		setCountMetric(countScope.Metrics().AppendEmpty(), c.Config.Logs, count)
	}
	return c.Metrics.ConsumeMetrics(ctx, countMetrics)
}

func setCountMetric(countMetric pmetric.Metric, metricType TypeConfig, count int) {
	countMetric.SetName(metricType.Name)
	countMetric.SetDescription(metricType.Description)
	sum := countMetric.SetEmptySum()
	sum.SetIsMonotonic(false)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetIntValue(int64(count))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
}
