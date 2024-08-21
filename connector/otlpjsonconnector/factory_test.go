// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonconnector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/testdata"
)

func TestNewFactory(t *testing.T) {
	cfg := &Config{}

	lc, err := consumer.NewLogs(func(context.Context, plog.Logs) error {
		return nil
	})
	assert.NoError(t, err)

	factory := NewFactory()
	conn, err := factory.CreateLogsToLogs(context.Background(),
		connectortest.NewNopSettings(), cfg, lc)

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestLogsToLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	sink := &consumertest.LogsSink{}
	conn, err := factory.CreateLogsToLogs(context.Background(),
		connectortest.NewNopSettings(), cfg, sink)
	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.False(t, conn.Capabilities().MutatesData)

	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	lp := testdata.GenerateLogs(1)
	marshaler := &plog.JSONMarshaler{}
	b, err := marshaler.MarshalLogs(lp)
	require.NoError(t, err)

	testLogs := testdata.GenerateLogs(1)
	testLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr(string(b))
	assert.NoError(t, conn.ConsumeLogs(context.Background(), testLogs))

	time.Sleep(1 * time.Second)
	require.Len(t, sink.AllLogs(), 1)
	assert.EqualValues(t, lp, sink.AllLogs()[0])
}
