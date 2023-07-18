package kafkareceiver

import (
	"context"
	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"bytes"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"math"
	"strings"
	"sync"
	"testing"
)

func TestHeaderExtractionTraces(t *testing.T) {
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverCreateSettings: receivertest.NewNopCreateSettings()})
	nextConsumer := (new(consumertest.TracesSink))
	c := tracesConsumerGroupHandler{
		unmarshaler:  newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: nextConsumer,
		// nextConsumer: &mockConsumer{},
		obsrecv: obsrecv,
	}
	headers := []string{"headerKey1", "headerKey2"}
	c.headerExtractor = &headerExtractor{
		logger:  zap.NewNop(),
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
				val, ok := rs.Resource().Attributes().Get("kafka.header.headerKey1")
				assert.Equal(t, ok, true)
				assert.Equal(t, val.Str(), "headerValue1")
				val, ok = rs.Resource().Attributes().Get("kafka.header.headerKey2")
				assert.Equal(t, ok, true)
				assert.Equal(t, val.Str(), "headerValue2")
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
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverCreateSettings: receivertest.NewNopCreateSettings()})
	nextConsumer := new(consumertest.LogsSink)
	unmarshaler := newTextLogsUnmarshaler()
	unmarshaler, err = unmarshaler.WithEnc("utf-8")
	c := logsConsumerGroupHandler{
		unmarshaler:  unmarshaler,
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
	}
	headers := []string{"headerKey1", "headerKey2"}
	c.headerExtractor = &headerExtractor{
		logger:  zap.NewNop(),
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
				val, ok := rs.Resource().Attributes().Get("kafka.header.headerKey1")
				assert.Equal(t, ok, true)
				assert.Equal(t, val.Str(), "headerValue1")
				val, ok = rs.Resource().Attributes().Get("kafka.header.headerKey2")
				assert.Equal(t, ok, true)
				assert.Equal(t, val.Str(), "headerValue2")
			}
		}
		assert.NoError(t, err)
		wg.Done()
	}()
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
		Value: []byte("Message"),
	}
	cancelFunc()
	wg.Wait()

}

func TestHeaderExtractionMetrics(t *testing.T) {
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverCreateSettings: receivertest.NewNopCreateSettings()})
	nextConsumer := new(consumertest.MetricsSink)
	c := metricsConsumerGroupHandler{
		unmarshaler:  newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
	}
	headers := []string{"headerKey1", "headerKey2"}
	c.headerExtractor = &headerExtractor{
		logger:  zap.NewNop(),
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
				val, ok := rs.Resource().Attributes().Get("kafka.header.headerKey1")
				assert.Equal(t, ok, true)
				assert.Equal(t, val.Str(), "headerValue1")
				val, ok = rs.Resource().Attributes().Get("kafka.header.headerKey2")
				assert.Equal(t, ok, true)
				assert.Equal(t, val.Str(), "headerValue2")
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
				Value: []byte("headerValue1"),
			},
			{
				Key:   []byte("headerKey2"),
				Value: []byte("headerValue2"),
			},
		},
		Value: bts,
	}
	// groupClaim.messageChan <- &sarama.ConsumerMessage{}
	cancelFunc()
	wg.Wait()

}
