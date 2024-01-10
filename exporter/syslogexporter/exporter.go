// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type syslogexporter struct {
	config    *Config
	logger    *zap.Logger
	tlsConfig *tls.Config
	formatter formatter
}

func initExporter(cfg *Config, createSettings exporter.CreateSettings) (*syslogexporter, error) {
	tlsConfig, err := cfg.TLSSetting.LoadTLSConfig()
	if err != nil {
		return nil, err
	}

	cfg.Network = strings.ToLower(cfg.Network)

	s := &syslogexporter{
		config:    cfg,
		logger:    createSettings.Logger,
		tlsConfig: tlsConfig,
		formatter: createFormatter(cfg.Protocol),
	}

	s.logger.Info("Syslog Exporter configured",
		zap.String("endpoint", cfg.Endpoint),
		zap.String("protocol", cfg.Protocol),
		zap.Int("port", cfg.Port),
	)

	return s, nil
}

func newLogsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg *Config,
) (exporter.Logs, error) {
	s, err := initExporter(cfg, params)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the logs exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		params,
		cfg,
		s.pushLogsData,
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(cfg.QueueSettings),
	)
}

func (se *syslogexporter) pushLogsData(_ context.Context, logs plog.Logs) error {
	batchMessages := strings.ToLower(se.config.Network) == "tcp"
	var err error
	if batchMessages {
		err = se.exportBatch(logs)
	} else {
		err = se.exportNonBatch(logs)
	}
	return err
}

func (se *syslogexporter) exportBatch(logs plog.Logs) error {
	var payload strings.Builder
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)
				formatted := se.formatter.format(logRecord)
				payload.WriteString(formatted)
			}
		}
	}

	if payload.Len() > 0 {
		sender, err := connect(se.logger, se.config, se.tlsConfig)
		if err != nil {
			return consumererror.NewLogs(err, logs)
		}
		defer sender.close()
		err = sender.Write(payload.String())
		if err != nil {
			return consumererror.NewLogs(err, logs)
		}
	}
	return nil
}

func (se *syslogexporter) exportNonBatch(logs plog.Logs) error {
	sender, err := connect(se.logger, se.config, se.tlsConfig)
	if err != nil {
		return consumererror.NewLogs(err, logs)
	}
	defer sender.close()

	errs := []error{}
	droppedLogs := plog.NewLogs()
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		droppedResourceLogs := droppedLogs.ResourceLogs().AppendEmpty()
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			droppedScopeLogs := droppedResourceLogs.ScopeLogs().AppendEmpty()
			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)
				formatted := se.formatter.format(logRecord)
				err = sender.Write(formatted)
				if err != nil {
					errs = append(errs, err)
					droppedLogRecord := droppedScopeLogs.LogRecords().AppendEmpty()
					logRecord.CopyTo(droppedLogRecord)
				}
			}
		}
	}

	if len(errs) > 0 {
		errs = deduplicateErrors(errs)
		return consumererror.NewLogs(multierr.Combine(errs...), droppedLogs)
	}

	return nil
}
