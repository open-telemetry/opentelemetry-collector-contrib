// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package redisreceiver

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const redisPort = "6379"

func TestOlderIntegration(t *testing.T) {
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
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.ChangeResourceAttributeValue("server.address", func(_ string) string {
				return "localhost"
			}),
			pmetrictest.ChangeResourceAttributeValue("server.port", func(_ string) string {
				return redisPort
			}),
		),
	).Run(t)
}
func TestLatestIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image:        "redis:9.0.3",
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
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.ChangeResourceAttributeValue("server.address", func(_ string) string {
				return "localhost"
			}),
			pmetrictest.ChangeResourceAttributeValue("server.port", func(_ string) string {
				return redisPort
			}),
		),
	).Run(t)
}
func TestClusterIntegration(t *testing.T) {
	networkName := "redis-net"
	startingPort := 6379
	var waits []wait.Strategy

	// Needs at least 6 containers to form a redis cluster with replication
	var containerRequests []scraperinttest.TestOption
	for i := 0; i < 6; i++ {
		containerWait := wait.ForListeningPort(nat.Port(strconv.Itoa(startingPort + i)))
		waits = append(waits, containerWait)
		containerRequests = append(containerRequests, scraperinttest.WithContainerRequest(testcontainers.ContainerRequest{
			Image:        "redis:9.0.3",
			Name:         fmt.Sprintf("redis-node%d", startingPort+i),
			ExposedPorts: []string{strconv.Itoa(startingPort + i)},
			WaitingFor:   containerWait,
			Networks:     []string{"redisinteg"},
		}))
	}

	clusterCmd := []string{
		"/bin/bash -c 'yes | redis-cli --cluster create redis-node1:6379 redis-node2:6379 redis-node3:6379 redis-node4:6379 redis-node5:6379 redis-node6:6379 --cluster-replicas 1'",
	}
	// also needs to run a command, but can be done on existing container.
	clusterCreateRequest := scraperinttest.WithContainerRequest(testcontainers.ContainerRequest{
		Image:      "redis:9.0.3",
		WaitingFor: wait.ForAll(waits...),
		LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
			PostStarts: []testcontainers.ContainerHook{
				scraperinttest.RunScript(clusterCmd),
			},
		}},
	})

	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithNetworkRequest(testcontainers.NetworkRequest{Name: networkName}),
		containerRequests[0],
		containerRequests[1],
		containerRequests[2],
		containerRequests[3],
		containerRequests[4],
		containerRequests[5],
		clusterCreateRequest,
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Endpoint = fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, redisPort))
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.ChangeResourceAttributeValue("server.address", func(_ string) string {
				return "localhost"
			}),
			pmetrictest.ChangeResourceAttributeValue("server.port", func(_ string) string {
				return redisPort
			}),
		),
	).Run(t)
}
