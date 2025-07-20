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

func (*kineticaLogsExporter) start(context.Context, component.Host) error {
	return nil
}

// shutdown will shut down the exporter.
func (*kineticaLogsExporter) shutdown(context.Context) error {
	return nil
}

// pushLogsData
//
//	@receiver e
//	@param ctx
//	@param ld
//	@return error
func (*kineticaLogsExporter) pushLogsData(context.Context, plog.Logs) error {
	return nil
}
