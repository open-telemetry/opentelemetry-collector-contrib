// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

type contextCapturingLogsConsumer struct {
	ctx context.Context
}

func (c *contextCapturingLogsConsumer) ConsumeLogs(
	ctx context.Context,
	_ plog.Logs,
) error {
	c.ctx = ctx
	return nil
}

func (c *contextCapturingLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func TestTransformProcessorLifecycle(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	cfg.LogStatements = []common.ContextStatements{
		{
			Context: "log",
			Statements: []string{
				`set(attributes["test"], "value")`,
			},
		},
	}

	next := &contextCapturingLogsConsumer{}

	p, err := factory.CreateLogs(
		t.Context(),
		processortest.NewNopSettings(metadata.Type),
		cfg,
		next,
	)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))

	ctxKey := struct{}{}
	ctx := context.WithValue(t.Context(), ctxKey, "test-value")

	require.NoError(t, p.ConsumeLogs(ctx, plog.NewLogs()))

	require.Same(t, ctx, next.ctx)
	require.Equal(t, "test-value", next.ctx.Value(ctxKey))

	require.NoError(t, p.Shutdown(t.Context()))
}
