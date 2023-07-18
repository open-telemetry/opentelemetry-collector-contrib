// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exceptionsconnector

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/metadata"
)

func TestConnectorLogConsumeTraces(t *testing.T) {
	traces := []ptrace.Traces{buildSampleTrace()}
	lsink := new(consumertest.LogsSink)

	p, err := newTestLogsConnector(t, lsink, zaptest.NewLogger(t))
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err = p.Start(ctx, componenttest.NewNopHost())
	defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
	require.NoError(t, err)

	expectedLogs, err := golden.ReadLogs("testdata/logs.yml")
	require.NoError(t, err)

	for _, traces := range traces {
		err = p.ConsumeTraces(ctx, traces)
		assert.NoError(t, err)

		logs := lsink.AllLogs()
		assert.Len(t, logs, 1)
		err = plogtest.CompareLogs(expectedLogs, logs[len(logs)-1])
		assert.NoError(t, err)
	}
}

func newTestLogsConnector(t *testing.T, lcon consumer.Logs, logger *zap.Logger) (*logsConnector, error) {
	cfg := &Config{
		Dimensions: []Dimension{
			{Name: exceptionTypeKey},
			{Name: exceptionMessageKey},
		},
	}

	c, err := newLogsConnector(logger, cfg)
	c.logsConsumer = lcon
	return c, err
}
