// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel/attribute"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func setFranzGo(tb testing.TB, value bool) {
	currentFranzState := franzGoConsumerFeatureGate.IsEnabled()
	require.NoError(tb, featuregate.GlobalRegistry().Set(franzGoConsumerFeatureGate.ID(), value))
	tb.Cleanup(func() {
		require.NoError(tb, featuregate.GlobalRegistry().Set(franzGoConsumerFeatureGate.ID(), currentFranzState))
	})
}

func TestConsumerShutdownConsuming(t *testing.T) {
	// Test that the consumer shuts down while consuming a message and
	// commits the offset after it's left the group.
	setFranzGo(t, true)

	topic := "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.ConsumerConfig = configkafka.ConsumerConfig{
		GroupID:    t.Name(),
		AutoCommit: configkafka.AutoCommitConfig{Enable: true, Interval: 10 * time.Second},
	}

	var called atomic.Int64
	var wg sync.WaitGroup
	settings, _, _ := mustNewSettings(t)
	newConsumeFunc := func() (newConsumeMessageFunc, chan<- struct{}) {
		consuming := make(chan struct{})
		return func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
			return func(ctx context.Context, _ kafkaMessage, _ attribute.Set) error {
				wg.Add(1)
				defer wg.Done()

				<-consuming
				called.Add(1)
				// Wait for the consumer to shutdown.
				<-ctx.Done()
				return nil
			}, nil
		}, consuming
	}

	// Send some traces to the otlp_spans topic.
	traces := testdata.GenerateTraces(5)
	data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)
	rs := []*kgo.Record{
		{Topic: topic, Value: data},
		{Topic: topic, Value: data},
	}

	test := func(expected int64) {
		ctx := context.Background()
		consumeFn, consuming := newConsumeFunc()
		consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, consumeFn)
		require.NoError(t, err)
		require.NoError(t, consumer.Start(ctx, componenttest.NewNopHost()))
		require.NoError(t, kafkaClient.ProduceSync(ctx, rs...).FirstErr())

		select {
		case consuming <- struct{}{}:
			close(consuming) // Close the channel so the rest exit.
		case <-time.After(time.Second):
			// panic("expected to consume a message")
		}
		require.NoError(t, consumer.Shutdown(ctx))
		wg.Wait() // Wait for the consume functions to exit.
		// Ensure that the consume function was called twice.
		require.Equal(t, expected, called.Load())
	}

	test(2)
	test(4)

	offsets, err := kadm.NewClient(kafkaClient).FetchOffsets(context.Background(), t.Name())
	require.NoError(t, err)
	// Lookup the last committed offset for partition 0
	offset, ok := offsets.Lookup(topic, 0)
	require.True(t, ok)
	require.Equal(t, int64(4), offset.Offset.At)
}
