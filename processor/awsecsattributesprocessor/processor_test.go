// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

func defaultTestConfig() *Config {
	return &Config{
		CacheTTL:    60,
		ContainerID: ContainerID{Sources: []string{"container.id"}},
	}
}

func TestCapabilities(t *testing.T) {
	c := newTestCore(t, defaultTestConfig(), staticEndpoints("http://unused"))
	require.Equal(t, consumer.Capabilities{MutatesData: true}, c.Capabilities())
}

func TestSyncMetadataAndGet(t *testing.T) {
	srv := newMetadataServer(t)
	c := newTestCore(t, defaultTestConfig(), staticEndpoints(srv.URL))

	md, err := c.get(t.Context(), testContainerID)
	require.NoError(t, err)
	require.Equal(t, expectedFlattenedMetadata, md.flat())

	// A second get is served from cache.
	md2, err := c.get(t.Context(), testContainerID)
	require.NoError(t, err)
	require.Equal(t, md.DockerID, md2.DockerID)
}

func TestGetNotFoundAfterSync(t *testing.T) {
	srv := newMetadataServer(t)
	c := newTestCore(t, defaultTestConfig(), staticEndpoints(srv.URL))

	_, err := c.get(t.Context(), "does-not-exist")
	require.ErrorContains(t, err, "metadata not found")
}

func TestRunsyncPreservesCacheOnEcsUnavailable(t *testing.T) {
	// Seed the cache, then run a sync whose endpoints fn reports the recoverable
	// ECS-unavailable error; the cache must be preserved and no error returned.
	c := newTestCore(t, defaultTestConfig(), func(context.Context, *zap.Logger) (map[string][]string, []containerMetadata, error) {
		return nil, nil, errECSTaskMetadataUnavailable
	})
	c.metadata[testContainerID] = containerMetadata{DockerID: testContainerID}

	require.NoError(t, c.runsync(t.Context()))
	require.Len(t, c.metadata, 1)
}

func TestRunsyncEndpointError(t *testing.T) {
	c := newTestCore(t, defaultTestConfig(), func(context.Context, *zap.Logger) (map[string][]string, []containerMetadata, error) {
		return nil, nil, errBoom
	})
	require.ErrorContains(t, c.runsync(t.Context()), "failed to fetch metadata endpoints")
}

func TestRunsyncPreload(t *testing.T) {
	// Endpoints with an empty URL list rely on the preload slice for metadata.
	c := newTestCore(t, defaultTestConfig(), func(context.Context, *zap.Logger) (map[string][]string, []containerMetadata, error) {
		return map[string][]string{testContainerID: {}}, []containerMetadata{{DockerID: testContainerID, Image: "img:preload"}}, nil
	})
	require.NoError(t, c.runsync(t.Context()))
	require.Equal(t, "img:preload", c.metadata[testContainerID].Image)
}

func TestSyncMetadataEvictsStale(t *testing.T) {
	srv := newMetadataServer(t)
	c := newTestCore(t, defaultTestConfig(), staticEndpoints(srv.URL))

	// Seed a stale entry and push the eviction clock past the TTL.
	c.metadata["stale"] = containerMetadata{DockerID: "stale"}
	c.metadataAge = time.Now().Add(-2 * time.Minute)

	require.NoError(t, c.syncMetadata(t.Context(), map[string][]string{testContainerID: {srv.URL}}))
	require.Contains(t, c.metadata, testContainerID)
	require.NotContains(t, c.metadata, "stale")
}

func TestSyncMetadataFetchError(t *testing.T) {
	c := newTestCore(t, defaultTestConfig(), staticEndpoints("http://unused"))
	// An unreachable endpoint is logged and skipped, not fatal.
	require.NoError(t, c.syncMetadata(t.Context(), map[string][]string{"x": {"http://127.0.0.1:0/nope"}}))
	require.NotContains(t, c.metadata, "x")
}

func TestFetchMetadata(t *testing.T) {
	srv := newMetadataServer(t)
	md, err := fetchMetadata(t.Context(), srv.URL)
	require.NoError(t, err)
	require.Equal(t, "196a0e6abfce1e33ee24b65e97875f089878dd7d1d7e9f15155d6094c8b908f5", md.DockerID)

	_, err = fetchMetadata(t.Context(), "http://127.0.0.1:0/nope")
	require.Error(t, err)
}

func TestContainerIDFromAttrs(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("log.file.name", testContainerID+"-json.log")
	attrs.PutStr("container.id", testContainerID)

	// First matching source wins; ID is normalized.
	require.Equal(t, testContainerID, containerIDFromAttrs(attrs, "container.id"))
	require.Equal(t, testContainerID, containerIDFromAttrs(attrs, "log.file.name"))
	// No matching source yields an empty ID.
	require.Empty(t, containerIDFromAttrs(attrs, "missing"))
}

func TestStartShutdownLifecycle(t *testing.T) {
	srv := newMetadataServer(t)
	c := newTestCore(t, defaultTestConfig(), staticEndpoints(srv.URL))

	require.NoError(t, c.Start(t.Context(), nil))
	require.Equal(t, "196a0e6abfce1e33ee24b65e97875f089878dd7d1d7e9f15155d6094c8b908f5", c.metadata[testContainerID].DockerID)
	require.NoError(t, c.Shutdown(t.Context()))
}

func TestStartFatalDockerClientError(t *testing.T) {
	c := newTestCore(t, defaultTestConfig(), staticEndpoints("http://unused"))
	c.newDockerClient = nonRecoverableDockerClient
	require.ErrorContains(t, c.Start(t.Context(), nil), "failed to initialize Docker API client")
}
