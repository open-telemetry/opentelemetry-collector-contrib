// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genaiadapterconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/genaiadapterconnector"

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/genaiadapterconnector/internal/metadata"
)

// must create a wrapper over the existing connector and set NoOp for ConsumeTraces
// to avoid spans from being exported twice
type logsOnlyConnector struct {
	*genAIAdapterConnector
}

func (l *logsOnlyConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if l.tracesConsumer != nil {
		return nil
	}

	l.transformTraces(td)

	logs := l.lloHandler.processSpans(td)

	if l.logsConsumer != nil && logs.LogRecordCount() > 0 {
		if err := l.logsConsumer.ConsumeLogs(ctx, logs); err != nil {
			return err
		}
	}

	return nil
}

var connectors = &sync.Map{}

func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToTraces(createTracesToTraces, metadata.TracesToTracesStability),
		connector.WithTracesToLogs(createTracesToLogs, metadata.TracesToLogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func getOrCreateConnector(set connector.Settings) *genAIAdapterConnector {
	id := set.ID.String()
	c := &genAIAdapterConnector{
		logger:     set.Logger,
		lloHandler: newLLOHandler(set.Logger),
	}
	actual, _ := connectors.LoadOrStore(id, c)
	return actual.(*genAIAdapterConnector)
}

func createTracesToTraces(
	_ context.Context,
	set connector.Settings,
	_ component.Config,
	tracesConsumer consumer.Traces,
) (connector.Traces, error) {
	c := getOrCreateConnector(set)
	c.mu.Lock()
	c.tracesConsumer = tracesConsumer
	c.mu.Unlock()
	return c, nil
}

func createTracesToLogs(
	_ context.Context,
	set connector.Settings,
	_ component.Config,
	logsConsumer consumer.Logs,
) (connector.Traces, error) {
	c := getOrCreateConnector(set)
	c.mu.Lock()
	c.logsConsumer = logsConsumer
	c.mu.Unlock()
	return &logsOnlyConnector{c}, nil
}
