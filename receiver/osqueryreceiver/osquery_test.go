// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver

import (
	"context"
	"testing"

	osquery "github.com/osquery/osquery-go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestOsQueryLogFactory(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Queries = []string{"select * from block_devices"}
	cfg.makeClient = func(string) (*osquery.ExtensionManagerClient, error) {
		return nil, nil
	}

	recv, err := createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, recv, "receiver creation failed")

	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestNewOsQueryLog(t *testing.T) {
	testLogs := plog.NewLogs()
	ld := newLog(testLogs, "select * from block_devices", map[string]string{"foo": "bar", "test": "test"})
	require.NotNil(t, ld)
	require.Equal(t, 1, ld.ResourceLogs().Len())
	require.Equal(t, 2, ld.ResourceLogs().At(0).Resource().Attributes().Len())
	require.Equal(t, 1, ld.ResourceLogs().At(0).ScopeLogs().Len())
	require.Equal(t, 1, ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().Len())
	require.Equal(t, 1, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())

	queryLogRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, "select * from block_devices", queryLogRecord.Body().AsString())
	require.Equal(t, plog.SeverityNumberInfo, queryLogRecord.SeverityNumber())
	require.Equal(t, 0, queryLogRecord.Attributes().Len())
}
