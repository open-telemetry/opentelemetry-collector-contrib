// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package kafkametricsreceiver

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const (
	kafkaPort     = "9092"
	zookeeperPort = "2181"
)

func TestIntegration(t *testing.T) {
	uid := fmt.Sprintf("-%s", uuid.NewString())
	networkName := "kafka-network" + uid
	zkContainerName := "zookeeper" + uid
	kafkaContainerName := "kafka" + uid

	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithNetworkRequest(
			testcontainers.NetworkRequest{
				Name:           networkName,
				CheckDuplicate: true,
			},
		),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Name:         zkContainerName,
				Image:        "ubuntu/zookeeper:3.1-22.04_beta",
				Networks:     []string{networkName},
				Hostname:     zkContainerName,
				ExposedPorts: []string{zookeeperPort},
				WaitingFor: wait.ForAll(
					wait.ForListeningPort(zookeeperPort).WithStartupTimeout(2 * time.Minute),
				),
			}),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Name:         kafkaContainerName,
				Image:        "ubuntu/kafka:3.1-22.04_beta",
				Networks:     []string{networkName},
				ExposedPorts: []string{kafkaPort},
				Env: map[string]string{
					"ZOOKEEPER_HOST": zkContainerName,
					"ZOOKEEPER_PORT": zookeeperPort,
				},
				WaitingFor: wait.ForAll(
					wait.ForListeningPort(kafkaPort).WithStartupTimeout(2 * time.Minute),
				),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = 5 * time.Second
				rCfg.Brokers = []string{fmt.Sprintf("%s:%s",
					ci.HostForNamedContainer(t, kafkaContainerName),
					ci.MappedPortForNamedContainer(t, kafkaContainerName, kafkaPort))}
				rCfg.Scrapers = []string{"brokers", "consumers", "topics"}
			}),
		// scraperinttest.WriteExpected(), // TODO remove
		scraperinttest.WithCompareOptions(
			// pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}
