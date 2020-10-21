// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcloudpubsubreceiver

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	pb "google.golang.org/genproto/googleapis/pubsub/v1"
)

func TestStartReceiver(t *testing.T) {
	ctx := context.Background()
	// Start a fake server running locally.
	srv := pstest.NewServer()
	defer srv.Close()
	_, err := srv.GServer.CreateTopic(ctx, &pb.Topic{
		Name: "projects/my-project/topics/otlp",
	})
	assert.NoError(t, err)
	_, err = srv.GServer.CreateSubscription(ctx, &pb.Subscription{
		Topic:              "projects/my-project/topics/otlp",
		Name:               "projects/my-project/subscriptions/my-trace",
		AckDeadlineSeconds: 10,
	})
	assert.NoError(t, err)
	_, err = srv.GServer.CreateSubscription(ctx, &pb.Subscription{
		Topic:              "projects/my-project/topics/otlp",
		Name:               "projects/my-project/subscriptions/my-metric",
		AckDeadlineSeconds: 10,
	})
	assert.NoError(t, err)
	_, err = srv.GServer.CreateSubscription(ctx, &pb.Subscription{
		Topic:              "projects/my-project/topics/otlp",
		Name:               "projects/my-project/subscriptions/my-log",
		AckDeadlineSeconds: 10,
	})
	assert.NoError(t, err)

	core, _ := observer.New(zap.WarnLevel)
	receiver := &pubsubReceiver{
		instanceName: "dummy",
		logger:       zap.New(core),
		userAgent:    "test-user-agent",

		config: &Config{
			Endpoint:    srv.Addr,
			UseInsecure: true,
			ProjectID:   "my-project",
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 12 * time.Second,
			},
			TracesSubscription:  "my-trace",
			MetricsSubscription: "my-metric",
			LogsSubscription:    "my-log",
		},
		tracesConsumer:  consumertest.NewTracesNop(),
		metricsConsumer: consumertest.NewMetricsNop(),
		logsConsumer:    consumertest.NewLogsNop(),
	}
	assert.NoError(t, receiver.Start(ctx, nil))
	assert.Nil(t, receiver.Shutdown(ctx))
	assert.Nil(t, receiver.Shutdown(ctx))
}

func TestStartReceiverWithNoServer(t *testing.T) {
	ctx := context.Background()
	core, _ := observer.New(zap.WarnLevel)
	receiver := &pubsubReceiver{
		instanceName: "dummy",
		logger:       zap.New(core),
		userAgent:    "test-user-agent",

		config: &Config{
			Endpoint:    "localhost:42",
			UseInsecure: true,
			ProjectID:   "my-project",
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 1 * time.Second,
			},
			ValidateExistence:   true,
			TracesSubscription:  "my-trace-topic",
			MetricsSubscription: "my-metric-topic",
			LogsSubscription:    "my-log-topic",
		},
		tracesConsumer:  consumertest.NewTracesNop(),
		metricsConsumer: consumertest.NewMetricsNop(),
		logsConsumer:    consumertest.NewLogsNop(),
	}
	receiver.tracesConsumer = consumertest.NewTracesNop()
	assert.Error(t, receiver.Start(ctx, nil))
	receiver.tracesConsumer = nil
	receiver.metricsConsumer = consumertest.NewMetricsNop()
	assert.Error(t, receiver.Start(ctx, nil))
	receiver.metricsConsumer = nil
	receiver.logsConsumer = consumertest.NewLogsNop()
	assert.Error(t, receiver.Start(ctx, nil))
}

func TestStartReceiverNoSubscription(t *testing.T) {
	ctx := context.Background()
	// Start a fake server running locally.
	srv := pstest.NewServer()
	defer srv.Close()
	core, _ := observer.New(zap.WarnLevel)
	receiver := &pubsubReceiver{
		instanceName: "dummy",
		logger:       zap.New(core),
		userAgent:    "test-user-agent",

		config: &Config{
			Endpoint:    srv.Addr,
			UseInsecure: true,
			ProjectID:   "my-project",
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 12 * time.Second,
			},
			ValidateExistence:   true,
			TracesSubscription:  "no-trace",
			MetricsSubscription: "no-metric",
			LogsSubscription:    "no-log",
		},
	}
	defer receiver.Shutdown(ctx)
	receiver.tracesConsumer = consumertest.NewTracesNop()
	assert.Error(t, receiver.Start(ctx, nil))
	receiver.tracesConsumer = nil
	receiver.metricsConsumer = consumertest.NewMetricsNop()
	assert.Error(t, receiver.Start(ctx, nil))
	receiver.metricsConsumer = nil
	receiver.logsConsumer = consumertest.NewLogsNop()
	assert.Error(t, receiver.Start(ctx, nil))
}

func TestTraceReceiverHandler(t *testing.T) {
	sink := new(consumertest.TracesSink)
	handler := createTracesReceiverHandler(sink)
	handler(context.Background(), &pubsub.Message{})
}

func TestMetricReceiverHandler(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	handler := createMetricReceiverHandler(sink)
	handler(context.Background(), &pubsub.Message{})
}

func TestLogReceiverHandler(t *testing.T) {
	sink := new(consumertest.LogsSink)
	handler := createLogReceiverHandler(sink)
	handler(context.Background(), &pubsub.Message{})
}
