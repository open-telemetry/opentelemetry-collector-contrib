// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genaiadapterconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/genaiadapterconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/genaiadapterconnector/adapters"
)

type genAIAdapterConnector struct {
	logger         *zap.Logger
	tracesConsumer consumer.Traces
	logsConsumer   consumer.Logs
	lloHandler     *lloHandler
}

func (c *genAIAdapterConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (c *genAIAdapterConnector) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (c *genAIAdapterConnector) Shutdown(_ context.Context) error {
	return nil
}

func (c *genAIAdapterConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	c.transformTraces(td)

	logs := c.lloHandler.processSpans(td)

	if c.tracesConsumer != nil {
		if err := c.tracesConsumer.ConsumeTraces(ctx, td); err != nil {
			return err
		}
	}

	if c.logsConsumer != nil && logs.LogRecordCount() > 0 {
		if err := c.logsConsumer.ConsumeLogs(ctx, logs); err != nil {
			return err
		}
	}

	return nil
}

func (c *genAIAdapterConnector) transformTraces(td ptrace.Traces) {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				attrs := span.Attributes()
				if _, ok := attrs.Get("openinference.span.kind"); ok {
					adapters.TransformOpenInferenceSpan(span)
				}
			}
		}
	}
}
