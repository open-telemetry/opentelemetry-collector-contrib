// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type syslogexporter struct {
	config    *Config
	logger    *zap.Logger
	tlsConfig *tls.Config
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
	}

	s.logger.Info("Syslog Exporter configured",
		zap.String("endpoint", cfg.Endpoint),
		zap.String("Protocol", cfg.Protocol),
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
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
	)
}

func (se *syslogexporter) logsToMap(record plog.LogRecord) map[string]any {
	attributes := record.Attributes().AsRaw()
	return attributes
}

func (se *syslogexporter) getTimestamp(record plog.LogRecord) time.Time {
	timestamp := record.Timestamp().AsTime()
	return timestamp
}

func (se *syslogexporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	type droppedResourceRecords struct {
		resource pcommon.Resource
		records  []plog.LogRecord
	}
	var (
		errs    []error
		dropped []droppedResourceRecords
	)
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		if droppedRecords, err := se.sendSyslogs(rl); err != nil {
			dropped = append(dropped, droppedResourceRecords{
				resource: rl.Resource(),
				records:  droppedRecords,
			})
			errs = append(errs, err)
		}
	}
	if len(dropped) > 0 {
		ld = plog.NewLogs()
		for i := range dropped {
			rls := ld.ResourceLogs().AppendEmpty()
			logRecords := rls.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			dropped[i].resource.MoveTo(rls.Resource())
			for j := 0; j < len(dropped[i].records); j++ {
				dropped[i].records[j].MoveTo(logRecords)
			}
		}
		errs = deduplicateErrors(errs)
		return consumererror.NewLogs(multierr.Combine(errs...), ld)
	}
	se.logger.Info("Connected successfully, exporting logs....")
	return nil
}

func (se *syslogexporter) sendSyslogs(rl plog.ResourceLogs) ([]plog.LogRecord, error) {
	var (
		errs           []error
		droppedRecords []plog.LogRecord
	)
	slgs := rl.ScopeLogs()
	for i := 0; i < slgs.Len(); i++ {
		slg := slgs.At(i)
		for j := 0; j < slg.LogRecords().Len(); j++ {
			lr := slg.LogRecords().At(j)
			formattedLine := se.logsToMap(lr)
			timestamp := se.getTimestamp(lr)
			s, errConn := connect(se.logger, se.config, se.tlsConfig)
			if errConn != nil {
				droppedRecords = append(droppedRecords, lr)
				errs = append(errs, errConn)
				continue
			}
			defer s.close()
			err := s.Write(formattedLine, timestamp)
			if err != nil {
				droppedRecords = append(droppedRecords, lr)
				errs = append(errs, err)
			}
		}
	}
	return droppedRecords, multierr.Combine(errs...)
}
