// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadatatest"
)

func TestNewTracesReceiver_version_err(t *testing.T) {
	c := Config{
		Encoding:        defaultEncoding,
		ProtocolVersion: "none",
	}
	r, err := newTracesReceiver(c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestNewTracesReceiver_encoding_err(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Encoding = "foo"
	r, err := newTracesReceiver(*c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewTracesReceiver_initial_offset_err(t *testing.T) {
	c := Config{
		InitialOffset: "foo",
		Encoding:      defaultEncoding,
	}
	r, err := newTracesReceiver(c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.EqualError(t, err, errInvalidInitialOffset.Error())
}

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
	c := tracesConsumerGroupHandler{
		unmarshaler:      newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewNop(),
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

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestTracesConsumerGroupHandler_session_done(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	c := tracesConsumerGroupHandler{
		unmarshaler:      newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
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
		unmarshaler:      newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
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
				unmarshaler:      newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
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

func TestTracesReceiver_encoding_extension(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	c := kafkaTracesConsumer{
		config:           Config{Encoding: "traces_encoding"},
		nextConsumer:     consumertest.NewNop(),
		consumeLoopWG:    &sync.WaitGroup{},
		settings:         settings,
		consumerGroup:    &testConsumerGroup{err: expectedErr},
		telemetryBuilder: nopTelemetryBuilder(t),
	}

	require.NoError(t, c.Start(context.Background(), &testComponentHost{}))
	require.NoError(t, c.Shutdown(context.Background()))
	assert.Eventually(t, func() bool {
		return logObserver.FilterField(zap.Error(expectedErr)).Len() > 0
	}, 10*time.Second, time.Millisecond*100)
}

func TestNewMetricsReceiver_version_err(t *testing.T) {
	c := Config{
		Encoding:        defaultEncoding,
		ProtocolVersion: "none",
	}
	r, err := newMetricsReceiver(c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestNewMetricsReceiver_encoding_err(t *testing.T) {
	c := Config{
		Encoding: "foo",
	}
	r, err := newMetricsReceiver(c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewMetricsReceiver_initial_offset_err(t *testing.T) {
	c := Config{
		InitialOffset: "foo",
		Encoding:      defaultEncoding,
	}
	r, err := newMetricsReceiver(c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.EqualError(t, err, errInvalidInitialOffset.Error())
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
	c := metricsConsumerGroupHandler{
		unmarshaler:      newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewNop(),
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

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestMetricsConsumerGroupHandler_session_done(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	c := metricsConsumerGroupHandler{
		unmarshaler:      newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
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
		unmarshaler:      newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
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
				unmarshaler:      newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
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

func TestMetricsReceiver_encoding_extension(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	c := kafkaMetricsConsumer{
		config:           Config{Encoding: "metrics_encoding"},
		nextConsumer:     consumertest.NewNop(),
		consumeLoopWG:    &sync.WaitGroup{},
		settings:         settings,
		consumerGroup:    &testConsumerGroup{err: expectedErr},
		telemetryBuilder: nopTelemetryBuilder(t),
	}

	require.NoError(t, c.Start(context.Background(), &testComponentHost{}))
	require.NoError(t, c.Shutdown(context.Background()))
	assert.Eventually(t, func() bool {
		return logObserver.FilterField(zap.Error(expectedErr)).Len() > 0
	}, 10*time.Second, time.Millisecond*100)
}

func TestNewLogsReceiver_version_err(t *testing.T) {
	c := Config{
		Encoding:        defaultEncoding,
		ProtocolVersion: "none",
	}
	r, err := newLogsReceiver(c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestNewLogsReceiver_encoding_err(t *testing.T) {
	c := Config{
		Encoding: "foo",
	}
	r, err := newLogsReceiver(c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewLogsReceiver_initial_offset_err(t *testing.T) {
	c := Config{
		InitialOffset: "foo",
		Encoding:      defaultEncoding,
	}
	r, err := newLogsReceiver(c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.EqualError(t, err, errInvalidInitialOffset.Error())
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
	c := logsConsumerGroupHandler{
		unmarshaler:      newPdataLogsUnmarshaler(&plog.ProtoUnmarshaler{}, defaultEncoding),
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewNop(),
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

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestLogsConsumerGroupHandler_session_done(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewTelemetrySettings())
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	c := logsConsumerGroupHandler{
		unmarshaler:      newPdataLogsUnmarshaler(&plog.ProtoUnmarshaler{}, defaultEncoding),
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
		unmarshaler:      newPdataLogsUnmarshaler(&plog.ProtoUnmarshaler{}, defaultEncoding),
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
				unmarshaler:      newPdataLogsUnmarshaler(&plog.ProtoUnmarshaler{}, defaultEncoding),
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

// Test unmarshaler for different charsets and encodings.
func TestLogsConsumerGroupHandler_unmarshal_text(t *testing.T) {
	tests := []struct {
		name string
		text string
		enc  string
	}{
		{
			name: "unmarshal test for English (ASCII characters) with text_utf8",
			text: "ASCII characters test",
			enc:  "utf8",
		},
		{
			name: "unmarshal test for unicode with text_utf8",
			text: "UTF8 测试 測試 テスト 테스트 ☺️",
			enc:  "utf8",
		},
		{
			name: "unmarshal test for Simplified Chinese with text_gbk",
			text: "GBK 简体中文解码测试",
			enc:  "gbk",
		},
		{
			name: "unmarshal test for Japanese with text_shift_jis",
			text: "Shift_JIS 日本のデコードテスト",
			enc:  "shift_jis",
		},
		{
			name: "unmarshal test for Korean with text_euc-kr",
			text: "EUC-KR 한국 디코딩 테스트",
			enc:  "euc-kr",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
			require.NoError(t, err)
			unmarshaler := newTextLogsUnmarshaler()
			unmarshaler, err = unmarshaler.WithEnc(test.enc)
			require.NoError(t, err)
			sink := &consumertest.LogsSink{}
			c := logsConsumerGroupHandler{
				unmarshaler:      unmarshaler,
				logger:           zap.NewNop(),
				ready:            make(chan bool),
				nextConsumer:     sink,
				obsrecv:          obsrecv,
				headerExtractor:  &nopHeaderExtractor{},
				telemetryBuilder: nopTelemetryBuilder(t),
			}

			wg := sync.WaitGroup{}
			wg.Add(1)
			groupClaim := &testConsumerGroupClaim{
				messageChan: make(chan *sarama.ConsumerMessage),
			}
			go func() {
				err = c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
				assert.NoError(t, err)
				wg.Done()
			}()
			enc, err := textutils.LookupEncoding(test.enc)
			require.NoError(t, err)
			encoder := enc.NewEncoder()
			encoded, err := encoder.Bytes([]byte(test.text))
			require.NoError(t, err)
			t1 := time.Now()
			groupClaim.messageChan <- &sarama.ConsumerMessage{Value: encoded}
			close(groupClaim.messageChan)
			wg.Wait()
			require.Equal(t, 1, sink.LogRecordCount())
			log := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
			assert.Equal(t, log.Body().Str(), test.text)
			assert.LessOrEqual(t, t1, log.ObservedTimestamp().AsTime())
			assert.LessOrEqual(t, log.ObservedTimestamp().AsTime(), time.Now())
		})
	}
}

func TestGetLogsUnmarshaler_encoding_text(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
	}{
		{
			name:     "default text encoding",
			encoding: "text",
		},
		{
			name:     "utf-8 text encoding",
			encoding: "text_utf-8",
		},
		{
			name:     "gbk text encoding",
			encoding: "text_gbk",
		},
		{
			name:     "shift_jis text encoding, which contains an underline",
			encoding: "text_shift_jis",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := getLogsUnmarshaler(test.encoding, defaultLogsUnmarshalers("Test Version", zap.NewNop()))
			assert.NoError(t, err)
		})
	}
}

func TestCreateLogs_encoding_text_error(t *testing.T) {
	cfg := Config{
		Encoding: "text_uft-8",
	}
	r, err := newLogsReceiver(cfg, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	// encoding error comes first
	assert.Error(t, err, "unsupported encoding")
}

func TestLogsReceiver_encoding_extension(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	c := kafkaLogsConsumer{
		config:           Config{Encoding: "logs_encoding"},
		nextConsumer:     consumertest.NewNop(),
		consumeLoopWG:    &sync.WaitGroup{},
		settings:         settings,
		consumerGroup:    &testConsumerGroup{err: expectedErr},
		telemetryBuilder: nopTelemetryBuilder(t),
	}

	require.NoError(t, c.Start(context.Background(), &testComponentHost{}))
	require.NoError(t, c.Shutdown(context.Background()))
	assert.Eventually(t, func() bool {
		return logObserver.FilterField(zap.Error(expectedErr)).Len() > 0
	}, 10*time.Second, time.Millisecond*100)
}

func TestToSaramaInitialOffset_earliest(t *testing.T) {
	saramaInitialOffset, err := toSaramaInitialOffset(offsetEarliest)

	require.NoError(t, err)
	assert.Equal(t, sarama.OffsetOldest, saramaInitialOffset)
}

func TestToSaramaInitialOffset_latest(t *testing.T) {
	saramaInitialOffset, err := toSaramaInitialOffset(offsetLatest)

	require.NoError(t, err)
	assert.Equal(t, sarama.OffsetNewest, saramaInitialOffset)
}

func TestToSaramaInitialOffset_default(t *testing.T) {
	saramaInitialOffset, err := toSaramaInitialOffset("")

	require.NoError(t, err)
	assert.Equal(t, sarama.OffsetNewest, saramaInitialOffset)
}

func TestToSaramaInitialOffset_invalid(t *testing.T) {
	_, err := toSaramaInitialOffset("other")

	assert.Equal(t, err, errInvalidInitialOffset)
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

func TestLoadEncodingExtension_logs(t *testing.T) {
	extension, err := loadEncodingExtension[plog.Unmarshaler](&testComponentHost{}, "logs_encoding")
	require.NoError(t, err)
	require.NotNil(t, extension)
}

func TestLoadEncodingExtension_notfound_error(t *testing.T) {
	extension, err := loadEncodingExtension[plog.Unmarshaler](&testComponentHost{}, "logs_notfound")
	require.Error(t, err)
	require.Nil(t, extension)
}

func TestLoadEncodingExtension_nounmarshaler_error(t *testing.T) {
	extension, err := loadEncodingExtension[plog.Unmarshaler](&testComponentHost{}, "logs_nounmarshaler")
	require.Error(t, err)
	require.Nil(t, extension)
}

type testComponentHost struct{}

func (h *testComponentHost) GetExtensions() map[component.ID]component.Component {
	return map[component.ID]component.Component{
		component.MustNewID("logs_encoding"):      &nopComponent{},
		component.MustNewID("logs_nounmarshaler"): &nopNoUnmarshalerComponent{},
		component.MustNewID("metrics_encoding"):   &nopComponent{},
		component.MustNewID("traces_encoding"):    &nopComponent{},
	}
}

type nopComponent struct{}

func (c *nopComponent) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (c *nopComponent) Shutdown(_ context.Context) error {
	return nil
}

func (c *nopComponent) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	return plog.NewLogs(), nil
}

func (c *nopComponent) UnmarshalMetrics(_ []byte) (pmetric.Metrics, error) {
	return pmetric.NewMetrics(), nil
}

func (c *nopComponent) UnmarshalTraces(_ []byte) (ptrace.Traces, error) {
	return ptrace.NewTraces(), nil
}

type nopNoUnmarshalerComponent struct{}

func (c *nopNoUnmarshalerComponent) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (c *nopNoUnmarshalerComponent) Shutdown(_ context.Context) error {
	return nil
}
