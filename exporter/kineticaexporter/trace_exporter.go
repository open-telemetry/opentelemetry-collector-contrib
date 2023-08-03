// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// kineticaTracesExporter
type kineticaTracesExporter struct {
	logger *zap.Logger
}

// newTracesExporter
//
//	@param logger
//	@param cfg
//	@return *kineticaTracesExporter
//	@return error
func newTracesExporter(logger *zap.Logger, cfg *Config) (*kineticaTracesExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	tracesExp := &kineticaTracesExporter{
		logger: logger,
	}
	return tracesExp, nil
}

func (e *kineticaTracesExporter) start(_ context.Context, _ component.Host) error {
	return nil
}

// shutdown will shut down the exporter.
func (e *kineticaTracesExporter) shutdown(_ context.Context) error {
	return nil
}

// pushTraceData
//
//	@receiver e
//	@param ctx
//	@param td
//	@return error
func (e *kineticaTracesExporter) pushTraceData(_ context.Context, td ptrace.Traces) error {
	var errs []error
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		resourceSpan := resourceSpans.At(i)
		scopeSpans := resourceSpan.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				e.logger.Debug("Added record")
			}
		}
	}

	return multierr.Combine(errs...)
}
