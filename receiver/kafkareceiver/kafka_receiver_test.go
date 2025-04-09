// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadatatest"
)

func TestTracesReceiverStart(t *testing.T) {
	c := kafkaTracesConsumer{
		config:           Config{Encoding: defaultEncoding},
		nextConsumer:     consumertest.NewNop(),
		consumeLoopWG:    &sync.WaitGroup{},
		settings:         receivertest.NewNopSettings(metadata.Type),
		consumerGroup:    &testConsumerGroup{},
		telemetryBuilder: nopTelemetryBuilder(t),
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, c.Shutdown(context.Background()))
}

func TestTracesReceiverStartConsume(t *testing.T) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(receivertest.NewNopSettings(metadata.Type).TelemetrySettings)
	require.NoError(t, err)
	c := kafkaTracesConsumer{
		nextConsumer:     consumertest.NewNop(),
		consumeLoopWG:    &sync.WaitGroup{},
		settings:         receivertest.NewNopSettings(metadata.Type),
		consumerGroup:    &testConsumerGroup{},
		telemetryBuilder: telemetryBuilder,
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancelFunc
	require.NoError(t, c.Shutdown(context.Background()))
	c.consumeLoopWG.Add(1)
	c.consumeLoop(ctx, &tracesConsumerGroupHandler{
		ready:            make(chan bool),
		telemetryBuilder: telemetryBuilder,
	})
}

func TestTracesReceiver_error(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	c := kafkaTracesConsumer{
		config:           Config{Encoding: defaultEncoding},
		nextConsumer:     consumertest.NewNop(),
		consumeLoopWG:    &sync.WaitGroup{},
		settings:         settings,
		consumerGroup:    &testConsumerGroup{err: expectedErr},
		telemetryBuilder: nopTelemetryBuilder(t),
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, c.Shutdown(context.Background()))
	assert.Eventually(t, func() bool {
		return logObserver.FilterField(zap.Error(expectedErr)).Len() > 0
	}, 10*time.Second, time.Millisecond*100)
}

func TestTracesConsumerGroupHandler(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	var called atomic.Bool
	c := tracesConsumerGroupHandler{
		unmarshaler: &ptrace.ProtoUnmarshaler{},
		logger:      zap.NewNop(),
		ready:       make(chan bool),
		nextConsumer: func() consumer.Traces {
			c, err := consumer.NewTraces(func(ctx context.Context, _ ptrace.Traces) error {
				defer called.Store(true)
				info := client.FromContext(ctx)
				assert.Equal(t, []string{"abcdefg"}, info.Metadata.Get("x-tenant-id"))
				assert.Equal(t, []string{"1234", "5678"}, info.Metadata.Get("x-request-ids"))
				return nil
			})
			require.NoError(t, err)
			return c
		}(),
		obsrecv:          obsrecv,
		headerExtractor:  &nopHeaderExtractor{},
		telemetryBuilder: telemetryBuilder,
	}

	testSession := testConsumerGroupSession{ctx: context.Background()}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)
	assertInternalTelemetry(t, tel, 0)

	require.NoError(t, c.Cleanup(testSession))
	assertInternalTelemetry(t, tel, 1)

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		assert.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{
			{
				Key:   []byte("x-tenant-id"),
				Value: []byte("abcdefg"),
			},
			{
				Key:   []byte("x-request-ids"),
				Value: []byte("1234"),
			},
			{
				Key:   []byte("x-request-ids"),
				Value: []byte("5678"),
			},
		},
	}
	close(groupClaim.messageChan)
	wg.Wait()
	assert.True(t, called.Load()) // Ensure nextConsumer was called.
}

func TestTracesConsumerGroupHandler_session_done(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	c := tracesConsumerGroupHandler{
		unmarshaler:      &ptrace.ProtoUnmarshaler{},
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewNop(),
		obsrecv:          obsrecv,
		headerExtractor:  &nopHeaderExtractor{},
		telemetryBuilder: telemetryBuilder,
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	testSession := testConsumerGroupSession{ctx: ctx}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)
	assertInternalTelemetry(t, tel, 0)

	require.NoError(t, c.Cleanup(testSession))
	assertInternalTelemetry(t, tel, 1)

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	defer close(groupClaim.messageChan)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		assert.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	cancelFunc()
	wg.Wait()
}

func TestTracesConsumerGroupHandler_error_unmarshal(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)
	c := tracesConsumerGroupHandler{
		unmarshaler:      &ptrace.ProtoUnmarshaler{},
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewNop(),
		obsrecv:          obsrecv,
		headerExtractor:  &nopHeaderExtractor{},
		telemetryBuilder: telemetryBuilder,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	go func() {
		err := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		assert.Error(t, err)
		wg.Done()
	}()
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: []byte("!@#")}
	close(groupClaim.messageChan)
	wg.Wait()
	metadatatest.AssertEqualKafkaReceiverOffsetLag(t, tel, []metricdata.DataPoint[int64]{
		{
			Value: 3,
			Attributes: attribute.NewSet(
				attribute.String("name", ""),
				attribute.String("partition", "5"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualKafkaReceiverCurrentOffset(t, tel, []metricdata.DataPoint[int64]{
		{
			Value: 0,
			Attributes: attribute.NewSet(
				attribute.String("name", ""),
				attribute.String("partition", "5"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualKafkaReceiverMessages(t, tel, []metricdata.DataPoint[int64]{
		{
			Value: 1,
			Attributes: attribute.NewSet(
				attribute.String("name", ""),
				attribute.String("partition", "5"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualKafkaReceiverUnmarshalFailedSpans(t, tel, []metricdata.DataPoint[int64]{
		{
			Value:      1,
			Attributes: attribute.NewSet(attribute.String("name", "")),
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestTracesConsumerGroupHandler_error_nextConsumer(t *testing.T) {
	consumerError := errors.New("failed to consume")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)

	tests := []struct {
		name               string
		err, expectedError error
		expectedBackoff    time.Duration
	}{
		{
			name:            "memory limiter data refused error",
			err:             errMemoryLimiterDataRefused,
			expectedError:   errMemoryLimiterDataRefused,
			expectedBackoff: backoff.DefaultInitialInterval,
		},
		{
			name:            "consumer error that does not require backoff",
			err:             consumerError,
			expectedError:   consumerError,
			expectedBackoff: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backOff := backoff.NewExponentialBackOff()
			backOff.RandomizationFactor = 0
			c := tracesConsumerGroupHandler{
				unmarshaler:      &ptrace.ProtoUnmarshaler{},
				logger:           zap.NewNop(),
				ready:            make(chan bool),
				nextConsumer:     consumertest.NewErr(tt.err),
				obsrecv:          obsrecv,
				headerExtractor:  &nopHeaderExtractor{},
				telemetryBuilder: nopTelemetryBuilder(t),
				backOff:          backOff,
			}

			wg := sync.WaitGroup{}
			wg.Add(1)
			groupClaim := &testConsumerGroupClaim{
				messageChan: make(chan *sarama.ConsumerMessage),
			}
			go func() {
				start := time.Now()
				e := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
				end := time.Now()
				if tt.expectedError != nil {
					assert.EqualError(t, e, tt.expectedError.Error())
				} else {
					assert.NoError(t, e)
				}
				assert.WithinDuration(t, start.Add(tt.expectedBackoff), end, 100*time.Millisecond)
				wg.Done()
			}()

			td := ptrace.NewTraces()
			td.ResourceSpans().AppendEmpty()
			unmarshaler := &ptrace.ProtoMarshaler{}
			bts, err := unmarshaler.MarshalTraces(td)
			require.NoError(t, err)
			groupClaim.messageChan <- &sarama.ConsumerMessage{Value: bts}
			close(groupClaim.messageChan)
			wg.Wait()
		})
	}
}

func TestMetricsReceiverStartConsume(t *testing.T) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(receivertest.NewNopSettings(metadata.Type).TelemetrySettings)
	require.NoError(t, err)
	c := kafkaMetricsConsumer{
		nextConsumer:     consumertest.NewNop(),
		consumeLoopWG:    &sync.WaitGroup{},
		settings:         receivertest.NewNopSettings(metadata.Type),
		consumerGroup:    &testConsumerGroup{},
		telemetryBuilder: telemetryBuilder,
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancelFunc
	require.NoError(t, c.Shutdown(context.Background()))
	c.consumeLoopWG.Add(1)
	c.consumeLoop(ctx, &logsConsumerGroupHandler{
		ready:            make(chan bool),
		telemetryBuilder: telemetryBuilder,
	})
}

func TestMetricsReceiver_error(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	c := kafkaMetricsConsumer{
		config:           Config{Encoding: defaultEncoding},
		nextConsumer:     consumertest.NewNop(),
		consumeLoopWG:    &sync.WaitGroup{},
		settings:         settings,
		consumerGroup:    &testConsumerGroup{err: expectedErr},
		telemetryBuilder: nopTelemetryBuilder(t),
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, c.Shutdown(context.Background()))
	assert.Eventually(t, func() bool {
		return logObserver.FilterField(zap.Error(expectedErr)).Len() > 0
	}, 10*time.Second, time.Millisecond*100)
}

func TestMetricsConsumerGroupHandler(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	var called atomic.Bool
	c := metricsConsumerGroupHandler{
		unmarshaler: &pmetric.ProtoUnmarshaler{},
		logger:      zap.NewNop(),
		ready:       make(chan bool),
		nextConsumer: func() consumer.Metrics {
			c, err := consumer.NewMetrics(func(ctx context.Context, _ pmetric.Metrics) error {
				defer called.Store(true)
				info := client.FromContext(ctx)
				assert.Equal(t, []string{"abcdefg"}, info.Metadata.Get("x-tenant-id"))
				assert.Equal(t, []string{"1234", "5678"}, info.Metadata.Get("x-request-ids"))
				return nil
			})
			require.NoError(t, err)
			return c
		}(),
		obsrecv:          obsrecv,
		headerExtractor:  &nopHeaderExtractor{},
		telemetryBuilder: telemetryBuilder,
	}

	testSession := testConsumerGroupSession{ctx: context.Background()}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)
	assertInternalTelemetry(t, tel, 0)

	require.NoError(t, c.Cleanup(testSession))
	assertInternalTelemetry(t, tel, 1)

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		assert.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{
			{
				Key:   []byte("x-tenant-id"),
				Value: []byte("abcdefg"),
			},
			{
				Key:   []byte("x-request-ids"),
				Value: []byte("1234"),
			},
			{
				Key:   []byte("x-request-ids"),
				Value: []byte("5678"),
			},
		},
	}
	close(groupClaim.messageChan)
	wg.Wait()
	assert.True(t, called.Load()) // Ensure nextConsumer was called.
}

func TestMetricsConsumerGroupHandler_session_done(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	c := metricsConsumerGroupHandler{
		unmarshaler:      &pmetric.ProtoUnmarshaler{},
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewNop(),
		obsrecv:          obsrecv,
		headerExtractor:  &nopHeaderExtractor{},
		telemetryBuilder: telemetryBuilder,
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	testSession := testConsumerGroupSession{ctx: ctx}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)
	assertInternalTelemetry(t, tel, 0)

	require.NoError(t, c.Cleanup(testSession))
	assertInternalTelemetry(t, tel, 1)

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	defer close(groupClaim.messageChan)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		assert.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	cancelFunc()
	wg.Wait()
}

func TestMetricsConsumerGroupHandler_error_unmarshal(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)
	c := metricsConsumerGroupHandler{
		unmarshaler:      &pmetric.ProtoUnmarshaler{},
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewNop(),
		obsrecv:          obsrecv,
		headerExtractor:  &nopHeaderExtractor{},
		telemetryBuilder: telemetryBuilder,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	go func() {
		err := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		assert.Error(t, err)
		wg.Done()
	}()
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: []byte("!@#")}
	close(groupClaim.messageChan)
	wg.Wait()
	metadatatest.AssertEqualKafkaReceiverOffsetLag(t, tel, []metricdata.DataPoint[int64]{
		{
			Value: 3,
			Attributes: attribute.NewSet(
				attribute.String("name", ""),
				attribute.String("partition", "5"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualKafkaReceiverCurrentOffset(t, tel, []metricdata.DataPoint[int64]{
		{
			Value: 0,
			Attributes: attribute.NewSet(
				attribute.String("name", ""),
				attribute.String("partition", "5"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualKafkaReceiverMessages(t, tel, []metricdata.DataPoint[int64]{
		{
			Value: 1,
			Attributes: attribute.NewSet(
				attribute.String("name", ""),
				attribute.String("partition", "5"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualKafkaReceiverUnmarshalFailedMetricPoints(t, tel, []metricdata.DataPoint[int64]{
		{
			Value:      1,
			Attributes: attribute.NewSet(attribute.String("name", "")),
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestMetricsConsumerGroupHandler_error_nextConsumer(t *testing.T) {
	consumerError := errors.New("failed to consume")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)

	tests := []struct {
		name               string
		err, expectedError error
		expectedBackoff    time.Duration
	}{
		{
			name:            "memory limiter data refused error",
			err:             errMemoryLimiterDataRefused,
			expectedError:   errMemoryLimiterDataRefused,
			expectedBackoff: backoff.DefaultInitialInterval,
		},
		{
			name:            "consumer error that does not require backoff",
			err:             consumerError,
			expectedError:   consumerError,
			expectedBackoff: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backOff := backoff.NewExponentialBackOff()
			backOff.RandomizationFactor = 0
			c := metricsConsumerGroupHandler{
				unmarshaler:      &pmetric.ProtoUnmarshaler{},
				logger:           zap.NewNop(),
				ready:            make(chan bool),
				nextConsumer:     consumertest.NewErr(tt.err),
				obsrecv:          obsrecv,
				headerExtractor:  &nopHeaderExtractor{},
				telemetryBuilder: nopTelemetryBuilder(t),
				backOff:          backOff,
			}

			wg := sync.WaitGroup{}
			wg.Add(1)
			groupClaim := &testConsumerGroupClaim{
				messageChan: make(chan *sarama.ConsumerMessage),
			}
			go func() {
				start := time.Now()
				e := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
				end := time.Now()
				if tt.expectedError != nil {
					assert.EqualError(t, e, tt.expectedError.Error())
				} else {
					assert.NoError(t, e)
				}
				assert.WithinDuration(t, start.Add(tt.expectedBackoff), end, 100*time.Millisecond)
				wg.Done()
			}()

			ld := testdata.GenerateMetrics(1)
			unmarshaler := &pmetric.ProtoMarshaler{}
			bts, err := unmarshaler.MarshalMetrics(ld)
			require.NoError(t, err)
			groupClaim.messageChan <- &sarama.ConsumerMessage{Value: bts}
			close(groupClaim.messageChan)
			wg.Wait()
		})
	}
}

func TestLogsReceiverStart(t *testing.T) {
	c := kafkaLogsConsumer{
		config:           *createDefaultConfig().(*Config),
		nextConsumer:     consumertest.NewNop(),
		consumeLoopWG:    &sync.WaitGroup{},
		settings:         receivertest.NewNopSettings(metadata.Type),
		consumerGroup:    &testConsumerGroup{},
		telemetryBuilder: nopTelemetryBuilder(t),
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, c.Shutdown(context.Background()))
}

func TestLogsReceiverStartConsume(t *testing.T) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(receivertest.NewNopSettings(metadata.Type).TelemetrySettings)
	require.NoError(t, err)
	c := kafkaLogsConsumer{
		nextConsumer:     consumertest.NewNop(),
		consumeLoopWG:    &sync.WaitGroup{},
		settings:         receivertest.NewNopSettings(metadata.Type),
		consumerGroup:    &testConsumerGroup{},
		telemetryBuilder: telemetryBuilder,
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancelFunc
	require.NoError(t, c.Shutdown(context.Background()))
	c.consumeLoopWG.Add(1)
	c.consumeLoop(ctx, &logsConsumerGroupHandler{
		ready:            make(chan bool),
		telemetryBuilder: telemetryBuilder,
	})
}

func TestLogsReceiver_error(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	c := kafkaLogsConsumer{
		nextConsumer:     consumertest.NewNop(),
		consumeLoopWG:    &sync.WaitGroup{},
		settings:         settings,
		consumerGroup:    &testConsumerGroup{err: expectedErr},
		config:           *createDefaultConfig().(*Config),
		telemetryBuilder: nopTelemetryBuilder(t),
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, c.Shutdown(context.Background()))
	assert.Eventually(t, func() bool {
		return logObserver.FilterField(zap.Error(expectedErr)).Len() > 0
	}, 10*time.Second, time.Millisecond*100)
}

func TestLogsConsumerGroupHandler(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	var called atomic.Bool
	c := logsConsumerGroupHandler{
		unmarshaler: &plog.ProtoUnmarshaler{},
		logger:      zap.NewNop(),
		ready:       make(chan bool),
		nextConsumer: func() consumer.Logs {
			c, err := consumer.NewLogs(func(ctx context.Context, _ plog.Logs) error {
				defer called.Store(true)
				info := client.FromContext(ctx)
				assert.Equal(t, []string{"abcdefg"}, info.Metadata.Get("x-tenant-id"))
				assert.Equal(t, []string{"1234", "5678"}, info.Metadata.Get("x-request-ids"))
				return nil
			})
			require.NoError(t, err)
			return c
		}(),
		obsrecv:          obsrecv,
		headerExtractor:  &nopHeaderExtractor{},
		telemetryBuilder: telemetryBuilder,
	}

	testSession := testConsumerGroupSession{ctx: context.Background()}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)
	assertInternalTelemetry(t, tel, 0)

	require.NoError(t, c.Cleanup(testSession))
	assertInternalTelemetry(t, tel, 1)

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		assert.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{
			{
				Key:   []byte("x-tenant-id"),
				Value: []byte("abcdefg"),
			},
			{
				Key:   []byte("x-request-ids"),
				Value: []byte("1234"),
			},
			{
				Key:   []byte("x-request-ids"),
				Value: []byte("5678"),
			},
		},
	}
	close(groupClaim.messageChan)
	wg.Wait()
	assert.True(t, called.Load()) // Ensure nextConsumer was called.
}

func TestLogsConsumerGroupHandler_session_done(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	c := logsConsumerGroupHandler{
		unmarshaler:      &plog.ProtoUnmarshaler{},
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewNop(),
		obsrecv:          obsrecv,
		headerExtractor:  &nopHeaderExtractor{},
		telemetryBuilder: telemetryBuilder,
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	testSession := testConsumerGroupSession{ctx: ctx}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)
	assertInternalTelemetry(t, tel, 0)

	require.NoError(t, c.Cleanup(testSession))
	assertInternalTelemetry(t, tel, 1)

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	defer close(groupClaim.messageChan)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		assert.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	cancelFunc()
	wg.Wait()
}

func TestLogsConsumerGroupHandler_error_unmarshal(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)
	c := logsConsumerGroupHandler{
		unmarshaler:      &plog.ProtoUnmarshaler{},
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewNop(),
		obsrecv:          obsrecv,
		headerExtractor:  &nopHeaderExtractor{},
		telemetryBuilder: telemetryBuilder,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	go func() {
		err := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		assert.Error(t, err)
		wg.Done()
	}()
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: []byte("!@#")}
	close(groupClaim.messageChan)
	wg.Wait()
	metadatatest.AssertEqualKafkaReceiverOffsetLag(t, tel, []metricdata.DataPoint[int64]{
		{
			Value: 3,
			Attributes: attribute.NewSet(
				attribute.String("name", ""),
				attribute.String("partition", "5"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualKafkaReceiverCurrentOffset(t, tel, []metricdata.DataPoint[int64]{
		{
			Value: 0,
			Attributes: attribute.NewSet(
				attribute.String("name", ""),
				attribute.String("partition", "5"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualKafkaReceiverMessages(t, tel, []metricdata.DataPoint[int64]{
		{
			Value: 1,
			Attributes: attribute.NewSet(
				attribute.String("name", ""),
				attribute.String("partition", "5"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualKafkaReceiverUnmarshalFailedLogRecords(t, tel, []metricdata.DataPoint[int64]{
		{
			Value: 1,
			Attributes: attribute.NewSet(
				attribute.String("name", ""),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestLogsConsumerGroupHandler_error_nextConsumer(t *testing.T) {
	consumerError := errors.New("failed to consume")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)

	tests := []struct {
		name               string
		err, expectedError error
		expectedBackoff    time.Duration
	}{
		{
			name:            "memory limiter data refused error",
			err:             errMemoryLimiterDataRefused,
			expectedError:   errMemoryLimiterDataRefused,
			expectedBackoff: backoff.DefaultInitialInterval,
		},
		{
			name:            "consumer error that does not require backoff",
			err:             consumerError,
			expectedError:   consumerError,
			expectedBackoff: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backOff := backoff.NewExponentialBackOff()
			backOff.RandomizationFactor = 0
			c := logsConsumerGroupHandler{
				unmarshaler:      &plog.ProtoUnmarshaler{},
				logger:           zap.NewNop(),
				ready:            make(chan bool),
				nextConsumer:     consumertest.NewErr(tt.err),
				obsrecv:          obsrecv,
				headerExtractor:  &nopHeaderExtractor{},
				telemetryBuilder: nopTelemetryBuilder(t),
				backOff:          backOff,
			}

			wg := sync.WaitGroup{}
			wg.Add(1)
			groupClaim := &testConsumerGroupClaim{
				messageChan: make(chan *sarama.ConsumerMessage),
			}
			go func() {
				start := time.Now()
				e := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
				end := time.Now()
				if tt.expectedError != nil {
					assert.EqualError(t, e, tt.expectedError.Error())
				} else {
					assert.NoError(t, e)
				}
				assert.WithinDuration(t, start.Add(tt.expectedBackoff), end, 100*time.Millisecond)
				wg.Done()
			}()

			ld := testdata.GenerateLogs(1)
			unmarshaler := &plog.ProtoMarshaler{}
			bts, err := unmarshaler.MarshalLogs(ld)
			require.NoError(t, err)
			groupClaim.messageChan <- &sarama.ConsumerMessage{Value: bts}
			close(groupClaim.messageChan)
			wg.Wait()
		})
	}
}

type testConsumerGroupClaim struct {
	messageChan chan *sarama.ConsumerMessage
}

var _ sarama.ConsumerGroupClaim = (*testConsumerGroupClaim)(nil)

const (
	testTopic               = "otlp_spans"
	testPartition           = 5
	testInitialOffset       = 6
	testHighWatermarkOffset = 4
)

func (t testConsumerGroupClaim) Topic() string {
	return testTopic
}

func (t testConsumerGroupClaim) Partition() int32 {
	return testPartition
}

func (t testConsumerGroupClaim) InitialOffset() int64 {
	return testInitialOffset
}

func (t testConsumerGroupClaim) HighWaterMarkOffset() int64 {
	return testHighWatermarkOffset
}

func (t testConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage {
	return t.messageChan
}

type testConsumerGroupSession struct {
	ctx context.Context
}

func (t testConsumerGroupSession) Commit() {
}

var _ sarama.ConsumerGroupSession = (*testConsumerGroupSession)(nil)

func (t testConsumerGroupSession) Claims() map[string][]int32 {
	panic("implement me")
}

func (t testConsumerGroupSession) MemberID() string {
	panic("implement me")
}

func (t testConsumerGroupSession) GenerationID() int32 {
	panic("implement me")
}

func (t testConsumerGroupSession) MarkOffset(string, int32, int64, string) {
}

func (t testConsumerGroupSession) ResetOffset(string, int32, int64, string) {
}

func (t testConsumerGroupSession) MarkMessage(*sarama.ConsumerMessage, string) {}

func (t testConsumerGroupSession) Context() context.Context {
	return t.ctx
}

type testConsumerGroup struct {
	once sync.Once
	err  error
}

var _ sarama.ConsumerGroup = (*testConsumerGroup)(nil)

func (t *testConsumerGroup) Consume(ctx context.Context, _ []string, handler sarama.ConsumerGroupHandler) error {
	t.once.Do(func() {
		_ = handler.Setup(testConsumerGroupSession{ctx: ctx})
	})
	return t.err
}

func (t *testConsumerGroup) Errors() <-chan error {
	panic("implement me")
}

func (t *testConsumerGroup) Close() error {
	return nil
}

func (t *testConsumerGroup) Pause(_ map[string][]int32) {
	panic("implement me")
}

func (t *testConsumerGroup) PauseAll() {
	panic("implement me")
}

func (t *testConsumerGroup) Resume(_ map[string][]int32) {
	panic("implement me")
}

func (t *testConsumerGroup) ResumeAll() {
	panic("implement me")
}

func assertInternalTelemetry(t *testing.T, tel *componenttest.Telemetry, partitionClose int64) {
	metadatatest.AssertEqualKafkaReceiverPartitionStart(t, tel, []metricdata.DataPoint[int64]{
		{
			Value:      1,
			Attributes: attribute.NewSet(attribute.String("name", "")),
		},
	}, metricdatatest.IgnoreTimestamp())
	if partitionClose > 0 {
		metadatatest.AssertEqualKafkaReceiverPartitionClose(t, tel, []metricdata.DataPoint[int64]{
			{
				Value:      partitionClose,
				Attributes: attribute.NewSet(attribute.String("name", "")),
			},
		}, metricdatatest.IgnoreTimestamp())
	}
}

func nopTelemetryBuilder(t *testing.T) *metadata.TelemetryBuilder {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(receivertest.NewNopSettings(metadata.Type).TelemetrySettings)
	require.NoError(t, err)
	return telemetryBuilder
}

func Test_newContextWithHeaders(t *testing.T) {
	type args struct {
		ctx     context.Context
		headers []*sarama.RecordHeader
	}
	tests := []struct {
		name string
		args args
		want map[string][]string
	}{
		{
			name: "no headers",
			args: args{
				ctx:     context.Background(),
				headers: []*sarama.RecordHeader{},
			},
			want: map[string][]string{},
		},
		{
			name: "single header",
			args: args{
				ctx: context.Background(),
				headers: []*sarama.RecordHeader{
					{Key: []byte("key1"), Value: []byte("value1")},
				},
			},
			want: map[string][]string{
				"key1": {"value1"},
			},
		},
		{
			name: "multiple headers",
			args: args{
				ctx: context.Background(),
				headers: []*sarama.RecordHeader{
					{Key: []byte("key1"), Value: []byte("value1")},
					{Key: []byte("key2"), Value: []byte("value2")},
				},
			},
			want: map[string][]string{
				"key1": {"value1"},
				"key2": {"value2"},
			},
		},
		{
			name: "duplicate keys",
			args: args{
				ctx: context.Background(),
				headers: []*sarama.RecordHeader{
					{Key: []byte("key1"), Value: []byte("value1")},
					{Key: []byte("key1"), Value: []byte("value2")},
				},
			},
			want: map[string][]string{
				"key1": {"value1", "value2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newContextWithHeaders(tt.args.ctx, tt.args.headers)
			clientInfo := client.FromContext(ctx)
			for k, wantVal := range tt.want {
				val := clientInfo.Metadata.Get(k)
				assert.Equal(t, wantVal, val)
			}
		})
	}
}
