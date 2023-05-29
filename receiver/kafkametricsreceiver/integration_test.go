// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package kafkametricsreceiver

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

const (
	zkPort       = 2181
	kafkaPort    = 9092
	kafkaZkImage = "johnnypark/kafka-zookeeper"
	// only one metric, number of brokers, will be reported.
	expectedMetrics = 1
)

type testHost struct {
	component.Host
	t *testing.T
}

func (h *testHost) ReportFatalError(err error) {
	h.t.Fatalf("receiver reported a fatal error: %v", err)
}

var _ component.Host = (*testHost)(nil)

func TestIntegration(t *testing.T) {
	t.Skip("Skip failing test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/17065")
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image: kafkaZkImage,
		ExposedPorts: []string{
			fmt.Sprintf("%d/tcp", kafkaPort),
			fmt.Sprintf("%d/tcp", zkPort),
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("2181/tcp").WithStartupTimeout(time.Minute*2),
			wait.ForListeningPort("9092/tcp").WithStartupTimeout(time.Minute*2),
		).WithDeadline(time.Minute * 2),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.Nil(t, err)

	mappedKafkaPort, err := container.MappedPort(ctx, "9092")
	require.Nil(t, err)

	hostIP, err := container.Host(ctx)
	require.Nil(t, err)

	kafkaAddress := fmt.Sprintf("%s:%s", hostIP, mappedKafkaPort.Port())

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Scrapers = []string{
		"brokers",
		"consumers",
		"topics",
	}
	cfg.Brokers = []string{kafkaAddress}
	cfg.CollectionInterval = 5 * time.Second
	consumer := new(consumertest.MetricsSink)

	var receiver receiver.Metrics

	receiver, err = f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumer)
	require.NoError(t, err, "failed to create receiver")
	require.Eventuallyf(t, func() bool {
		err = receiver.Start(context.Background(), &testHost{t: t})
		return err == nil
	}, 30*time.Second, 5*time.Second, fmt.Sprintf("failed to start metrics receiver. %v", err))
	t.Logf("waiting for metrics...")
	require.Eventuallyf(t,
		func() bool {
			return consumer.DataPointCount() >= expectedMetrics
		}, 30*time.Second, 5*time.Second,
		"expected metrics not received",
	)
}
