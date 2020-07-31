// Copyright 2020 OpenTelemetry Authors
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

package dockerstatsreceiver

import (
	"context"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

type mockConsumer struct{}

var _ (consumer.MetricsConsumerOld) = (*mockConsumer)(nil)

func (mc *mockConsumer) ConsumeMetricsData(ctx context.Context, md consumerdata.MetricsData) error {
	return nil
}

func (mc *mockConsumer) PullReceivedMetrics() []*metricspb.Metric {
	return nil
}

func TestNewReceiver(t *testing.T) {
	config := &Config{
		Endpoint:           "unix:///run/some.sock",
		CollectionInterval: 1 * time.Second,
	}
	logger := zap.NewNop()
	nextConsumer := &mockConsumer{}
	mr, err := NewReceiver(context.Background(), config, logger, nextConsumer)
	assert.NotNil(t, mr)
	assert.Nil(t, err)

	receiver := mr.(*Receiver)
	assert.Equal(t, config, receiver.config)
	assert.Equal(t, nextConsumer, receiver.nextConsumer)
	assert.Equal(t, logger, receiver.logger)
	assert.Equal(t, "unix", receiver.transport)
}

func TestNewReceiverErrors(t *testing.T) {
	logger := zap.NewNop()
	nextConsumer := &mockConsumer{}

	r, err := NewReceiver(context.Background(), &Config{}, logger, nextConsumer)
	assert.Nil(t, r)
	require.Error(t, err)
	assert.Equal(t, "config.Endpoint must be specified", err.Error())

	r, err = NewReceiver(context.Background(), &Config{Endpoint: "someEndpoint"}, logger, nextConsumer)
	assert.Nil(t, r)
	require.Error(t, err)
	assert.Equal(t, "config.CollectionInterval must be specified", err.Error())
}

func TestErrorsInStart(t *testing.T) {
	unreachable := "unix:///not/a/thing.sock"
	config := &Config{
		Endpoint:           unreachable,
		CollectionInterval: 1 * time.Second,
	}
	logger := zap.NewNop()
	nextConsumer := &mockConsumer{}
	receiver, err := NewReceiver(context.Background(), config, logger, nextConsumer)
	assert.NotNil(t, receiver)
	assert.Nil(t, err)

	// out of order modification to trigger client creation failure
	config.Endpoint = "..not/a/valid/endpoint"

	host := componenttest.NewErrorWaitingHost()
	err = receiver.Start(context.Background(), host)
	require.Error(t, err)

	// restore for connection error a host fatal error
	config.Endpoint = unreachable
	err = receiver.Start(context.Background(), host)
	assert.Nil(t, err)

	received, err := host.WaitForFatalError(500 * time.Millisecond)
	require.True(t, received)
	require.Error(t, err)
	assert.Equal(
		t, "Cannot connect to the Docker daemon at unix:///not/a/thing.sock. Is the docker daemon running?",
		err.Error(),
	)

	require.Nil(t, receiver.Shutdown(context.Background()))
}
