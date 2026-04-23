// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	rcvr "go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

func TestStorage(t *testing.T) {
	ctx := t.Context()

	storageExt := storagetest.NewFileBackedStorageExtension("test", t.TempDir())
	host := storagetest.NewStorageHost().
		WithExtension(storageExt.ID, storageExt)

	id := storageExt.ID
	r := createReceiver(t, id)
	require.NoError(t, r.Start(ctx, host))

	myBytes := []byte("my_value")
	require.NoError(t, r.storageClient.Set(ctx, "key", myBytes))
	val, err := r.storageClient.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes, val)

	// Cycle the receiver
	require.NoError(t, r.Shutdown(ctx))
	for _, e := range host.GetExtensions() {
		require.NoError(t, e.Shutdown(ctx))
	}

	r = createReceiver(t, id)
	err = r.Start(ctx, host)
	require.NoError(t, err)

	// Value has persisted
	val, err = r.storageClient.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes, val)

	err = r.storageClient.Delete(ctx, "key")
	require.NoError(t, err)

	// Value is gone
	val, err = r.storageClient.Get(ctx, "key")
	require.NoError(t, err)
	require.Nil(t, val)

	require.NoError(t, r.Shutdown(ctx))

	_, err = r.storageClient.Get(ctx, "key")
	require.Error(t, err)
	require.Equal(t, "client closed", err.Error())
}

func TestFindCorrectStorageExtension(t *testing.T) {
	correctStoragedExt := storagetest.NewInMemoryStorageExtension("want")
	id := correctStoragedExt.ID
	r := createReceiver(t, id)
	host := storagetest.NewStorageHost().
		WithNonStorageExtension("one").
		WithFileBackedStorageExtension("foo", t.TempDir()).
		WithExtension(id, correctStoragedExt).
		WithFileBackedStorageExtension("bar", t.TempDir()).
		WithNonStorageExtension("two")

	err := r.Start(t.Context(), host)
	require.NoError(t, err)
	require.NotNil(t, r.storageClient)
	defer func() { require.NoError(t, r.Shutdown(t.Context())) }()

	clientCreatorID, err := storagetest.CreatorID(t.Context(), r.storageClient)
	require.NoError(t, err)
	require.Equal(t, id, clientCreatorID)
}

func TestFailOnMissingStorageExtension(t *testing.T) {
	id := component.MustNewIDWithName("test", "missing")
	r := createReceiver(t, id)
	err := r.Start(t.Context(), storagetest.NewStorageHost())
	require.Error(t, err)
	require.Equal(t, "storage client: storage extension 'test/missing' not found", err.Error())
	require.NoError(t, r.Shutdown(t.Context()))
}

func TestFailOnNonStorageExtension(t *testing.T) {
	nonStorageExt := storagetest.NewNonStorageExtension("non")
	id := nonStorageExt.ID
	r := createReceiver(t, id)
	host := storagetest.NewStorageHost().
		WithExtension(id, nonStorageExt)

	err := r.Start(t.Context(), host)
	require.Error(t, err)
	require.Equal(t, "storage client: non-storage extension 'non_storage/non' found", err.Error())
}

func createReceiver(t *testing.T, storageID component.ID) *receiver {
	params := rcvr.Settings{
		ID:                component.MustNewID("test"),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)

	logsReceiver, err := factory.CreateLogs(
		t.Context(),
		params,
		factory.CreateDefaultConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err, "receiver should successfully build")

	r, ok := logsReceiver.(*receiver)
	require.True(t, ok)
	r.storageID = &storageID
	return r
}

type mockStorageExtension struct {
	component.StartFunc
	component.ShutdownFunc

	gotKind        component.Kind
	gotComponentID component.ID
	gotStorageName string
}

var _ storage.Extension = (*mockStorageExtension)(nil)

func (r *mockStorageExtension) GetClient(_ context.Context, kind component.Kind, id component.ID, storageName string) (storage.Client, error) {
	r.gotKind = kind
	r.gotComponentID = id
	r.gotStorageName = storageName
	return storage.NewNopClient(), nil
}

func TestGetStorageClientNormalizesComponentID(t *testing.T) {
	ctx := t.Context()
	storageExtID := storagetest.NewStorageID("mock_storage")
	storageExt := &mockStorageExtension{
		StartFunc:    func(context.Context, component.Host) error { return nil },
		ShutdownFunc: func(context.Context) error { return nil },
	}
	host := storagetest.NewStorageHost().WithExtension(storageExtID, storageExt)

	tests := []struct {
		name         string
		componentID  component.ID
		wantTypeName string
	}{
		{
			name:         "strips_underscores_from_type",
			componentID:  component.MustNewIDWithName("my_receiver", "default"),
			wantTypeName: "myreceiver",
		},
		{
			name:         "multiple_underscores",
			componentID:  component.MustNewIDWithName("a_b_c", "inst"),
			wantTypeName: "abc",
		},
		{
			name:         "no_underscores_unchanged",
			componentID:  component.MustNewIDWithName("nounderscores", "default"),
			wantTypeName: "nounderscores",
		},
		{
			name:         "file_log_receiver",
			componentID:  component.MustNewIDWithName("file_log", "default"),
			wantTypeName: "filelog",
		},
		{
			name:         "file_log_receiver",
			componentID:  component.MustNewIDWithName("filelog", "default"),
			wantTypeName: "filelog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := GetStorageClient(ctx, host, &storageExtID, tt.componentID)
			require.NoError(t, err)
			require.NotNil(t, client)
			require.NoError(t, client.Close(ctx))

			wantID := component.MustNewIDWithName(tt.wantTypeName, tt.componentID.Name())
			require.Equal(t, component.KindReceiver, storageExt.gotKind)
			require.Equal(t, wantID, storageExt.gotComponentID)
			require.Empty(t, storageExt.gotStorageName)
		})
	}
}
