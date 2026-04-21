// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package redisreceiver

import (
	"fmt"
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
	redisPort    = "6379"
	sentinelPort = "26379"
)

func TestIntegrationV6(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image:        "redis:6.0.3",
				ExposedPorts: []string{redisPort},
				WaitingFor:   wait.ForListeningPort(redisPort),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Endpoint = fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, redisPort))
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.ChangeResourceAttributeValue("server.address", func(_ string) string {
				return "localhost"
			}),
			pmetrictest.ChangeResourceAttributeValue("server.port", func(_ string) string {
				return redisPort
			}),
		),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", "expected-old.yaml")),
	).Run(t)
}

func TestIntegrationV7Cluster(t *testing.T) {
	t.Skip("Skipping due to flakiness, possibly related to https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30411")
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(testcontainers.ContainerRequest{
			ExposedPorts: []string{
				redisPort,
				"6380",
				"6381",
				"6382",
				"6383",
				"6384",
				"6385",
			},
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration"),
				Dockerfile: "Dockerfile.cluster",
			},
			WaitingFor: wait.ForListeningPort("6385").WithStartupTimeout(30 * time.Second),
		}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				// Strictly speaking this is non-deterministic and may not be the right port for one with repl offset
				// However, we're using socat and some port forwarding in the Dockerfile to ensure this always points
				// to a replica node, so in practice any failures due to cluster node role changes is unlikely
				rCfg.Endpoint = fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, "6385"))
				rCfg.MetricsBuilderConfig.Metrics.RedisReplicationReplicaOffset.Enabled = true
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", "expected-cluster.yaml")),
		scraperinttest.WithCreateContainerTimeout(time.Minute),
		scraperinttest.WithCompareTimeout(time.Minute),
	).Run(t)
}

func TestIntegrationV8Sentinel(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image:        "redis:8.2.1",
				ExposedPorts: []string{sentinelPort},
				Cmd: []string{
					"redis-sentinel",
					"/etc/redis/sentinel.conf",
				},
				Files: []testcontainers.ContainerFile{
					{
						HostFilePath:      filepath.Join("testdata", "integration", "redis-sentinel.conf"),
						ContainerFilePath: "/etc/redis/sentinel.conf",
						FileMode:          0o644,
					},
				},
				WaitingFor: wait.ForListeningPort(sentinelPort),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Endpoint = fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, sentinelPort))
				rCfg.MetricsBuilderConfig.Metrics.RedisMode.Enabled = true
				rCfg.MetricsBuilderConfig.Metrics.RedisSentinelMasters.Enabled = true
				rCfg.MetricsBuilderConfig.Metrics.RedisSentinelRunningScripts.Enabled = true
				rCfg.MetricsBuilderConfig.Metrics.RedisSentinelScriptsQueueLength.Enabled = true
				rCfg.MetricsBuilderConfig.Metrics.RedisSentinelSimulateFailureFlags.Enabled = true
				rCfg.MetricsBuilderConfig.Metrics.RedisSentinelTiltSinceSeconds.Enabled = true
				rCfg.MetricsBuilderConfig.Metrics.RedisSentinelTotalTilt.Enabled = true
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", "expected-sentinel.yaml")),
		scraperinttest.WithCreateContainerTimeout(time.Minute),
	).Run(t)
}
