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
	exp "go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

func NewFactory() exp.Factory {
	return exp.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exp.WithLogs(createLogsExporter, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		RetrySettings:      exporterhelper.NewDefaultRetrySettings(),
		AWSSessionSettings: awsutil.CreateDefaultSessionConfig(),
		QueueSettings: QueueSettings{
			QueueSize: exporterhelper.NewDefaultQueueSettings().QueueSize,
		},
	}
}

func createLogsExporter(_ context.Context, params exp.CreateSettings, config component.Config) (exp.Logs, error) {
	expConfig, ok := config.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration type; can't cast to awscloudwatchlogsexporter.Config")
	}
	return newCwLogsExporter(expConfig, params)

}
