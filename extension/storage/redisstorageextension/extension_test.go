// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisstorageextension

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestExtensionIntegrity(t *testing.T) {
	t.Skip("Requires a Redis cluster to be present at localhost:6379")
	ctx := t.Context()
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
		for i := range keys {
			err := c.Set(ctx, keys[i], myBytes)
			require.NoError(t, err)
		}

		// Repeatedly thrash client
		for range 100 {
			// Make sure my values are still mine
			for i := range keys {
				v, err := c.Get(ctx, keys[i])
				require.NoError(t, err)
				require.Equal(t, myBytes, v)
			}

			// Delete my values
			for i := range keys {
				err := c.Delete(ctx, keys[i])
				require.NoError(t, err)
			}

			// Reset my values
			for i := range keys {
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
	ctx := t.Context()
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
	ctx := t.Context()
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
		ctx := t.Context()
		client := redisClient{
			client: mockedClient,
			prefix: "test_",
		}

		ops := []*storage.Operation{
			{Type: storage.Set, Key: "key1", Value: []byte("val1")},
			{Type: storage.Delete, Key: "key1"},
		}

		mock.ExpectSet(client.prefix+"key1", []byte("val1"), 0).SetVal("OK")
		mock.ExpectDel(client.prefix + "key1").SetVal(1)

		err := client.Batch(ctx, ops...)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("single operations", func(t *testing.T) {
		mockedClient, mock := redismock.NewClientMock()
		ctx := t.Context()
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

func TestIncrement(t *testing.T) {
	newClient := func(expiration time.Duration) (redisClient, redismock.ClientMock) {
		mockedClient, mock := redismock.NewClientMock()
		return redisClient{
			client:     mockedClient,
			prefix:     "test_",
			expiration: expiration,
			logger:     zap.NewNop(),
		}, mock
	}

	t.Run("new key without expiration", func(t *testing.T) {
		client, mock := newClient(0)
		mock.ExpectIncrBy("test_key", 5).SetVal(5)

		v, err := client.IncrementBy(t.Context(), "key", 5)
		require.NoError(t, err)
		require.Equal(t, int64(5), v)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("new key with expiration", func(t *testing.T) {
		client, mock := newClient(time.Minute)
		mock.ExpectIncrBy("test_key", 5).SetVal(5)
		mock.ExpectTTL("test_key").SetVal(-1)
		mock.ExpectExpire("test_key", time.Minute).SetVal(true)

		v, err := client.IncrementBy(t.Context(), "key", 5)
		require.NoError(t, err)
		require.Equal(t, int64(5), v)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("existing key keeps its expiration", func(t *testing.T) {
		client, mock := newClient(time.Minute)
		mock.ExpectIncrBy("test_key", 5).SetVal(12)
		mock.ExpectTTL("test_key").SetVal(30 * time.Second)

		v, err := client.IncrementBy(t.Context(), "key", 5)
		require.NoError(t, err)
		require.Equal(t, int64(12), v)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("existing key without expiration gets one", func(t *testing.T) {
		client, mock := newClient(time.Minute)
		mock.ExpectIncrBy("test_key", 5).SetVal(12)
		mock.ExpectTTL("test_key").SetVal(-1)
		mock.ExpectExpire("test_key", time.Minute).SetVal(true)

		v, err := client.IncrementBy(t.Context(), "key", 5)
		require.NoError(t, err)
		require.Equal(t, int64(12), v)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("negative delta", func(t *testing.T) {
		client, mock := newClient(0)
		mock.ExpectIncrBy("test_key", -3).SetVal(7)

		v, err := client.IncrementBy(t.Context(), "key", -3)
		require.NoError(t, err)
		require.Equal(t, int64(7), v)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("batch with multiple keys", func(t *testing.T) {
		client, mock := newClient(time.Minute)
		mock.MatchExpectationsInOrder(false)
		mock.ExpectIncrBy("test_a", 1).SetVal(1)
		mock.ExpectTTL("test_a").SetVal(-1)
		mock.ExpectIncrBy("test_b", 2).SetVal(10)
		mock.ExpectTTL("test_b").SetVal(30 * time.Second)
		mock.ExpectExpire("test_a", time.Minute).SetVal(true)

		res, err := client.BatchIncrementBy(t.Context(), map[string]int64{"a": 1, "b": 2})
		require.NoError(t, err)
		require.Equal(t, map[string]int64{"a": 1, "b": 10}, res)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty batch", func(t *testing.T) {
		client, mock := newClient(time.Minute)

		res, err := client.BatchIncrementBy(t.Context(), map[string]int64{})
		require.NoError(t, err)
		require.Empty(t, res)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("increment error", func(t *testing.T) {
		client, mock := newClient(0)
		mock.ExpectIncrBy("test_key", 1).SetErr(errors.New("WRONGTYPE"))

		res, err := client.BatchIncrementBy(t.Context(), map[string]int64{"key": 1})
		require.ErrorContains(t, err, "WRONGTYPE")
		require.Nil(t, res)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("expire error is logged, not returned", func(t *testing.T) {
		client, mock := newClient(time.Minute)
		core, logs := observer.New(zap.WarnLevel)
		client.logger = zap.New(core)
		mock.ExpectIncrBy("test_key", 1).SetVal(1)
		mock.ExpectTTL("test_key").SetVal(-1)
		mock.ExpectExpire("test_key", time.Minute).SetErr(errors.New("boom"))

		v, err := client.IncrementBy(t.Context(), "key", 1)
		require.NoError(t, err)
		require.Equal(t, int64(1), v)
		require.Equal(t, 1, logs.FilterMessage("failed to set expiration on new counter keys").Len())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestIncrementConcurrent(t *testing.T) {
	t.Skip("Requires a Redis cluster to be present at localhost:6379")
	ctx := t.Context()
	const goroutines, increments = 10, 100

	newClient := func() redisClient {
		c := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
		t.Cleanup(func() { require.NoError(t, c.Close()) })
		return redisClient{client: c, prefix: "test_incr_", expiration: time.Hour, logger: zap.NewNop()}
	}
	cleanup := newClient()
	require.NoError(t, cleanup.Delete(ctx, "counter"))
	t.Cleanup(func() { require.NoError(t, cleanup.Delete(ctx, "counter")) })

	var wg sync.WaitGroup
	for range goroutines {
		c := newClient()
		wg.Go(func() {
			for range increments {
				_, err := c.IncrementBy(ctx, "counter", 1)
				assert.NoError(t, err)
			}
		})
	}
	wg.Wait()

	v, err := cleanup.Get(ctx, "counter")
	require.NoError(t, err)
	require.Equal(t, []byte("1000"), v)

	// TTL is set when the key is created and not refreshed by later increments.
	ttl, err := cleanup.client.TTL(ctx, "test_incr_counter").Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.NoError(t, cleanup.client.Expire(ctx, "test_incr_counter", 5*time.Minute).Err())
	_, err = cleanup.IncrementBy(ctx, "counter", 1)
	require.NoError(t, err)
	ttl, err = cleanup.client.TTL(ctx, "test_incr_counter").Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.LessOrEqual(t, ttl, 5*time.Minute)
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

	extension, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)
	require.NoError(t, se.Start(t.Context(), componenttest.NewNopHost()))

	return se
}

func newTestEntity(name string) component.ID {
	return component.MustNewIDWithName("nop", name)
}
