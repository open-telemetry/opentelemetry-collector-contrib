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

//go:build integration
// +build integration

package container

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerIntegration(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("failed creating docker client from environment: %v", err)
	}
	defer cli.Close()

	// Remove image so more code paths are hit. (Don't need to prune children layers though to keep test faster.)
	_, _ = cli.ImageRemove(context.Background(), "docker.io/library/nginx:1.19", types.ImageRemoveOptions{
		Force:         true,
		PruneChildren: false,
	})

	con := New(t)
	started := con.StartImage("docker.io/library/nginx:1.19", WithPortReady(80))

	require.NotEmpty(t, started.AddrForPort(80), "IP address was empty")
	require.NotEmpty(t, started.ID)

	time.Sleep(5 * time.Second)
	resp, err := http.Get("http://" + started.AddrForPort(80))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, resp.StatusCode, http.StatusOK)

	con.Cleanup()

	assert.Zero(t, len(con.runningContainers), "started containers should be empty after cleanup")

	_, err = cli.ContainerInspect(context.Background(), string(started.ID))
	require.Error(t, err, "inspect should have returned an error")
	require.True(t, client.IsErrNotFound(err), "inspect error should have been of type not found")
}

func TestRemoveContainerIntegration(t *testing.T) {
	con := New(t)
	nginx := con.StartImage("docker.io/library/nginx:1.19", WithPortReady(80))
	require.Equal(t, 1, len(con.runningContainers))

	err := con.RemoveContainer(nginx)
	require.NoError(t, err)
	require.Zero(t, len(con.runningContainers))

	err = con.RemoveContainer(nginx)
	require.Error(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("failed removing container %v", nginx.ID))
}
