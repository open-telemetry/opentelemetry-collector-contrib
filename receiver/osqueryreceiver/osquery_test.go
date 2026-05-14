// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/metadata"
)

func TestOsQueryLogFactory(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Queries = []string{"select * from block_devices"}

	recv, err := createLogsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, recv, "receiver creation failed")

	err = recv.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = recv.Shutdown(t.Context())
	require.NoError(t, err)
}

type testClient struct {
	queryResult []map[string]string
}

func (testClient) Close() {
}

func (t testClient) QueryRowsContext(_ context.Context, _ string) ([]map[string]string, error) {
	return t.queryResult, nil
}

func TestCollect(t *testing.T) {
	makeClient := func(string) (client, error) {
		return testClient{
			queryResult: []map[string]string{
				{
					"test": "test",
					"foo":  "foo",
				},
			},
		}, nil
	}
	rcvr := &osQueryReceiver{
		config:       &Config{Queries: []string{"select * from block_devices"}},
		logger:       zap.NewNop(),
		createClient: makeClient,
	}
	ld, err := rcvr.collect(t.Context())
	require.NoError(t, err)
	require.NotNil(t, ld)
	require.Equal(t, 1, ld.ResourceLogs().Len())
	require.Equal(t, 1, ld.ResourceLogs().At(0).ScopeLogs().Len())
	require.Equal(t, 1, ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().Len())
	require.Equal(t, 1, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())

	queryLogRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, "select * from block_devices", queryLogRecord.Body().AsString())
	require.Equal(t, plog.SeverityNumberInfo, queryLogRecord.SeverityNumber())
	require.Equal(t, 2, queryLogRecord.Attributes().Len())
}
