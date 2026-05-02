// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

type Processor struct {
	contexts []common.MetricsConsumer
	logger   *zap.Logger
	tracer   trace.Tracer
}

func NewProcessor(contextStatements []common.ContextStatements, errorMode ottl.ErrorMode, settings component.TelemetrySettings, metricFunctions map[string]ottl.Factory[*ottlmetric.TransformContext], dataPointFunctions map[string]ottl.Factory[*ottldatapoint.TransformContext]) (*Processor, error) {
	pc, err := common.NewMetricParserCollection(settings, common.WithMetricParser(metricFunctions), common.WithDataPointParser(dataPointFunctions), common.WithMetricErrorMode(errorMode))
	if err != nil {
		return nil, err
	}

	contexts := make([]common.MetricsConsumer, len(contextStatements))
	var errors error
	for i, cs := range contextStatements {
		context, err := pc.ParseContextStatements(cs)
		if err != nil {
			errors = multierr.Append(errors, err)
		}
		contexts[i] = context
	}

	if errors != nil {
		return nil, errors
	}

	return &Processor{
		contexts: contexts,
		logger:   settings.Logger,
		tracer:   metadata.Tracer(settings),
	}, nil
}

func (p *Processor) ProcessMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	ctx, span := p.tracer.Start(ctx, "transform.ProcessMetrics")
	defer span.End()

	for _, c := range p.contexts {
		ctxChild, contextSpan := p.tracer.Start(ctx, "transform.context",
			trace.WithAttributes(attribute.String("transform.context.type", string(c.Context()))))
		err := c.ConsumeMetrics(ctxChild, md)
		if err != nil {
			contextSpan.RecordError(err)
			contextSpan.End()
			p.logger.Error("failed processing metrics", zap.Error(err))
			span.RecordError(err)
			return md, err
		}
		contextSpan.End()
	}

	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int("transform.context.count", len(p.contexts)),
			attribute.Int("transform.resource_metrics.count", md.ResourceMetrics().Len()),
		)
	}

	return md, nil
}
