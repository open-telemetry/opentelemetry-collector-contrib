// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlexemplar"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type parsedContextStatements struct {
	common.MetricsConsumer
	sharedCache bool
}

type Processor struct {
	contexts     []parsedContextStatements
	logger       *zap.Logger
	sharedCaches map[common.ContextID]*pcommon.Map
}

func NewProcessor(contextStatements []common.ContextStatements, errorMode ottl.ErrorMode, settings component.TelemetrySettings, metricFunctions map[string]ottl.Factory[*ottlmetric.TransformContext], dataPointFunctions map[string]ottl.Factory[*ottldatapoint.TransformContext], exemplarFunctions map[string]ottl.Factory[*ottlexemplar.TransformContext]) (*Processor, error) {
	pc, err := common.NewMetricParserCollection(settings, common.WithMetricParser(metricFunctions), common.WithDataPointParser(dataPointFunctions), common.WithExemplarParser(exemplarFunctions), common.WithMetricErrorMode(errorMode))
	if err != nil {
		return nil, err
	}

	contexts := make([]parsedContextStatements, len(contextStatements))
	var errors error
	for i, cs := range contextStatements {
		context, err := pc.ParseContextStatements(cs)
		if err != nil {
			errors = multierr.Append(errors, err)
		}
		contexts[i] = parsedContextStatements{
			MetricsConsumer: context,
			sharedCache:     cs.SharedCache,
		}
	}

	if errors != nil {
		return nil, errors
	}

	var sharedCaches map[common.ContextID]*pcommon.Map
	for _, c := range contexts {
		if c.sharedCache {
			if sharedCaches == nil {
				sharedCaches = map[common.ContextID]*pcommon.Map{}
			}
			m := pcommon.NewMap()
			sharedCaches[c.Context()] = &m
		}
	}

	return &Processor{
		contexts:     contexts,
		logger:       settings.Logger,
		sharedCaches: sharedCaches,
	}, nil
}

func (p *Processor) ProcessMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	for _, c := range p.contexts {
		cache := common.LoadContextCache(p.sharedCaches, c.Context(), c.sharedCache)
		err := c.ConsumeMetrics(ctx, md, cache)
		if err != nil {
			p.logger.Error("failed processing metrics", zap.Error(err))
			return md, err
		}
	}
	return md, nil
}
