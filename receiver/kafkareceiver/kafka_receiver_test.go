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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func TestNewTracesReceiver_version_err(t *testing.T) {
	c := Config{
		Encoding:        defaultEncoding,
		ProtocolVersion: "none",
	}
	unmarshaler := defaultTracesUnmarshalers()[c.Encoding]
	r, err := newTracesReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestNewTracesReceiver_encoding_err(t *testing.T) {
	c := Config{
		Encoding: "foo",
	}
	unmarshaler := defaultTracesUnmarshalers()[c.Encoding]
	r, err := newTracesReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.Error(t, err)
	assert.Nil(t, r)
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewTracesReceiver_err_auth_type(t *testing.T) {
	c := Config{
		ProtocolVersion: "2.0.0",
		Authentication: kafka.Authentication{
			TLS: &configtls.ClientConfig{
				Config: configtls.Config{
					CAFile: "/doesnotexist",
				},
			},
		},
		Encoding: defaultEncoding,
		Metadata: kafkaexporter.Metadata{
			Full: false,
		},
	}
	unmarshaler := defaultTracesUnmarshalers()[c.Encoding]
	r, err := newTracesReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.Contains(t, err.Error(), "failed to load TLS config")
}

func TestNewTracesReceiver_initial_offset_err(t *testing.T) {
	c := Config{
		InitialOffset: "foo",
		Encoding:      defaultEncoding,
	}
	unmarshaler := defaultTracesUnmarshalers()[c.Encoding]
	r, err := newTracesReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.EqualError(t, err, errInvalidInitialOffset.Error())
}

func TestTracesReceiverStart(t *testing.T) {
	c := kafkaTracesConsumer{
		nextConsumer:     consumertest.NewNop(),
		settings:         receivertest.NewNopSettings(),
		consumerGroup:    &testConsumerGroup{},
		telemetryBuilder: nopTelemetryBuilder(t),
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, c.Shutdown(context.Background()))
}

func TestTracesReceiverStartConsume(t *testing.T) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(receivertest.NewNopSettings().TelemetrySettings)
	require.NoError(t, err)
	c := kafkaTracesConsumer{
		nextConsumer:     consumertest.NewNop(),
		settings:         receivertest.NewNopSettings(),
		consumerGroup:    &testConsumerGroup{},
		telemetryBuilder: telemetryBuilder,
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancelFunc
	require.NoError(t, c.Shutdown(context.Background()))
	err = c.consumeLoop(ctx, &tracesConsumerGroupHandler{
		ready:            make(chan bool),
		telemetryBuilder: telemetryBuilder,
	})
	assert.EqualError(t, err, context.Canceled.Error())
}

func TestTracesReceiver_error(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings()
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	c := kafkaTracesConsumer{
		nextConsumer:     consumertest.NewNop(),
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
	tel := setupTestTelemetry()
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewSettings().TelemetrySettings)
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
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
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestTracesConsumerGroupHandler_session_done(t *testing.T) {
	tel := setupTestTelemetry()
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewSettings().TelemetrySettings)
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
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
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	cancelFunc()
	wg.Wait()
}

func TestTracesConsumerGroupHandler_error_unmarshal(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
	require.NoError(t, err)
	tel := setupTestTelemetry()
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewSettings().TelemetrySettings)
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
		require.Error(t, err)
		wg.Done()
	}()
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: []byte("!@#")}
	close(groupClaim.messageChan)
	wg.Wait()
	tel.assertMetrics(t, []metricdata.Metrics{
		{
			Name:        "otelcol_kafka_receiver_offset_lag",
			Unit:        "1",
			Description: "Current offset lag",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: 3,
						Attributes: attribute.NewSet(
							attribute.String("name", ""),
							attribute.String("partition", "5"),
						),
					},
				},
			},
		},
		{
			Name:        "otelcol_kafka_receiver_current_offset",
			Unit:        "1",
			Description: "Current message offset",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: 0,
						Attributes: attribute.NewSet(
							attribute.String("name", ""),
							attribute.String("partition", "5"),
						),
					},
				},
			},
		},
		{
			Name:        "otelcol_kafka_receiver_messages",
			Unit:        "1",
			Description: "Number of received messages",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: 1,
						Attributes: attribute.NewSet(
							attribute.String("name", ""),
							attribute.String("partition", "5"),
						),
					},
				},
			},
		},
		{
			Name:        "otelcol_kafka_receiver_unmarshal_failed_spans",
			Unit:        "1",
			Description: "Number of spans failed to be unmarshaled",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value:      1,
						Attributes: attribute.NewSet(attribute.String("name", "")),
					},
				},
			},
		},
	})
}

func TestTracesConsumerGroupHandler_error_nextConsumer(t *testing.T) {
	consumerError := errors.New("failed to consume")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
	require.NoError(t, err)
	c := tracesConsumerGroupHandler{
		unmarshaler:      newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewErr(consumerError),
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
		e := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		assert.EqualError(t, e, consumerError.Error())
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
}

func TestNewMetricsReceiver_version_err(t *testing.T) {
	c := Config{
		Encoding:        defaultEncoding,
		ProtocolVersion: "none",
	}
	unmarshaler := defaultMetricsUnmarshalers()[c.Encoding]
	r, err := newMetricsReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestNewMetricsReceiver_encoding_err(t *testing.T) {
	c := Config{
		Encoding: "foo",
	}
	unmarshaler := defaultMetricsUnmarshalers()[c.Encoding]
	_, err := newMetricsReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.Error(t, err)
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewMetricsExporter_err_auth_type(t *testing.T) {
	c := Config{
		ProtocolVersion: "2.0.0",
		Authentication: kafka.Authentication{
			TLS: &configtls.ClientConfig{
				Config: configtls.Config{
					CAFile: "/doesnotexist",
				},
			},
		},
		Encoding: defaultEncoding,
		Metadata: kafkaexporter.Metadata{
			Full: false,
		},
	}
	unmarshaler := defaultMetricsUnmarshalers()[c.Encoding]
	r, err := newMetricsReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS config")
}

func TestNewMetricsReceiver_initial_offset_err(t *testing.T) {
	c := Config{
		InitialOffset: "foo",
		Encoding:      defaultEncoding,
	}
	unmarshaler := defaultMetricsUnmarshalers()[c.Encoding]
	r, err := newMetricsReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.EqualError(t, err, errInvalidInitialOffset.Error())
}

func TestMetricsReceiverStartConsume(t *testing.T) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(receivertest.NewNopSettings().TelemetrySettings)
	require.NoError(t, err)
	c := kafkaMetricsConsumer{
		nextConsumer:     consumertest.NewNop(),
		settings:         receivertest.NewNopSettings(),
		consumerGroup:    &testConsumerGroup{},
		telemetryBuilder: telemetryBuilder,
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancelFunc
	require.NoError(t, c.Shutdown(context.Background()))
	err = c.consumeLoop(ctx, &logsConsumerGroupHandler{
		ready:            make(chan bool),
		telemetryBuilder: telemetryBuilder,
	})
	assert.EqualError(t, err, context.Canceled.Error())
}

func TestMetricsReceiver_error(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings()
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	c := kafkaMetricsConsumer{
		nextConsumer:     consumertest.NewNop(),
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
	tel := setupTestTelemetry()
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewSettings().TelemetrySettings)
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
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
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestMetricsConsumerGroupHandler_session_done(t *testing.T) {
	tel := setupTestTelemetry()
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewSettings().TelemetrySettings)
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
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
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	cancelFunc()
	wg.Wait()
}

func TestMetricsConsumerGroupHandler_error_unmarshal(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
	require.NoError(t, err)
	tel := setupTestTelemetry()
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewSettings().TelemetrySettings)
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
		require.Error(t, err)
		wg.Done()
	}()
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: []byte("!@#")}
	close(groupClaim.messageChan)
	wg.Wait()
	tel.assertMetrics(t, []metricdata.Metrics{
		{
			Name:        "otelcol_kafka_receiver_offset_lag",
			Unit:        "1",
			Description: "Current offset lag",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: 3,
						Attributes: attribute.NewSet(
							attribute.String("name", ""),
							attribute.String("partition", "5"),
						),
					},
				},
			},
		},
		{
			Name:        "otelcol_kafka_receiver_current_offset",
			Unit:        "1",
			Description: "Current message offset",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: 0,
						Attributes: attribute.NewSet(
							attribute.String("name", ""),
							attribute.String("partition", "5"),
						),
					},
				},
			},
		},
		{
			Name:        "otelcol_kafka_receiver_messages",
			Unit:        "1",
			Description: "Number of received messages",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: 1,
						Attributes: attribute.NewSet(
							attribute.String("name", ""),
							attribute.String("partition", "5"),
						),
					},
				},
			},
		},
		{
			Name:        "otelcol_kafka_receiver_unmarshal_failed_metric_points",
			Unit:        "1",
			Description: "Number of metric points failed to be unmarshaled",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value:      1,
						Attributes: attribute.NewSet(attribute.String("name", "")),
					},
				},
			},
		},
	})
}

func TestMetricsConsumerGroupHandler_error_nextConsumer(t *testing.T) {
	consumerError := errors.New("failed to consume")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
	require.NoError(t, err)
	c := metricsConsumerGroupHandler{
		unmarshaler:      newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewErr(consumerError),
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
		e := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		assert.EqualError(t, e, consumerError.Error())
		wg.Done()
	}()

	ld := testdata.GenerateMetrics(1)
	unmarshaler := &pmetric.ProtoMarshaler{}
	bts, err := unmarshaler.MarshalMetrics(ld)
	require.NoError(t, err)
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: bts}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestNewLogsReceiver_version_err(t *testing.T) {
	c := Config{
		Encoding:        defaultEncoding,
		ProtocolVersion: "none",
	}
	unmarshaler := defaultLogsUnmarshalers("Test Version", zap.NewNop())[c.Encoding]
	r, err := newLogsReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestNewLogsReceiver_encoding_err(t *testing.T) {
	c := Config{
		Encoding: "foo",
	}
	unmarshaler := defaultLogsUnmarshalers("Test Version", zap.NewNop())[c.Encoding]
	r, err := newLogsReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.Error(t, err)
	assert.Nil(t, r)
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewLogsExporter_err_auth_type(t *testing.T) {
	c := Config{
		ProtocolVersion: "2.0.0",
		Authentication: kafka.Authentication{
			TLS: &configtls.ClientConfig{
				Config: configtls.Config{
					CAFile: "/doesnotexist",
				},
			},
		},
		Encoding: defaultEncoding,
		Metadata: kafkaexporter.Metadata{
			Full: false,
		},
	}
	unmarshaler := defaultLogsUnmarshalers("Test Version", zap.NewNop())[c.Encoding]
	r, err := newLogsReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS config")
}

func TestNewLogsReceiver_initial_offset_err(t *testing.T) {
	c := Config{
		InitialOffset: "foo",
		Encoding:      defaultEncoding,
	}
	unmarshaler := defaultLogsUnmarshalers("Test Version", zap.NewNop())[c.Encoding]
	r, err := newLogsReceiver(c, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.EqualError(t, err, errInvalidInitialOffset.Error())
}

func TestLogsReceiverStart(t *testing.T) {
	c := kafkaLogsConsumer{
		nextConsumer:     consumertest.NewNop(),
		settings:         receivertest.NewNopSettings(),
		consumerGroup:    &testConsumerGroup{},
		telemetryBuilder: nopTelemetryBuilder(t),
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, c.Shutdown(context.Background()))
}

func TestLogsReceiverStartConsume(t *testing.T) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(receivertest.NewNopSettings().TelemetrySettings)
	require.NoError(t, err)
	c := kafkaLogsConsumer{
		nextConsumer:     consumertest.NewNop(),
		settings:         receivertest.NewNopSettings(),
		consumerGroup:    &testConsumerGroup{},
		telemetryBuilder: telemetryBuilder,
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancelFunc
	require.NoError(t, c.Shutdown(context.Background()))
	err = c.consumeLoop(ctx, &logsConsumerGroupHandler{
		ready:            make(chan bool),
		telemetryBuilder: telemetryBuilder,
	})
	assert.EqualError(t, err, context.Canceled.Error())
}

func TestLogsReceiver_error(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings()
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	c := kafkaLogsConsumer{
		nextConsumer:     consumertest.NewNop(),
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
	tel := setupTestTelemetry()
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewSettings().TelemetrySettings)
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
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
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestLogsConsumerGroupHandler_session_done(t *testing.T) {
	tel := setupTestTelemetry()
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewSettings().TelemetrySettings)
	require.NoError(t, err)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
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
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	cancelFunc()
	wg.Wait()
}

func TestLogsConsumerGroupHandler_error_unmarshal(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
	require.NoError(t, err)
	tel := setupTestTelemetry()
	telemetryBuilder, err := metadata.NewTelemetryBuilder(tel.NewSettings().TelemetrySettings)
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
		require.Error(t, err)
		wg.Done()
	}()
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: []byte("!@#")}
	close(groupClaim.messageChan)
	wg.Wait()
	tel.assertMetrics(t, []metricdata.Metrics{
		{
			Name:        "otelcol_kafka_receiver_offset_lag",
			Unit:        "1",
			Description: "Current offset lag",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: 3,
						Attributes: attribute.NewSet(
							attribute.String("name", ""),
							attribute.String("partition", "5"),
						),
					},
				},
			},
		},
		{
			Name:        "otelcol_kafka_receiver_current_offset",
			Unit:        "1",
			Description: "Current message offset",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: 0,
						Attributes: attribute.NewSet(
							attribute.String("name", ""),
							attribute.String("partition", "5"),
						),
					},
				},
			},
		},
		{
			Name:        "otelcol_kafka_receiver_messages",
			Unit:        "1",
			Description: "Number of received messages",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: 1,
						Attributes: attribute.NewSet(
							attribute.String("name", ""),
							attribute.String("partition", "5"),
						),
					},
				},
			},
		},
		{
			Name:        "otelcol_kafka_receiver_unmarshal_failed_log_records",
			Unit:        "1",
			Description: "Number of log records failed to be unmarshaled",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value:      1,
						Attributes: attribute.NewSet(attribute.String("name", "")),
					},
				},
			},
		},
	})
}

func TestLogsConsumerGroupHandler_error_nextConsumer(t *testing.T) {
	consumerError := errors.New("failed to consume")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
	require.NoError(t, err)
	c := logsConsumerGroupHandler{
		unmarshaler:      newPdataLogsUnmarshaler(&plog.ProtoUnmarshaler{}, defaultEncoding),
		logger:           zap.NewNop(),
		ready:            make(chan bool),
		nextConsumer:     consumertest.NewErr(consumerError),
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
		e := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		assert.EqualError(t, e, consumerError.Error())
		wg.Done()
	}()

	ld := testdata.GenerateLogs(1)
	unmarshaler := &plog.ProtoMarshaler{}
	bts, err := unmarshaler.MarshalLogs(ld)
	require.NoError(t, err)
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: bts}
	close(groupClaim.messageChan)
	wg.Wait()
}

// Test unmarshaler for different charsets and encodings.
func TestLogsConsumerGroupHandler_unmarshal_text(t *testing.T) {
	tests := []struct {
		name string
		text string
		enc  string
	}{
		{
			name: "unmarshal test for Englist (ASCII characters) with text_utf8",
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
			obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
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
			encCfg := textutils.NewEncodingConfig()
			encCfg.Encoding = test.enc
			enc, err := encCfg.Build()
			require.NoError(t, err)
			encoder := enc.Encoding.NewEncoder()
			encoded, err := encoder.Bytes([]byte(test.text))
			require.NoError(t, err)
			t1 := time.Now()
			groupClaim.messageChan <- &sarama.ConsumerMessage{Value: encoded}
			close(groupClaim.messageChan)
			wg.Wait()
			require.Equal(t, sink.LogRecordCount(), 1)
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

func TestCreateLogsReceiver_encoding_text_error(t *testing.T) {
	cfg := Config{
		Encoding: "text_uft-8",
	}
	unmarshaler := defaultLogsUnmarshalers("Test Version", zap.NewNop())[cfg.Encoding]
	_, err := newLogsReceiver(cfg, receivertest.NewNopSettings(), unmarshaler, consumertest.NewNop())
	// encoding error comes first
	assert.Error(t, err, "unsupported encoding")
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
	panic("implement me")
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

func assertInternalTelemetry(t *testing.T, tel componentTestTelemetry, partitionClose int64) {
	wantMetrics := []metricdata.Metrics{
		{
			Name:        "otelcol_kafka_receiver_partition_start",
			Unit:        "1",
			Description: "Number of started partitions",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value:      1,
						Attributes: attribute.NewSet(attribute.String("name", "")),
					},
				},
			},
		},
	}
	if partitionClose > 0 {
		wantMetrics = append(wantMetrics, metricdata.Metrics{
			Name:        "otelcol_kafka_receiver_partition_close",
			Unit:        "1",
			Description: "Number of finished partitions",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value:      partitionClose,
						Attributes: attribute.NewSet(attribute.String("name", "")),
					},
				},
			},
		})
	}
	tel.assertMetrics(t, wantMetrics)
}

func nopTelemetryBuilder(t *testing.T) *metadata.TelemetryBuilder {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(receivertest.NewNopSettings().TelemetrySettings)
	require.NoError(t, err)
	return telemetryBuilder
}
