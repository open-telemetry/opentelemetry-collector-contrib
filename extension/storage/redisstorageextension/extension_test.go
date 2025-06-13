// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisstorageextension

import (
	"context"
	"sync"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
)

func TestExtensionIntegrity(t *testing.T) {
	t.Skip("Requires a Redis cluster to be present at localhost:6379")
	ctx := context.Background()
	se := newTestExtension(t)

	type mockComponent struct {
		kind component.Kind
		name component.ID
	}

	components := []mockComponent{
		{kind: component.KindReceiver, name: newTestEntity("receiver_one")},
		{kind: component.KindReceiver, name: newTestEntity("receiver_two")},
		{kind: component.KindProcessor, name: newTestEntity("processor_one")},
		{kind: component.KindProcessor, name: newTestEntity("processor_two")},
		{kind: component.KindExporter, name: newTestEntity("exporter_one")},
		{kind: component.KindExporter, name: newTestEntity("exporter_two")},
		{kind: component.KindExtension, name: newTestEntity("extension_one")},
		{kind: component.KindExtension, name: newTestEntity("extension_two")},
	}

	// Make a client for each component
	clients := make(map[component.ID]storage.Client)
	for _, c := range components {
		client, err := se.GetClient(ctx, c.kind, c.name, "")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, client.Close(ctx))
		})

		clients[c.name] = client
	}

	thrashClient := func(wg *sync.WaitGroup, n component.ID, c storage.Client) {
		// keys and values
		keys := []string{"a", "b", "c", "d", "e"}
		myBytes := []byte(n.Name())

		// Set my values
		for i := 0; i < len(keys); i++ {
			err := c.Set(ctx, keys[i], myBytes)
			require.NoError(t, err)
		}

		// Repeatedly thrash client
		for j := 0; j < 100; j++ {
			// Make sure my values are still mine
			for i := 0; i < len(keys); i++ {
				v, err := c.Get(ctx, keys[i])
				require.NoError(t, err)
				require.Equal(t, myBytes, v)
			}

			// Delete my values
			for i := 0; i < len(keys); i++ {
				err := c.Delete(ctx, keys[i])
				require.NoError(t, err)
			}

			// Reset my values
			for i := 0; i < len(keys); i++ {
				err := c.Set(ctx, keys[i], myBytes)
				require.NoError(t, err)
			}
		}
		wg.Done()
	}

	// Use clients concurrently
	var wg sync.WaitGroup
	for name, client := range clients {
		wg.Add(1)
		go thrashClient(&wg, name, client)
	}
	wg.Wait()
}

func TestClientHandlesSimpleCases(t *testing.T) {
	t.Skip("Requires a Redis cluster to be present at localhost:6379")
	ctx := context.Background()
	se := newTestExtension(t)

	client, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"",
	)

	myBytes := []byte("value")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// Set the data
	err = client.Set(ctx, "key", myBytes)
	require.NoError(t, err)

	// Set it again (nop does not error)
	err = client.Set(ctx, "key", myBytes)
	require.NoError(t, err)

	// Get actual data
	data, err := client.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes, data)

	// Delete the data
	err = client.Delete(ctx, "key")
	require.NoError(t, err)

	// Delete it again (nop does not error)
	err = client.Delete(ctx, "key")
	require.NoError(t, err)

	// Get missing data
	data, err = client.Get(ctx, "key")
	require.NoError(t, err)
	require.Nil(t, data)
}

func TestTwoClientsWithDifferentNames(t *testing.T) {
	t.Skip("Requires a Redis cluster to be present at localhost:6379")
	ctx := context.Background()
	se := newTestExtension(t)

	client1, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"foo",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client1.Close(ctx))
	})

	client2, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"bar",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client2.Close(ctx))
	})

	myBytes1 := []byte("value1")
	myBytes2 := []byte("value2")

	// Set the data
	err = client1.Set(ctx, "key", myBytes1)
	require.NoError(t, err)

	err = client2.Set(ctx, "key", myBytes2)
	require.NoError(t, err)

	// Check it was associated accordingly
	data, err := client1.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes1, data)

	data, err = client2.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes2, data)
}

func TestRedisKey(t *testing.T) {
	t.Run("batch operations", func(t *testing.T) {
		mockedClient, mock := redismock.NewClientMock()
		ctx := context.Background()
		client := redisClient{
			client: mockedClient,
			prefix: "test_",
		}

		ops := []*storage.Operation{
			{Type: storage.Set, Key: "key1", Value: []byte("val1")},
			{Type: storage.Get, Key: "key1"},
			{Type: storage.Delete, Key: "key1"},
		}

		mock.ExpectSet(client.prefix+"key1", []byte("val1"), 0).SetVal("OK")
		mock.ExpectGet(client.prefix + "key1").SetVal("val1")
		mock.ExpectDel(client.prefix + "key1").SetVal(1)

		err := client.Batch(ctx, ops...)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("single operations", func(t *testing.T) {
		mockedClient, mock := redismock.NewClientMock()
		ctx := context.Background()
		client := redisClient{
			client: mockedClient,
			prefix: "test_",
		}

		mock.ExpectSet(client.prefix+"key1", []byte("val1"), 0).SetVal("OK")
		mock.ExpectGet(client.prefix + "key1").SetVal("val1")
		mock.ExpectDel(client.prefix + "key1").SetVal(1)

		err := client.Set(ctx, "key1", []byte("val1"))
		require.NoError(t, err)

		val, err := client.Get(ctx, "key1")
		require.Equal(t, []byte("val1"), val)
		require.NoError(t, err)

		err = client.Delete(ctx, "key1")
		require.NoError(t, err)

		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prefix   string
		ent      component.ID
		kind     string
		name     string
		expected string
	}{
		{
			prefix:   "test_",
			ent:      newTestEntity("my_component"),
			kind:     "receiver",
			name:     "",
			expected: "receiver_nop_my_component_test_",
		},
		{
			prefix:   "",
			ent:      newTestEntity("my_component"),
			kind:     "receiver",
			name:     "",
			expected: "receiver_nop_my_component",
		},
		{
			prefix:   "",
			ent:      newTestEntity("my_component"),
			kind:     "receiver",
			name:     "rdsExt",
			expected: "receiver_nop_my_component_rdsExt",
		},
		{
			prefix:   "",
			ent:      newTestEntity(""),
			kind:     "receiver",
			name:     "rdsExt",
			expected: "receiver_nop__rdsExt",
		},
		{
			prefix:   "",
			ent:      newTestEntity(""),
			kind:     "receiver",
			name:     "",
			expected: "receiver_nop_",
		},
		{
			prefix:   "pref_",
			ent:      newTestEntity("my_test_component"),
			kind:     "receiver",
			name:     "rdsExt",
			expected: "receiver_nop_my_test_component_rdsExt_pref_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			cfg := &Config{
				Prefix: tt.prefix,
			}
			rs := redisStorage{cfg: cfg}
			got := rs.getPrefix(tt.ent, tt.kind, tt.name)
			require.Equal(t, tt.expected, got)
		})
	}
}

func newTestExtension(t *testing.T) storage.Extension {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	extension, err := f.Create(context.Background(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)
	require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))

	return se
}

func newTestEntity(name string) component.ID {
	return component.MustNewIDWithName("nop", name)
}
