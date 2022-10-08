// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

func TestStorage(t *testing.T) {
	ctx := context.Background()

	storageExt := storagetest.NewFileBackedStorageExtension("test", t.TempDir())
	host := storagetest.NewStorageHost().
		WithExtension(storageExt.ID(), storageExt)

	id := storageExt.ID()
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
	id := correctStoragedExt.ID()
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
	id := config.NewComponentIDWithName("test", "missing")
	r := createReceiver(t, id)
	err := r.Start(context.Background(), storagetest.NewStorageHost())
	require.Error(t, err)
	require.Equal(t, "storage client: storage extension 'test/missing' not found", err.Error())
}

func TestFailOnNonStorageExtension(t *testing.T) {
	nonStorageExt := storagetest.NewNonStorageExtension("non")
	id := nonStorageExt.ID()
	r := createReceiver(t, id)
	host := storagetest.NewStorageHost().
		WithExtension(id, nonStorageExt)

	err := r.Start(context.Background(), host)
	require.Error(t, err)
	require.Equal(t, "storage client: non-storage extension 'non_storage/non' found", err.Error())
}

func createReceiver(t *testing.T, storageID config.ComponentID) *receiver {
	params := component.ReceiverCreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	factory := NewFactory(TestReceiverType{}, component.StabilityLevelInDevelopment)

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
