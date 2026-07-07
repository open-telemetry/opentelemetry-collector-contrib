// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/criticalpath"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/traceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/transactions"
)

type coralogixProcessor struct {
	config *Config
	component.StartFunc
	component.ShutdownFunc
	logger *zap.Logger
}

func newCoralogixProcessor(ctx context.Context, set processor.Settings, cfg *Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	sp := &coralogixProcessor{
		config: cfg,
		logger: set.Logger.With(zap.String("component", "coralogixprocessor")),
	}

	return processorhelper.NewTraces(ctx,
		set,
		cfg,
		nextConsumer,
		sp.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (sp *coralogixProcessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	if !sp.config.TransactionsConfig.Enabled && !sp.config.CriticalPathConfig.Enabled {
		return td, nil
	}
	if td.SpanCount() == 0 {
		return td, nil
	}

	spansByTraceID := traceutil.GroupSpansByTraceID(td)
	if sp.config.TransactionsConfig.Enabled && sp.config.CriticalPathConfig.Enabled {
		transactionLogger := sp.logger.With(zap.String("feature", "transactions"))
		criticalPathLogger := sp.logger.With(zap.String("feature", "critical_path"))
		for traceID, spans := range spansByTraceID {
			tree := traceutil.BuildTraceTree(spans)
			transactions.ApplyTransactionAttributesToTree(tree, transactionLogger)
			criticalpath.ApplyCriticalPathAttributesToTree(traceID, tree, criticalPathLogger)
		}
		return td, nil
	}

	if sp.config.TransactionsConfig.Enabled {
		transactions.ApplyTransactionsAttributesByTraceID(
			spansByTraceID,
			sp.logger.With(zap.String("feature", "transactions")),
		)
	}

	if sp.config.CriticalPathConfig.Enabled {
		criticalpath.ApplyCriticalPathAttributesByTraceID(
			spansByTraceID,
			sp.logger.With(zap.String("feature", "critical_path")),
		)
	}

	return td, nil
}
