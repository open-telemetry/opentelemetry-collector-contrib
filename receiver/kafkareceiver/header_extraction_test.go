// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"sync"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestHeaderExtractionTraces(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverCreateSettings: receivertest.NewNopCreateSettings(),
	})
	require.NoError(t, err)
	nextConsumer := &consumertest.TracesSink{}
	c := tracesConsumerGroupHandler{
		unmarshaler:  newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zaptest.NewLogger(t),
		ready:        make(chan bool),
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
	}
	headers := []string{"headerKey1", "headerKey2"}
	c.headerExtractor = &headerExtractor{
		logger:  zaptest.NewLogger(t),
		headers: headers,
	}
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer close(groupClaim.messageChan)
	testSession := testConsumerGroupSession{ctx: ctx}
	require.NoError(t, c.Setup(testSession))
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		err = c.ConsumeClaim(testSession, groupClaim)
		for _, trace := range nextConsumer.AllTraces() {
			for i := 0; i < trace.ResourceSpans().Len(); i++ {
				rs := trace.ResourceSpans().At(i)
				validateHeader(t, rs.Resource(), "kafka.header.headerKey1", "headerValue1")
				validateHeader(t, rs.Resource(), "kafka.header.headerKey2", "headerValue2")
			}
		}
		assert.NoError(t, err)
		wg.Done()
	}()
	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().Resource()
	td.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	unmarshaler := &ptrace.ProtoMarshaler{}
	bts, err := unmarshaler.MarshalTraces(td)
	groupClaim.messageChan <- &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{
			{
				Key:   []byte("headerKey1"),
				Value: []byte("headerValue1"),
			},
			{
				Key:   []byte("headerKey2"),
				Value: []byte("headerValue2"),
			},
		},
		Value: bts,
	}
	cancelFunc()
	wg.Wait()

}

func TestHeaderExtractionLogs(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverCreateSettings: receivertest.NewNopCreateSettings(),
	})
	require.NoError(t, err)
	nextConsumer := &consumertest.LogsSink{}
	unmarshaler := newTextLogsUnmarshaler()
	unmarshaler, err = unmarshaler.WithEnc("utf-8")
	c := logsConsumerGroupHandler{
		unmarshaler:  unmarshaler,
		logger:       zaptest.NewLogger(t),
		ready:        make(chan bool),
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
	}
	headers := []string{"headerKey1", "headerKey2"}
	c.headerExtractor = &headerExtractor{
		logger:  zaptest.NewLogger(t),
		headers: headers,
	}
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer close(groupClaim.messageChan)
	testSession := testConsumerGroupSession{ctx: ctx}
	require.NoError(t, c.Setup(testSession))
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		err = c.ConsumeClaim(testSession, groupClaim)
		for _, logs := range nextConsumer.AllLogs() {
			for i := 0; i < logs.ResourceLogs().Len(); i++ {
				rs := logs.ResourceLogs().At(i)
				validateHeader(t, rs.Resource(), "kafka.header.headerKey1", "headerValueLog1")
				validateHeader(t, rs.Resource(), "kafka.header.headerKey2", "headerValueLog2")
			}
		}
		assert.NoError(t, err)
		wg.Done()
	}()
	groupClaim.messageChan <- &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{
			{
				Key:   []byte("headerKey1"),
				Value: []byte("headerValueLog1"),
			},
			{
				Key:   []byte("headerKey2"),
				Value: []byte("headerValueLog2"),
			},
		},
		Value: []byte("Message"),
	}
	cancelFunc()
	wg.Wait()

}

func TestHeaderExtractionMetrics(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverCreateSettings: receivertest.NewNopCreateSettings(),
	})
	require.NoError(t, err)
	nextConsumer := &consumertest.MetricsSink{}
	c := metricsConsumerGroupHandler{
		unmarshaler:  newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zaptest.NewLogger(t),
		ready:        make(chan bool),
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
	}
	headers := []string{"headerKey1", "headerKey2"}
	c.headerExtractor = &headerExtractor{
		logger:  zaptest.NewLogger(t),
		headers: headers,
	}
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer close(groupClaim.messageChan)
	testSession := testConsumerGroupSession{ctx: ctx}
	require.NoError(t, c.Setup(testSession))
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		err = c.ConsumeClaim(testSession, groupClaim)
		for _, metric := range nextConsumer.AllMetrics() {
			for i := 0; i < metric.ResourceMetrics().Len(); i++ {
				rs := metric.ResourceMetrics().At(i)
				validateHeader(t, rs.Resource(), "kafka.header.headerKey1", "headerValueMetric1")
				validateHeader(t, rs.Resource(), "kafka.header.headerKey2", "headerValueMetric2")
			}
		}
		assert.NoError(t, err)
		wg.Done()
	}()
	ld := testdata.GenerateMetricsOneMetric()
	unmarshaler := &pmetric.ProtoMarshaler{}
	bts, err := unmarshaler.MarshalMetrics(ld)
	groupClaim.messageChan <- &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{
			{
				Key:   []byte("headerKey1"),
				Value: []byte("headerValueMetric1"),
			},
			{
				Key:   []byte("headerKey2"),
				Value: []byte("headerValueMetric2"),
			},
		},
		Value: bts,
	}
	cancelFunc()
	wg.Wait()

}

func validateHeader(t *testing.T, rs pcommon.Resource, headerKey string, headerValue string) {
	val, ok := rs.Attributes().Get(headerKey)
	assert.Equal(t, ok, true)
	assert.Equal(t, val.Str(), headerValue)
}
