// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

var Type = component.MustNewType("alertsgen")

func NewFactory() connector.Factory { return connector.NewFactory( Type, createDefaultConfig, connector.WithTracesToLogs(createTracesToLogs, component.StabilityLevelAlpha), connector.WithMetricsToLogs(createMetricsToLogs, component.StabilityLevelAlpha), connector.WithTracesToMetrics(createTracesToMetrics, component.StabilityLevelAlpha) ) }

func createTracesToLogs(_ context.Context, params connector.Settings, cfg component.Config, next consumer.Logs) (connector.Traces, error) { return newTracesToLogs(params, cfg, next) }

func createMetricsToLogs(_ context.Context, params connector.Settings, cfg component.Config, next consumer.Logs) (connector.Metrics, error) { return newMetricsToLogs(params, cfg, next) }

func createTracesToMetrics(_ context.Context, params connector.Settings, cfg component.Config, next consumer.Metrics) (connector.Traces, error) { return newTracesToMetrics(params, cfg, next) }
