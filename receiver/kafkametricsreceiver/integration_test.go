// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package kafkametricsreceiver

import (
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const (
	networkName   = "kafka-network"
	kafkaPort     = "9092"
	zookeeperPort = "2181"
	zookeeperHost = "zookeeper"
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
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Name:         "zookeeper",
				Image:        "ubuntu/zookeeper:3.1-22.04_beta",
				Networks:     []string{networkName},
				Hostname:     zookeeperHost,
				ExposedPorts: []string{zookeeperPort},
				WaitingFor: wait.ForAll(
					wait.ForListeningPort(zookeeperPort).WithStartupTimeout(2 * time.Minute),
				),
			}),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Name:         "kafka",
				Image:        "ubuntu/kafka:3.1-22.04_beta",
				Networks:     []string{networkName},
				ExposedPorts: []string{kafkaPort},
				Env: map[string]string{
					"ZOOKEEPER_HOST": zookeeperHost,
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
					ci.HostForNamedContainer(t, "kafka"),
					ci.MappedPortForNamedContainer(t, "kafka", kafkaPort))}
				rCfg.Scrapers = []string{
					"brokers",
					"consumers",
					"topics",
				}
				rCfg.Metrics = map[string]bool{
					"messaging.kafka.broker.count":                true,
					"messaging.kafka.broker.consumer_fetch_rate":  true,
					"messaging.kafka.broker.incoming_byte_rate":   true,
					"messaging.kafka.broker.outgoing_byte_rate":   true,
					"messaging.kafka.broker.request_latency":      true,
					"messaging.kafka.broker.response_rate":        true,
					"messaging.kafka.broker.response_size":        true,
					"messaging.kafka.broker.request_rate":         true,
					"messaging.kafka.broker.request_size":         true,
					"messaging.kafka.broker.requests_in_flight":   true,
					"messaging.kafka.broker.consumer_fetch_count": true,
				}
			}),

		//  scraperinttest.WriteExpected(), // TODO remove
		scraperinttest.WithCompareOptions(
			// pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}
