// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/sumconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

// sum can sum attribute values from spans, span event, metrics, data points, or log records
// and emit the sums onto a metrics pipeline.
type sum struct {
	metricsConsumer consumer.Metrics
	component.StartFunc
	component.ShutdownFunc

	spansMetricDefs      map[string]metricDef[ottlspan.TransformContext]
	spanEventsMetricDefs map[string]metricDef[ottlspanevent.TransformContext]
	metricsMetricDefs    map[string]metricDef[ottlmetric.TransformContext]
	dataPointsMetricDefs map[string]metricDef[ottldatapoint.TransformContext]
	logsMetricDefs       map[string]metricDef[ottllog.TransformContext]
}

func (c *sum) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *sum) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	sumMetrics := pmetric.NewMetrics()
	sumMetrics.ResourceMetrics().EnsureCapacity(td.ResourceSpans().Len())

	return c.metricsConsumer.ConsumeMetrics(ctx, sumMetrics)
}

func (c *sum) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	sumMetrics := pmetric.NewMetrics()
	sumMetrics.ResourceMetrics().EnsureCapacity(md.ResourceMetrics().Len())

	return c.metricsConsumer.ConsumeMetrics(ctx, sumMetrics)
}

func (c *sum) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	sumMetrics := pmetric.NewMetrics()
	sumMetrics.ResourceMetrics().EnsureCapacity(ld.ResourceLogs().Len())

	return c.metricsConsumer.ConsumeMetrics(ctx, sumMetrics)
}
