// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
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

func TestCollect_Collections(t *testing.T) {
	makeClient := func(string) (client, error) {
		return testClient{
			queryResult: []map[string]string{
				{
					"hostname": "test-host",
				},
			},
		}, nil
	}
	cfg := &Config{Collections: []string{"system_info"}}
	rcvr := &osQueryReceiver{
		config:       cfg,
		logger:       zap.NewNop(),
		createClient: makeClient,
		collections:  resolveCollections(cfg.Collections, zap.NewNop()),
	}
	ld, err := rcvr.collect(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, ld.ResourceLogs().Len())

	logRecord := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	collectionAttr, ok := logRecord.Attributes().Get("collection")
	require.True(t, ok)
	require.Equal(t, "system_info", collectionAttr.AsString())
}

func TestCollect_UnknownCollectionIsSkipped(t *testing.T) {
	cfg := &Config{Collections: []string{"not_a_real_collection"}}
	resolved := resolveCollections(cfg.Collections, zap.NewNop())
	require.Empty(t, resolved)
}

type erroringClient struct{}

func (erroringClient) Close() {}

func (erroringClient) QueryRowsContext(context.Context, string) ([]map[string]string, error) {
	return nil, errors.New("boom")
}

func TestCollect_CollectionsDiffAcrossCycles(t *testing.T) {
	current := []map[string]string{{"username": "alice"}}
	makeClient := func(string) (client, error) {
		return testClient{queryResult: current}, nil
	}
	cfg := &Config{Collections: []string{"users_info"}}
	rcvr := &osQueryReceiver{
		config:       cfg,
		logger:       zap.NewNop(),
		createClient: makeClient,
		collections:  resolveCollections(cfg.Collections, zap.NewNop()),
		state:        newCollectionState(storage.NewNopClient(), zap.NewNop()),
	}

	ld, err := rcvr.collect(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, ld.ResourceLogs().Len())
	require.Equal(t, 1, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())

	// Second cycle, unchanged rows: nothing new or modified, nothing emitted.
	ld, err = rcvr.collect(t.Context())
	require.NoError(t, err)
	require.Equal(t, 0, ld.ResourceLogs().Len())

	// Third cycle, one new row: only that row is emitted.
	current = []map[string]string{
		{"username": "alice"},
		{"username": "bob"},
	}
	ld, err = rcvr.collect(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, ld.ResourceLogs().Len())
	logRecords := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 1, logRecords.Len())
	usernameAttr, ok := logRecords.At(0).Attributes().Get("username")
	require.True(t, ok)
	require.Equal(t, "bob", usernameAttr.AsString())
}

func TestCollect_QueryErrorDoesNotCorruptCollectionState(t *testing.T) {
	saved := []map[string]string{{"username": "alice"}}
	callCount := 0
	makeClient := func(string) (client, error) {
		callCount++
		if callCount == 2 {
			return erroringClient{}, nil
		}
		return testClient{queryResult: saved}, nil
	}
	cfg := &Config{Collections: []string{"users_info"}}
	rcvr := &osQueryReceiver{
		config:       cfg,
		logger:       zap.NewNop(),
		createClient: makeClient,
		collections:  resolveCollections(cfg.Collections, zap.NewNop()),
		state:        newCollectionState(storage.NewNopClient(), zap.NewNop()),
	}

	_, err := rcvr.collect(t.Context())
	require.NoError(t, err)

	_, err = rcvr.collect(t.Context())
	require.Error(t, err)

	previous, loadErr := rcvr.state.load(t.Context(), "users_info")
	require.NoError(t, loadErr)
	require.Equal(t, saved, previous)
}

func TestSnapshotCollect_AlwaysEmitsAll(t *testing.T) {
	rows := []map[string]string{{"username": "alice"}}
	makeClient := func(string) (client, error) {
		return testClient{queryResult: rows}, nil
	}
	cfg := &Config{Collections: []string{"users_info"}}
	rcvr := &osQueryReceiver{
		config:       cfg,
		logger:       zap.NewNop(),
		createClient: makeClient,
		collections:  resolveCollections(cfg.Collections, zap.NewNop()),
		state:        newCollectionState(storage.NewNopClient(), zap.NewNop()),
	}

	_, err := rcvr.collect(t.Context())
	require.NoError(t, err)

	// A snapshot cycle emits every row even though nothing changed since the
	// last change-only cycle.
	ld, err := rcvr.snapshotCollect(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, ld.ResourceLogs().Len())
	require.Equal(t, 1, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
}

func TestStartShutdown_ResolvesStorageClient(t *testing.T) {
	cfg := &Config{Queries: []string{"select 1"}}
	rcvr := newOsQueryReceiver(cfg, receivertest.NewNopSettings(metadata.Type))

	require.NoError(t, rcvr.start(t.Context(), componenttest.NewNopHost()))
	require.NotNil(t, rcvr.state)
	require.NoError(t, rcvr.shutdown(t.Context()))
}

func TestStart_UnknownStorageExtension(t *testing.T) {
	id := component.MustNewID("does_not_exist")
	cfg := &Config{Queries: []string{"select 1"}, StorageID: &id}
	rcvr := newOsQueryReceiver(cfg, receivertest.NewNopSettings(metadata.Type))

	require.Error(t, rcvr.start(t.Context(), componenttest.NewNopHost()))
}
