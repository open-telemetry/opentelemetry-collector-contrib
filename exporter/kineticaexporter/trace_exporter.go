// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kineticaexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
func newTracesExporter(logger *zap.Logger, _ *Config) *kineticaTracesExporter {
	tracesExp := &kineticaTracesExporter{
		logger: logger,
	}
	return tracesExp
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
func (e *kineticaTracesExporter) pushTraceData(_ context.Context, _ ptrace.Traces) error {
	return nil
}
