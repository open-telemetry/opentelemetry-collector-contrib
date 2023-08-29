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
	rcvr "go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

func TestStorage(t *testing.T) {
	ctx := context.Background()

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

	err := r.Start(context.Background(), host)
	require.NoError(t, err)
	require.NotNil(t, r.storageClient)

	clientCreatorID, err := storagetest.CreatorID(context.Background(), r.storageClient)
	require.NoError(t, err)
	require.Equal(t, id, clientCreatorID)
}

func TestFailOnMissingStorageExtension(t *testing.T) {
	id := component.NewIDWithName("test", "missing")
	r := createReceiver(t, id)
	err := r.Start(context.Background(), storagetest.NewStorageHost())
	require.Error(t, err)
	require.Equal(t, "storage client: storage extension 'test/missing' not found", err.Error())
	require.NoError(t, r.Shutdown(context.Background()))
}

func TestFailOnNonStorageExtension(t *testing.T) {
	nonStorageExt := storagetest.NewNonStorageExtension("non")
	id := nonStorageExt.ID
	r := createReceiver(t, id)
	host := storagetest.NewStorageHost().
		WithExtension(id, nonStorageExt)

	err := r.Start(context.Background(), host)
	require.Error(t, err)
	require.Equal(t, "storage client: non-storage extension 'non_storage/non' found", err.Error())
}

func createReceiver(t *testing.T, storageID component.ID) *receiver {
	params := rcvr.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)

	logsReceiver, err := factory.CreateLogsReceiver(
		context.Background(),
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
