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

func NewFactory() connector.Factory {
	connectors := &sync.Map{}
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToTraces(
			func(_ context.Context, set connector.Settings, _ component.Config, tc consumer.Traces) (connector.Traces, error) {
				c := getOrCreateConnector(connectors, set)
				c.tracesConsumer = tc
				return c, nil
			},
			metadata.TracesToTracesStability,
		),
		connector.WithTracesToLogs(
			func(_ context.Context, set connector.Settings, _ component.Config, lc consumer.Logs) (connector.Traces, error) {
				c := getOrCreateConnector(connectors, set)
				c.logsConsumer = lc
				return &logsOnlyConnector{c}, nil
			},
			metadata.TracesToLogsStability,
		),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func getOrCreateConnector(connectors *sync.Map, set connector.Settings) *genAIAdapterConnector {
	id := set.ID.String()
	c := &genAIAdapterConnector{
		logger:     set.Logger,
		lloHandler: newLLOHandler(set.Logger),
	}
	actual, _ := connectors.LoadOrStore(id, c)
	return actual.(*genAIAdapterConnector)
}
