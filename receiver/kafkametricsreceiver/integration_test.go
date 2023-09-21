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
				rCfg.Metrics.KafkaBrokers.Enabled = false
				rCfg.Metrics.KafkaConsumerGroupLag.Enabled = true
				rCfg.Metrics.KafkaConsumerGroupLagSum.Enabled = true
				rCfg.Metrics.KafkaConsumerGroupMembers.Enabled = true
				rCfg.Metrics.KafkaConsumerGroupOffset.Enabled = true
				rCfg.Metrics.KafkaConsumerGroupOffsetSum.Enabled = true
				rCfg.Metrics.KafkaPartitionCurrentOffset.Enabled = true
				rCfg.Metrics.KafkaPartitionOldestOffset.Enabled = true
				rCfg.Metrics.KafkaPartitionReplicas.Enabled = true
				rCfg.Metrics.KafkaPartitionReplicasInSync.Enabled = true
				rCfg.Metrics.KafkaTopicPartitions.Enabled = true
				rCfg.Metrics.MessagingKafkaBrokerConsumerFetchCount.Enabled = true
				rCfg.Metrics.MessagingKafkaBrokerConsumerFetchRate.Enabled = true
				rCfg.Metrics.MessagingKafkaBrokerCount.Enabled = true
				rCfg.Metrics.MessagingKafkaBrokerIncomingByteRate.Enabled = true
				rCfg.Metrics.MessagingKafkaBrokerOutgoingByteRate.Enabled = true
				rCfg.Metrics.MessagingKafkaBrokerRequestLatency.Enabled = true
				rCfg.Metrics.MessagingKafkaBrokerRequestRate.Enabled = true
				rCfg.Metrics.MessagingKafkaBrokerRequestSize.Enabled = true
				rCfg.Metrics.MessagingKafkaBrokerRequestsInFlight.Enabled = true
				rCfg.Metrics.MessagingKafkaBrokerResponseRate.Enabled = true
				rCfg.Metrics.MessagingKafkaBrokerResponseSize.Enabled = true
			}),

		// scraperinttest.WriteExpected(), // TODO remove
		scraperinttest.WithCompareOptions(
			// pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
		scraperinttest.WriteExpected(),
	).Run(t)
}
