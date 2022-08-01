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

	"go.opentelemetry.io/collector/config"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

func TestStorage(t *testing.T) {
	ctx := context.Background()
	r := createReceiver(t)
	host := storagetest.NewStorageHost(t, t.TempDir(), config.NewComponentIDWithName("nop", "test"))
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

	r = createReceiver(t)
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
	require.Equal(t, "database not open", err.Error())
}

func TestNoStorageExtension(t *testing.T) {
	host := storagetest.NewStorageHost(t, t.TempDir())

	r := createReceiver(t)
	require.NoError(t, r.Start(context.Background(), host))
	require.Nil(t, r.storageExtension)
	require.NoError(t, r.Shutdown(context.Background()))
}

func TestNamedStorageExtension(t *testing.T) {
	idOne := config.NewComponentIDWithName("nop", "one")
	idTwo := config.NewComponentIDWithName("nop", "two")
	host := storagetest.NewStorageHost(t, t.TempDir(), idOne, idTwo)

	r := createReceiver(t)
	r.storageID = idTwo
	require.NoError(t, r.Start(context.Background(), host))
	require.Equal(t, host.GetExtensions()[idTwo], r.storageExtension)
	require.NoError(t, r.Shutdown(context.Background()))
}

func TestAutoselectUnnamedStorageExtension(t *testing.T) {
	idOne := config.NewComponentIDWithName("nop", "one")
	host := storagetest.NewStorageHost(t, t.TempDir(), idOne)

	r := createReceiver(t)
	require.NoError(t, r.Start(context.Background(), host))
	require.Equal(t, host.GetExtensions()[idOne], r.storageExtension)
	require.NoError(t, r.Shutdown(context.Background()))
}

func TestDisableStorageExtension(t *testing.T) {
	idOne := config.NewComponentIDWithName("nop", "one")
	idTwo := config.NewComponentIDWithName("nop", "two")

	hostNoExt := storagetest.NewStorageHost(t, t.TempDir())
	hostOneExt := storagetest.NewStorageHost(t, t.TempDir(), idOne)
	hostTwoExt := storagetest.NewStorageHost(t, t.TempDir(), idOne, idTwo)

	rNone := createReceiver(t)
	rNone.storageID = config.NewComponentID("false")
	require.NoError(t, rNone.Start(context.Background(), hostNoExt))
	require.Nil(t, rNone.storageExtension)
	require.NoError(t, rNone.Shutdown(context.Background()))

	rOne := createReceiver(t)
	rOne.storageID = config.NewComponentID("false")
	require.NoError(t, rOne.Start(context.Background(), hostOneExt))
	require.Nil(t, rOne.storageExtension)
	require.NoError(t, rOne.Shutdown(context.Background()))

	rTwo := createReceiver(t)
	rTwo.storageID = config.NewComponentID("false")
	require.NoError(t, rTwo.Start(context.Background(), hostTwoExt))
	require.Nil(t, rTwo.storageExtension)
	require.NoError(t, rTwo.Shutdown(context.Background()))

}

func TestFailAmbiguousStorageExtensions(t *testing.T) {
	idOne := config.NewComponentIDWithName("nop", "one")
	idTwo := config.NewComponentIDWithName("nop", "two")
	host := storagetest.NewStorageHost(t, t.TempDir(), idOne, idTwo)

	r := createReceiver(t)
	err := r.Start(context.Background(), host)
	require.ErrorContains(t, err, "ambiguous storage extension")
}

func TestFailMissingStorageExtensions(t *testing.T) {
	idOne := config.NewComponentIDWithName("nop", "one")
	idTwo := config.NewComponentIDWithName("nop", "two")
	host := storagetest.NewStorageHost(t, t.TempDir(), idOne, idTwo)

	r := createReceiver(t)
	r.storageID = config.NewComponentIDWithName("nop", "three")
	err := r.Start(context.Background(), host)
	require.ErrorContains(t, err, "storage extension not found: nop/three")
}

func createReceiver(t *testing.T) *receiver {
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
	return r
}

func TestPersisterImplementation(t *testing.T) {
	ctx := context.Background()
	myBytes := []byte("string")
	p := newMockPersister()

	err := p.Set(ctx, "key", myBytes)
	require.NoError(t, err)

	val, err := p.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes, val)

	err = p.Delete(ctx, "key")
	require.NoError(t, err)
}
