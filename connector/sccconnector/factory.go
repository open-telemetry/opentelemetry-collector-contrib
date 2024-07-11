// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sccconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/sccconnector"

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
)

// this is the name used to refer to the connector in the config.yaml
const (
	typeStr      = "scc"
	scopeName    = "otelcol/sccconnector"
	scopeVersion = "v0.0.1"
	schemaUrl    = "https://opentelemetry.io/schemas/1.26.0"
)

// NewFactory creates a factory for example connector.
func NewFactory() connector.Factory {
	// OpenTelemetry connector factory to make a factory for connectors
	return connector.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		connector.WithTracesToLogs(createTracesToLogsConnector, component.StabilityLevelAlpha),
		connector.WithMetricsToLogs(createMetricToLogsConnector, component.StabilityLevelAlpha),
		connector.WithLogsToLogs(createLogsToLogsConnector, component.StabilityLevelAlpha))
}

func createTracesToLogsConnector(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Logs) (connector.Traces, error) {
	c, err := newConnector(params, cfg)
	if err != nil {
		return nil, err
	}
	c.logsConsumer = nextConsumer
	return c, nil
}
func createMetricToLogsConnector(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Logs) (connector.Metrics, error) {
	c, err := newConnector(params, cfg)
	if err != nil {
		return nil, err
	}
	c.logsConsumer = nextConsumer
	return c, nil
}
func createLogsToLogsConnector(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Logs) (connector.Logs, error) {
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
	params.Logger.Info("Building sccconnector connector")

	res := pcommon.NewResource()
	params.Resource.CopyTo(res)
	res.Attributes().PutStr("service.name", "sccconnector")

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

var matchedBody pcommon.Value = pcommon.NewValueStr("matched")
var notMatchedBody pcommon.Value = pcommon.NewValueStr("not matched")

func (c *connectorImp) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return nil
}
func (c *connectorImp) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return nil
}
func (c *connectorImp) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return nil
}
