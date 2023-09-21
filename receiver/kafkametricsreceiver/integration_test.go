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
					"messaging.kafka.broker.count",
					"messaging.kafka.broker.consumer_fetch_rate",
					"messaging.kafka.broker.incoming_byte_rate",
					"messaging.kafka.broker.outgoing_byte_rate",
					"messaging.kafka.broker.request_latency",
					"messaging.kafka.broker.response_rate",
					"messaging.kafka.broker.response_size",
					"messaging.kafka.broker.request_rate",
					"messaging.kafka.broker.request_size",
					"messaging.kafka.broker.requests_in_flight",
					"messaging.kafka.broker.consumer_fetch_count",
					"kafka.topic.partitions",
					"kafka.partition.current_offset",
					"kafka.partition.oldest_offset",
					"kafka.partition.replicas",
					"kafka.partition.replicas_in_sync",
					"kafka.consumer_group.members",
					"kafka.consumer_group.offset",
					"kafka.consumer_group.offset_sum",
					"kafka.consumer_group.lag",
					"kafka.consumer_group.lag_sum",
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
