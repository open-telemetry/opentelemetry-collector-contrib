// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvcheckerconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/semconvcheckerconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/semconvcheckerconnector/internal/metadata"
)

// NewFactory creates a factory for example connector.
func NewFactory() connector.Factory {
	// OpenTelemetry connector factory to make a factory for connectors
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToLogs(createTracesToLogsConnector, metadata.TracesToLogsStability),
		connector.WithMetricsToLogs(createMetricToLogsConnector, metadata.MetricsToLogsStability),
		connector.WithLogsToLogs(createLogsToLogsConnector, metadata.LogsToLogsStability))
}

func createTracesToLogsConnector(_ context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Logs) (connector.Traces, error) {
	c, err := newConnector(params, cfg)
	if err != nil {
		return nil, err
	}
	c.logsConsumer = nextConsumer
	return c, nil
}
func createMetricToLogsConnector(_ context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Logs) (connector.Metrics, error) {
	c, err := newConnector(params, cfg)
	if err != nil {
		return nil, err
	}
	c.logsConsumer = nextConsumer
	return c, nil
}
func createLogsToLogsConnector(_ context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Logs) (connector.Logs, error) {
	c, err := newConnector(params, cfg)
	if err != nil {
		return nil, err
	}
	c.logsConsumer = nextConsumer
	return c, nil
}

// schema for connector
type connectorImp struct {
	logsConsumer consumer.Logs
	logger       *zap.Logger

	resource pcommon.Resource

	component.StartFunc
	component.ShutdownFunc
}

// newConnector is a function to create a new connector
func newConnector(params connector.Settings, config component.Config) (*connectorImp, error) {
	params.Logger.Info("Building semconvcheckerconnector connector")
	cfg := config.(*Config)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	res := pcommon.NewResource()
	params.Resource.CopyTo(res)
	res.Attributes().PutStr("connector.name", "semconvchecker")

	return &connectorImp{
		logger:   params.Logger,
		resource: res,
	}, nil
}

var _ connector.Traces = &connectorImp{}

// Capabilities implements the consumer interface.
func (c *connectorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *connectorImp) ConsumeTraces(_ context.Context, _ ptrace.Traces) error {
	return nil
}
func (c *connectorImp) ConsumeMetrics(_ context.Context, _ pmetric.Metrics) error {
	return nil
}
func (c *connectorImp) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	return nil
}
