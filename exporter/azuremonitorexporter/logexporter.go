// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"context"

	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logExporter struct {
	config           *Config
	transportChannel transportChannel
	logger           *zap.Logger
}

func (exporter *logExporter) onLogData(_ context.Context, logData plog.Logs) error {
	resourceLogs := logData.ResourceLogs()
	logPacker := newLogPacker(exporter.logger)

	for i := 0; i < resourceLogs.Len(); i++ {
		scopeLogs := resourceLogs.At(i).ScopeLogs()
		resource := resourceLogs.At(i).Resource()
		for j := 0; j < scopeLogs.Len(); j++ {
			logs := scopeLogs.At(j).LogRecords()
			scope := scopeLogs.At(j).Scope()
			for k := 0; k < logs.Len(); k++ {
				envelope := logPacker.LogRecordToEnvelope(logs.At(k), resource, scope)
				envelope.IKey = string(exporter.config.InstrumentationKey)
				exporter.transportChannel.Send(envelope)
			}
		}
	}

	return nil
}

// Returns a new instance of the log exporter
func newLogsExporter(config *Config, transportChannel transportChannel, set exporter.CreateSettings) (exporter.Logs, error) {
	exporter := &logExporter{
		config:           config,
		transportChannel: transportChannel,
		logger:           set.Logger,
	}

	return exporterhelper.NewLogsExporter(context.TODO(), set, config, exporter.onLogData)
}
