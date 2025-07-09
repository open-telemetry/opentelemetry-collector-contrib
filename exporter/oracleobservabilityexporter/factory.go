// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Copyright © 2025, Oracle and/or its affiliates.

package oracleobservabilityexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/oracleobservabilityexporter"

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/oracleobservabilityexporter/internal/metadata"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	filestorage := component.MustNewID("file_storage")
	cfg := &Config{
		BackOffConfig: configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     5 * time.Second,
			RandomizationFactor: 0.5,
			Multiplier:          1.5,
			MaxInterval:         30 * time.Second,
			MaxElapsedTime:      0,
		},
		QueueConfig: exporterhelper.QueueBatchConfig{
			Enabled:         true,
			NumConsumers:    10,
			QueueSize:       1000,
			BlockOnOverflow: true,
			StorageID:       &filestorage,
			WaitForResult:   false,
			Sizer:           exporterhelper.RequestSizerTypeRequests,
		},
		TimeoutConfig: exporterhelper.TimeoutConfig{
			Timeout: 0,
		},
		AuthType: ConfigFile,
	}

	// Disable storage in test environments
	if isTestEnvironment() {
		cfg.QueueConfig.StorageID = nil
	}
	return cfg
}

func isTestEnvironment() bool {
	return testing.Testing()
}

func createLogsExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	config := cfg.(*Config)
	return exporterhelper.NewLogs(
		ctx,
		params,
		config,
		func(_ context.Context, _ plog.Logs) error {
			return nil
		},
	)
}
