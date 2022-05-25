// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elasticsearchreceiver

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"path/filepath"
	"testing"
	"time"
)

var (
	containerRequest7_9 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.elasticsearch",
		},
		ExposedPorts: []string{"9200:9200"},
		WaitingFor: wait.ForListeningPort("9200").
			WithStartupTimeout(2 * time.Minute),
	}
)

func TestElasticsearchIntegration(t *testing.T) {
	//Starts an elasticsearch docker container
	t.Run("Running elasticsearch 7.9", func(t *testing.T) {
		t.Parallel()
		container := getContainer(t, containerRequest7_9)
		//todo
		fmt.Println(container)
	})
}

func getContainer(t *testing.T, req testcontainers.ContainerRequest) testcontainers.Container {
	require.NoError(t, req.Validate())
	container, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)

	// no cmd
	code, err := container.Exec(context.Background(), []string{"eswrapper"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	return container
}
