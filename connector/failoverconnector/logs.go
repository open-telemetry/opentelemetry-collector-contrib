// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logsFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *failoverRouter[consumer.Logs]
	logger   *zap.Logger
}

func (f *logsFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs will try to export to the current set priority level and handle failover in the case of an error
func (f *logsFailover) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	return nil
}

func (f *logsFailover) Shutdown(_ context.Context) error {
	return nil
}

func newLogsToLogs(set connector.CreateSettings, cfg component.Config, logs consumer.Logs) (connector.Logs, error) {
	config := cfg.(*Config)
	lr, ok := logs.(connector.LogsRouter)
	if !ok {
		return nil, errors.New("consumer is not of type LogsRouter")
	}

	failover := newFailoverRouter[consumer.Logs](lr.Consumer, config) // temp add type spec to resolve linter issues
	return &logsFailover{
		config:   config,
		failover: failover,
		logger:   set.TelemetrySettings.Logger,
	}, nil
}
