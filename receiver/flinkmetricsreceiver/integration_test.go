// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package flinkmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const (
	networkName = "flink-network"
	flinkPort   = "8081"
)

func TestIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithNetworkRequest(
			testcontainers.NetworkRequest{
				Name:           networkName,
				CheckDuplicate: true,
			},
		),
		scraperinttest.WithContainerRequest(testcontainers.ContainerRequest{
			Image:        "flink:1.17.0",
			Name:         "jobmanager",
			Networks:     []string{networkName},
			ExposedPorts: []string{flinkPort},
			Cmd:          []string{"jobmanager"},
			Env: map[string]string{
				"FLINK_PROPERTIES": "jobmanager.rpc.address: jobmanager",
			},
			Files: []testcontainers.ContainerFile{{
				HostFilePath:      filepath.Join("testdata", "integration", "setup.sh"),
				ContainerFilePath: "/setup.sh",
				FileMode:          700,
			}},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort(flinkPort),
				wait.ForHTTP("jobmanager/metrics").WithPort(flinkPort),
				wait.ForHTTP("taskmanagers/metrics").WithPort(flinkPort),
			),
			LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
				PostStarts: []testcontainers.ContainerHook{
					scraperinttest.RunScript([]string{"/setup.sh"}),
				},
			}},
		}),
		scraperinttest.WithContainerRequest(testcontainers.ContainerRequest{
			Image:    "flink:1.17.0",
			Name:     "taskmanager",
			Networks: []string{networkName},
			Cmd:      []string{"taskmanager"},
			Env: map[string]string{
				"FLINK_PROPERTIES": "jobmanager.rpc.address: jobmanager",
			},
		}),
		scraperinttest.WithCustomConfig(func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
			rCfg := cfg.(*Config)
			rCfg.CollectionInterval = 100 * time.Millisecond
			rCfg.Endpoint = fmt.Sprintf("http://%s",
				net.JoinHostPort(
					ci.HostForNamedContainer(t, "jobmanager"),
					ci.MappedPortForNamedContainer(t, "jobmanager", flinkPort),
				))
		}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceAttributeValue("host.name"),
			pmetrictest.IgnoreResourceAttributeValue("flink.task.name"),
			pmetrictest.IgnoreResourceAttributeValue("flink.taskmanager.id"),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}
