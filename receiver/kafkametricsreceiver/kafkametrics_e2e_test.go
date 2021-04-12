// Copyright  OpenTelemetry Authors
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

// +build integration

package kafkametricsreceiver

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testing/container"
)

const (
	zkPort       = 2181
	kafkaPort    = 9092
	kafkaZkImage = "johnnypark/kafka-zookeeper"
	//only one metric, number of brokers, will be reported.
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

func TestIntegrationSingleNode(t *testing.T) {
	docker := container.New(t)
	container := docker.StartImage(kafkaZkImage, container.WithPortReady(kafkaPort), container.WithPortReady(zkPort))
	kafkaAddress := container.AddrForPort(kafkaPort)
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Scrapers = []string{
		"consumers",
		"brokers",
		"topics",
	}
	cfg.Brokers = []string{kafkaAddress}
	cfg.CollectionInterval = 5 * time.Second
	consumer := new(consumertest.MetricsSink)
	params := component.ReceiverCreateParams{Logger: zaptest.NewLogger(t)}

	var receiver component.MetricsReceiver
	var err error
	t.Logf("waiting to connect to kafka...")
	require.Eventuallyf(t,
		func() bool {
			receiver, err = f.CreateMetricsReceiver(context.Background(), params, cfg, consumer)
			return err == nil
		}, 30*time.Second, 5*time.Second,
		fmt.Sprintf("failed to create metrics receiver. %v", err),
	)
	t.Logf("connected to kafka")
	require.NoError(t, receiver.Start(context.Background(), &testHost{t: t}))
	t.Logf("waiting for metrics...")
	require.Eventuallyf(t,
		func() bool {
			return consumer.MetricsCount() >= expectedMetrics
		}, 30*time.Second, 5*time.Second,
		"expected metrics not received",
	)
}
