// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

type gatedStorage struct {
	storage.Extension
	entered, canceled, release, closed chan struct{}
	setOnce, closeOnce                 sync.Once
}

func (s *gatedStorage) GetClient(ctx context.Context, kind component.Kind, id component.ID, name string) (storage.Client, error) {
	client, err := s.Extension.GetClient(ctx, kind, id, name)
	if err != nil {
		return nil, err
	}
	return &gatedClient{Client: client, gate: s}, nil
}

type gatedClient struct {
	storage.Client
	gate *gatedStorage
}

func (c *gatedClient) Set(ctx context.Context, key string, value []byte) error {
	first := false
	c.gate.setOnce.Do(func() { first = true })
	if first {
		close(c.gate.entered)
		select {
		case <-ctx.Done():
			close(c.gate.canceled)
			<-c.gate.release
		case <-c.gate.release:
		}
	}
	return c.Client.Set(ctx, key, value)
}

func (c *gatedClient) Close(ctx context.Context) error {
	c.gate.closeOnce.Do(func() { close(c.gate.closed) })
	return c.Client.Close(ctx)
}

func TestShutdownWaitsForPeriodicSave(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		id := storagetest.NewStorageID("test")
		ext := &gatedStorage{
			Extension: storagetest.NewInMemoryStorageExtension("test"),
			entered:   make(chan struct{}), canceled: make(chan struct{}),
			release: make(chan struct{}), closed: make(chan struct{}),
		}
		var once sync.Once
		release := func() { once.Do(func() { close(ext.release) }) }
		cfg := createDefaultConfig().(*Config)
		cfg.Storage = &id
		cfg.SaveInterval = time.Millisecond
		p := newManualProcessor(t, cfg)
		require.NoError(t, p.Start(ctx, storagetest.NewStorageHost().WithExtension(id, ext)))
		defer func() {
			release()
			_ = p.Shutdown(context.WithoutCancel(ctx))
		}()
		select {
		case <-ext.entered:
		case <-ctx.Done():
			t.Fatal("periodic save did not start")
		}
		done := make(chan error, 1)
		go func() { done <- p.Shutdown(ctx) }()
		select {
		case <-ext.canceled:
		case <-ctx.Done():
			t.Fatal("worker did not observe shutdown")
		}
		synctest.Wait()
		select {
		case <-ext.closed:
			t.Error("storage closed before the periodic save completed")
		default:
		}
		var shutdownErr error
		returnedEarly := false
		select {
		case shutdownErr = <-done:
			returnedEarly = true
			t.Error("shutdown returned before the periodic save completed")
		default:
		}
		release()
		if !returnedEarly {
			select {
			case shutdownErr = <-done:
			case <-ctx.Done():
				t.Fatal("shutdown did not finish")
			}
		}
		require.NoError(t, shutdownErr)
		require.NoError(t, p.Shutdown(ctx))
	})
}

func TestShutdownSnapshotIntegrity(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	id := storagetest.NewStorageID("test")
	ext := &gatedStorage{
		Extension: storagetest.NewFileBackedStorageExtension("test", t.TempDir()),
		entered:   make(chan struct{}), canceled: make(chan struct{}),
		release: make(chan struct{}), closed: make(chan struct{}),
	}
	var once sync.Once
	release := func() { once.Do(func() { close(ext.release) }) }
	t.Cleanup(release)
	cfg := createDefaultConfig().(*Config)
	cfg.Storage = &id
	cfg.SaveInterval = time.Millisecond
	p := newManualProcessor(t, cfg)
	require.NoError(t, p.Start(ctx, storagetest.NewStorageHost().WithExtension(id, ext)))
	t.Cleanup(func() {
		release()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cleanupCancel()
		_ = p.Shutdown(cleanupCtx)
	})
	_, err := p.processLogs(ctx, makeLogRecord("first synthetic event"))
	require.NoError(t, err)
	select {
	case <-ext.entered:
	case <-ctx.Done():
		t.Fatal("periodic save did not start")
	}
	_, err = p.processLogs(ctx, makeLogRecord("second distinct synthetic event with additional words"))
	require.NoError(t, err)
	p.mu.Lock()
	want, err := p.drain.Snapshot()
	p.mu.Unlock()
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- p.Shutdown(ctx) }()
	select {
	case <-ext.canceled:
	case <-ctx.Done():
		t.Fatal("worker did not observe shutdown")
	}
	release()
	select {
	case err = <-done:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("shutdown did not finish")
	}
	require.NoError(t, p.Shutdown(ctx))
	cfg2 := createDefaultConfig().(*Config)
	cfg2.Storage = &id
	p2 := newManualProcessor(t, cfg2)
	require.NoError(t, p2.Start(ctx, storagetest.NewStorageHost().WithExtension(id, ext.Extension)))
	got, err := p2.drain.Snapshot()
	require.NoError(t, err)
	assert.JSONEq(t, string(want), string(got))
	_, ok := p2.drain.Match("second distinct synthetic event with additional words")
	assert.True(t, ok)
	require.NoError(t, p2.Shutdown(ctx))
}

type errorClient struct {
	storage.Client
	setErr, closeErr error
}

func (c *errorClient) Set(context.Context, string, []byte) error { return c.setErr }
func (c *errorClient) Close(context.Context) error               { return c.closeErr }

func TestShutdownAggregatesStorageErrors(t *testing.T) {
	setErr, closeErr := errors.New("set failed"), errors.New("close failed")
	p := newManualProcessor(t, createDefaultConfig().(*Config))
	p.storageClient = &errorClient{setErr: setErr, closeErr: closeErr}
	err := p.Shutdown(t.Context())
	require.ErrorIs(t, err, setErr)
	require.ErrorIs(t, err, closeErr)
}

type retryCloseClient struct {
	storage.Client
	closeErr   error
	closeCalls int
}

func (c *retryCloseClient) Close(ctx context.Context) error {
	c.closeCalls++
	if c.closeCalls == 1 {
		return c.closeErr
	}
	return c.Client.Close(ctx)
}

func TestShutdownRetriesFailedClose(t *testing.T) {
	closeErr := errors.New("close failed")
	client := &retryCloseClient{
		Client:   storagetest.NewInMemoryClient(component.KindProcessor, component.MustNewID("drain"), ""),
		closeErr: closeErr,
	}
	p := newManualProcessor(t, createDefaultConfig().(*Config))
	p.storageClient = client
	require.ErrorIs(t, p.Shutdown(t.Context()), closeErr)
	require.NoError(t, p.Shutdown(t.Context()))
	assert.Equal(t, 2, client.closeCalls)
	require.NoError(t, p.Shutdown(t.Context()))
	assert.Equal(t, 2, client.closeCalls)
}

func TestShutdownBeforeFirstTick(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Storage = storageID()
	cfg.SaveInterval = time.Hour
	p := newManualProcessor(t, cfg)
	require.NoError(t, p.Start(t.Context(), fileBackedStorageHost(t)))
	require.NoError(t, p.Shutdown(t.Context()))
	require.NoError(t, p.Shutdown(t.Context()))
}
