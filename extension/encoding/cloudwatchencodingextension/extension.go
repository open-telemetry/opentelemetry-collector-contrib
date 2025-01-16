// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatchencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/cloudwatchencodingextension"

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/cloudwatch"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var (
	_ encoding.LogsUnmarshalerExtension    = (*cloudwatchExtension)(nil)
	_ encoding.MetricsUnmarshalerExtension = (*cloudwatchExtension)(nil)
)

type cloudwatchExtension struct {
	config *Config
	logger *zap.Logger
}

func createExtension(_ context.Context, settings extension.Settings, config component.Config) (extension.Extension, error) {
	return &cloudwatchExtension{
		config: config.(*Config),
		logger: settings.Logger,
	}, nil
}

func (c *cloudwatchExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (c *cloudwatchExtension) Shutdown(_ context.Context) error {
	return nil
}

func (c *cloudwatchExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	return cloudwatch.UnmarshalLogs(buf)
}

func (c *cloudwatchExtension) UnmarshalMetrics(buf []byte) (pmetric.Metrics, error) {
	return cloudwatch.UnmarshalMetrics(c.config.Format, buf)
}
