// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// testContainerID is a full-length (64 hex char) Docker container ID.
const testContainerID = "9e160da5ce00c77637967534f4390f39d9d78137e72666dc3cc52eabf211bdae"

// payload is a sample ECS container metadata document.
const payload = `{
	"ContainerARN": "arn:aws:ecs:eu-west-1:035955823396:container/cds-305/ec7ff82b7a3a44a5bbbe9bcf11daee33/cc1c133f-bd1f-4006-8dae-4cd8a3f54f19",
	"CreatedAt": "2023-06-22T12:41:18.335883278Z",
	"DesiredStatus": "RUNNING",
	"DockerId": "196a0e6abfce1e33ee24b65e97875f089878dd7d1d7e9f15155d6094c8b908f5",
	"DockerName": "ecs-cadvisor-task-definition-7-cadvisor-bae592b5e4c1a3bb3800",
	"Image": "gcr.io/cadvisor/cadvisor:latest",
	"ImageID": "sha256:68c29634fe49724f94ed34f18224336f776392f7a5a4014969ac5798a2ec96dc",
	"KnownStatus": "RUNNING",
	"Labels": {
	  "com.custom.app": "go-test-app",
	  "com.amazonaws.ecs.cluster": "cds-305",
	  "com.amazonaws.ecs.container-name": "cadvisor",
	  "com.amazonaws.ecs.task-arn": "arn:aws:ecs:eu-west-1:035955823396:task/cds-305/ec7ff82b7a3a44a5bbbe9bcf11daee33",
	  "com.amazonaws.ecs.task-definition-family": "cadvisor-task-definition",
	  "com.amazonaws.ecs.task-definition-version": "7"
	},
	"Limits": {
	  "CPU": 10,
	  "Memory": 300
	},
	"Name": "cadvisor",
	"Networks": [
	  {
		"IPv4Addresses": [
		  "172.17.0.2"
		],
		"NetworkMode": "bridge"
	  }
	],
	"Ports": [
	  {
		"ContainerPort": 8080,
		"HostIp": "0.0.0.0",
		"HostPort": 32911,
		"Protocol": "tcp"
	  },
	  {
		"ContainerPort": 8080,
		"HostIp": "::",
		"HostPort": 32911,
		"Protocol": "tcp"
	  }
	],
	"StartedAt": "2023-06-22T12:41:18.713571182Z",
	"Type": "NORMAL",
	"Volumes": [
	  {
		"Destination": "/var",
		"Source": "/var"
	  },
	  {
		"Destination": "/etc",
		"Source": "/etc"
	  }
	]
  }
`

// expectedFlattenedMetadata is the result of Flat() applied to payload.
var expectedFlattenedMetadata = map[string]any{
	"aws.ecs.cluster":                 "cds-305",
	"aws.ecs.container.arn":           "arn:aws:ecs:eu-west-1:035955823396:container/cds-305/ec7ff82b7a3a44a5bbbe9bcf11daee33/cc1c133f-bd1f-4006-8dae-4cd8a3f54f19",
	"aws.ecs.container.name":          "cadvisor",
	"aws.ecs.task.arn":                "arn:aws:ecs:eu-west-1:035955823396:task/cds-305/ec7ff82b7a3a44a5bbbe9bcf11daee33",
	"aws.ecs.task.definition.family":  "cadvisor-task-definition",
	"aws.ecs.task.definition.version": "7",
	"aws.ecs.task.known.status":       "RUNNING",
	"created.at":                      "2023-06-22T12:41:18.335883278Z",
	"desired.status":                  "RUNNING",
	"docker.id":                       "196a0e6abfce1e33ee24b65e97875f089878dd7d1d7e9f15155d6094c8b908f5",
	"docker.name":                     "ecs-cadvisor-task-definition-7-cadvisor-bae592b5e4c1a3bb3800",
	"image":                           "gcr.io/cadvisor/cadvisor:latest",
	"image.id":                        "sha256:68c29634fe49724f94ed34f18224336f776392f7a5a4014969ac5798a2ec96dc",
	"limits.cpu":                      10,
	"limits.memory":                   300,
	"name":                            "cadvisor",
	"networks.0.ipv4.addresses.0":     "172.17.0.2",
	"networks.0.network.mode":         "bridge",
	"ports.0.container.port":          8080,
	"ports.0.host.ip":                 "0.0.0.0",
	"ports.0.host.port":               32911,
	"ports.0.protocol":                "tcp",
	"ports.1.container.port":          8080,
	"ports.1.host.ip":                 "::",
	"ports.1.host.port":               32911,
	"ports.1.protocol":                "tcp",
	"started.at":                      "2023-06-22T12:41:18.713571182Z",
	"type":                            "NORMAL",
	"volumes.0.destination":           "/var",
	"volumes.0.source":                "/var",
	"volumes.1.destination":           "/etc",
	"volumes.1.source":                "/etc",
	"labels.com.custom.app":           "go-test-app",
}

// newMetadataServer returns an httptest server that serves the metadata payload,
// registered for cleanup on the test.
func newMetadataServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// payload is a constant, valid JSON document; errors are not expected and
		// must not call require from within the handler goroutine.
		var m map[string]any
		_ = json.Unmarshal([]byte(payload), &m)
		_ = json.NewEncoder(w).Encode(m)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// staticEndpoints returns an endpointsFn that always reports a single container
// served from url.
func staticEndpoints(url string) endpointsFn {
	return func(context.Context, *zap.Logger) (map[string][]string, []containerMetadata, error) {
		return map[string][]string{testContainerID: {url}}, nil, nil
	}
}

// errBoom is a generic non-recoverable error for tests.
var errBoom = errors.New("boom")

// recoverableDockerClient is a newDockerClient replacement that simulates an
// unreachable daemon, so Start takes the no-Docker path without a real socket.
func recoverableDockerClient() (*client.Client, error) {
	return nil, errors.New("dial unix /var/run/docker.sock: connect: connection refused")
}

// nonRecoverableDockerClient simulates a misconfigured Docker client, which Start
// must treat as a fatal error.
func nonRecoverableDockerClient() (*client.Client, error) {
	return nil, errors.New("unable to parse docker host")
}

// zaptestLogger returns a test-scoped zap logger.
func zaptestLogger(t *testing.T) *zap.Logger {
	t.Helper()
	return zaptest.NewLogger(t)
}

// newTestCore builds an ecsCore wired to the metadata server and a stubbed Docker
// client, with an initialized config.
func newTestCore(t *testing.T, cfg *Config, endpoints endpointsFn) *ecsCore {
	t.Helper()
	require.NoError(t, cfg.init())
	c := newCore(zaptestLogger(t), cfg, endpoints)
	c.newDockerClient = recoverableDockerClient
	return c
}
