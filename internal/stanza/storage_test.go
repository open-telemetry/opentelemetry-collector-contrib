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

package stanza

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

func TestStorage(t *testing.T) {
	ctx := context.Background()
	tempDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	r := createReceiver(t)
	host := storagetest.NewStorageHost(t, tempDir, "test")
	err = r.Start(ctx, host)
	require.NoError(t, err)

	myBytes := []byte("my_value")

	r.storageClient.Set(ctx, "key", myBytes)
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

func TestFailOnMultipleStorageExtensions(t *testing.T) {
	ctx := context.Background()
	tempDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	r := createReceiver(t)
	host := storagetest.NewStorageHost(t, tempDir, "one", "two")
	err = r.Start(ctx, host)
	require.Error(t, err)
	require.Equal(t, "storage client: multiple storage extensions found", err.Error())
}

func createReceiver(t *testing.T) *receiver {
	params := component.ReceiverCreateSettings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zaptest.NewLogger(t),
		},
	}
	mockConsumer := mockLogsConsumer{}

	factory := NewFactory(TestReceiverType{})

	logsReceiver, err := factory.CreateLogsReceiver(
		context.Background(),
		params,
		factory.CreateDefaultConfig(),
		&mockConsumer,
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
