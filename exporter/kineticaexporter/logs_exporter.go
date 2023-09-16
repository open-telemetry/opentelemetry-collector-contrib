// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kineticaexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type kineticaLogsExporter struct {
	logger *zap.Logger
}

// newLogsExporter
//
//	@param logger
//	@param cfg
//	@return *kineticaLogsExporter
//	@return error
func newLogsExporter(logger *zap.Logger, _ *Config) *kineticaLogsExporter {
	logsExp := &kineticaLogsExporter{
		logger: logger,
	}
	return logsExp
}

func (e *kineticaLogsExporter) start(_ context.Context, _ component.Host) error {

	return nil
}

// shutdown will shut down the exporter.
func (e *kineticaLogsExporter) shutdown(_ context.Context) error {
	return nil
}

// pushLogsData
//
//	@receiver e
//	@param ctx
//	@param ld
//	@return error
func (e *kineticaLogsExporter) pushLogsData(_ context.Context, _ plog.Logs) error {
	return nil
}
