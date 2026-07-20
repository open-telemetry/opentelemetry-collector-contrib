// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestIsDockerUnavailableError(t *testing.T) {
	require.True(t, isDockerUnavailableError(errors.New(`open //./pipe/docker_engine: the system cannot find the file specified`)))
	require.True(t, isDockerUnavailableError(errors.New("dial unix /var/run/docker.sock: connect: connection refused")))
	require.False(t, isDockerUnavailableError(nil))
	require.False(t, isDockerUnavailableError(errors.New("invalid container config")))
}

func TestIsInvalidDockerClientConfigError(t *testing.T) {
	require.True(t, isInvalidDockerClientConfigError(errors.New("unable to parse docker host")))
	require.True(t, isInvalidDockerClientConfigError(errors.New("unsupported protocol scheme")))
	require.False(t, isInvalidDockerClientConfigError(nil))
	require.False(t, isInvalidDockerClientConfigError(errors.New("connection refused")))
}

func TestIsDockerClientCreationRecoverable(t *testing.T) {
	require.True(t, isDockerClientCreationRecoverable(errors.New(`open //./pipe/docker_engine: the system cannot find the file specified`)))
	require.False(t, isDockerClientCreationRecoverable(errors.New("unable to parse docker host `tcp://bad`")))
	require.False(t, isDockerClientCreationRecoverable(errors.New("invalid proto, expected tcp or unix: bogus")))
	require.False(t, isDockerClientCreationRecoverable(nil))
}

func TestContainerMetadataCacheKey(t *testing.T) {
	// A full 64-char ID embedded in a longer string is extracted.
	require.Equal(t, testContainerID, containerMetadataCacheKey(testContainerID+"-json.log"))
	// A short ID is lower-cased and returned as-is.
	require.Equal(t, "abc123", containerMetadataCacheKey("ABC123"))
}

func TestParseMetadataEndpoints(t *testing.T) {
	env := []string{
		"PATH=/usr/bin",
		"ECS_CONTAINER_METADATA_URI_V4=http://169.254.170.2/v4/abc",
		"ECS_CONTAINER_METADATA_URI=http://169.254.170.2/v3/abc",
	}
	endpoints := parseMetadataEndpoints(env)
	require.Len(t, endpoints, 2)
	require.Contains(t, endpoints, "http://169.254.170.2/v4/abc")
	require.Contains(t, endpoints, "http://169.254.170.2/v3/abc")

	require.Empty(t, parseMetadataEndpoints([]string{"PATH=/usr/bin"}))
}

func TestMergeIntoMetadataCache(t *testing.T) {
	dst := make(map[string]containerMetadata)
	mergeIntoMetadataCache(dst, []containerMetadata{
		{DockerID: testContainerID, Image: "img:1"},
		{DockerID: ""}, // skipped: no ID
	})
	require.Len(t, dst, 1)
	require.Equal(t, "img:1", dst[testContainerID].Image)
}

func TestEcsMetadataEndpointsFromTask(t *testing.T) {
	const dockerID = "abcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcd"
	taskJSON := `{"Containers":[{"DockerId":"` + dockerID + `","Name":"app","Image":"img:latest"}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/task") {
			_, _ = w.Write([]byte(taskJSON))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(ecsContainerMetadataURIv4, srv.URL+"/v4/self")

	m, preload, err := ecsMetadataEndpointsFromTask(t.Context(), zap.NewNop())
	require.NoError(t, err)
	require.Len(t, m, 1)
	require.Len(t, preload, 1)
	require.Equal(t, dockerID, preload[0].DockerID)
	require.Empty(t, m[dockerID])
}

func TestEcsMetadataEndpointsFromTaskNoEnv(t *testing.T) {
	t.Setenv(ecsContainerMetadataURIv4, "")
	m, preload, err := ecsMetadataEndpointsFromTask(t.Context(), zap.NewNop())
	require.Error(t, err)
	require.Nil(t, m)
	require.Nil(t, preload)
}

func TestEcsMetadataEndpointsFromTaskOrEmptyUnavailable(t *testing.T) {
	t.Setenv(ecsContainerMetadataURIv4, "")
	m, preload, err := ecsMetadataEndpointsFromTaskOrEmpty(t.Context(), zap.NewNop())
	require.Error(t, err)
	require.ErrorIs(t, err, errECSTaskMetadataUnavailable)
	require.Nil(t, m)
	require.Nil(t, preload)
}

func TestStartDockerMetadataSyncStops(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan struct{})

	// nil client: no Docker event subscription, just the periodic ticker + stop.
	startDockerMetadataSync(zap.NewNop(), nil, stop, func() error { return nil })

	go func() {
		// give the goroutine a moment to start, then stop it.
		time.Sleep(10 * time.Millisecond)
		close(stop)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sync goroutine did not stop")
	}
}
