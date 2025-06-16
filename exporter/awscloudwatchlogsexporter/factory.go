// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package awscloudwatchlogsexporter provides a logging exporter for the OpenTelemetry collector.
// This package is subject to change and may break configuration settings and behavior.
package awscloudwatchlogsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	queueSettings := exporterhelper.NewDefaultQueueConfig()
	// For backwards compatibilitiy, we default to 1 consumer
	queueSettings.NumConsumers = 1

	return &Config{
		BackOffConfig:      configretry.NewDefaultBackOffConfig(),
		AWSSessionSettings: awsutil.CreateDefaultSessionConfig(),
		QueueSettings:      queueSettings,
	}
}

func createLogsExporter(ctx context.Context, params exporter.Settings, config component.Config) (exporter.Logs, error) {
	expConfig, ok := config.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration type; can't cast to awscloudwatchlogsexporter.Config")
	}
	return newCwLogsExporter(ctx, expConfig, params)
}
