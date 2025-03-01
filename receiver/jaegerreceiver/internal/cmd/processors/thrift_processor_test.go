// Copyright The OpenTelemetry Authors
// Copyright (c) 2019 The Jaeger Authors.
// Copyright (c) 2017 Uber Technologies, Inc.
// SPDX-License-Identifier: Apache-2.0

package processors

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/jaegertracing/jaeger-idl/thrift-gen/agent"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/jaeger"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/zipkincore"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/cmd/servers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/cmd/servers/thriftudp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/cmd/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/internal/metricstest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/pkg/metrics"
)

// TODO make these tests faster, they take almost 4 seconds

var (
	compactFactory = thrift.NewTCompactProtocolFactoryConf(&thrift.TConfiguration{})
	binaryFactory  = thrift.NewTBinaryProtocolFactoryConf(&thrift.TConfiguration{})

	testSpanName = "span1"

	batch = &jaeger.Batch{
		Process: jaeger.NewProcess(),
		Spans:   []*jaeger.Span{{OperationName: testSpanName}},
	}
)

func createProcessor(t *testing.T, mFactory metrics.Factory, tFactory thrift.TProtocolFactory, handler AgentProcessor) (string, Processor) {
	transport, err := thriftudp.NewTUDPServerTransport("127.0.0.1:0")
	require.NoError(t, err)

	queueSize := 10
	maxPacketSize := 65000
	server, err := servers.NewTBufferedServer(transport, queueSize, maxPacketSize, mFactory)
	require.NoError(t, err)

	numProcessors := 1
	processor, err := NewThriftProcessor(server, numProcessors, mFactory, tFactory, handler, zaptest.NewLogger(t))
	require.NoError(t, err)

	go processor.Serve()
	for i := 0; i < 1000; i++ {
		if processor.IsServing() {
			break
		}
		time.Sleep(10 * time.Microsecond)
	}
	require.True(t, processor.IsServing(), "processor must be serving")

	return transport.Addr().String(), processor
}

// revive:disable-next-line function-result-limit
func initCollectorAndReporter(t *testing.T) (*metricstest.Factory, *testutils.MockCollector, *agent.AgentProcessor) {
	mockCollector := testutils.StartMockCollector(t)
	logger := zaptest.NewLogger(t)
	
	reporter := testutils.NewMockReporter(mockCollector, logger)
	metricsFactory := metricstest.NewFactory(0)
	
	agentProcessor := agent.NewAgentProcessor(reporter)
	
	return metricsFactory, mockCollector, agentProcessor
}
func TestNewThriftProcessor_ZeroCount(t *testing.T) {
	_, err := NewThriftProcessor(nil, 0, nil, nil, nil, zaptest.NewLogger(t))
	require.EqualError(t, err, "number of processors must be greater than 0, called with 0")
}

func TestProcessorWithCompactZipkin(t *testing.T) {
	metricsFactory, collector, agentProcessor := initCollectorAndReporter(t)
	defer collector.Close()

	hostPort, processor := createProcessor(t, metricsFactory, compactFactory, agentProcessor)
	defer processor.Stop()

	client, clientCloser, err := testutils.NewZipkinThriftUDPClient(hostPort)
	require.NoError(t, err)
	defer clientCloser.Close()

	span := zipkincore.NewSpan()
	span.Name = testSpanName
	span.Annotations = []*zipkincore.Annotation{{Value: zipkincore.CLIENT_SEND, Host: &zipkincore.Endpoint{ServiceName: "foo"}}}

	err = client.EmitZipkinBatch(context.Background(), []*zipkincore.Span{span})
	require.NoError(t, err)

	assertZipkinProcessorCorrectness(t, collector, metricsFactory)
}


type failingHandler struct {
	err error
}

func (h failingHandler) Process(context.Context, thrift.TProtocol /* iprot */, thrift.TProtocol /* oprot */) (success bool, err thrift.TException) {
	return false, thrift.NewTApplicationException(0, h.err.Error())
}

func TestProcessor_HandlerError(t *testing.T) {
	metricsFactory := metricstest.NewFactory(0)

	handler := failingHandler{err: errors.New("doh")}

	hostPort, processor := createProcessor(t, metricsFactory, compactFactory, handler)
	defer processor.Stop()

	client, clientCloser, err := testutils.NewZipkinThriftUDPClient(hostPort)
	require.NoError(t, err)
	defer clientCloser.Close()

	err = client.EmitZipkinBatch(context.Background(), []*zipkincore.Span{{Name: testSpanName}})
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		c, _ := metricsFactory.Snapshot()
		if _, ok := c["thrift.udp.t-processor.handler-errors"]; ok {
			break
		}
		time.Sleep(time.Millisecond)
	}

	metricsFactory.AssertCounterMetrics(t,
		metricstest.ExpectedMetric{Name: "thrift.udp.t-processor.handler-errors", Value: 1},
		metricstest.ExpectedMetric{Name: "thrift.udp.server.packets.processed", Value: 1},
	)
}

// TestJaegerProcessor instantiates a real UDP receiver and a mock collector
// and executes end-to-end batch submission.
func TestJaegerProcessor(t *testing.T) {
	tests := []struct {
		factory thrift.TProtocolFactory
		name    string
	}{
		{compactFactory, "compact"},
		{binaryFactory, "binary"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metricsFactory, collector, agentProcessor := initCollectorAndReporter(t)
			defer collector.Close()

			hostPort, processor := createProcessor(t, metricsFactory, test.factory, agentProcessor)
			defer processor.Stop()

			client, clientCloser, err := testutils.NewJaegerThriftUDPClient(hostPort, test.factory)
			require.NoError(t, err)
			defer clientCloser.Close()

			err = client.EmitBatch(context.Background(), batch)
			require.NoError(t, err)

			assertJaegerProcessorCorrectness(t, collector, metricsFactory)
		})
	}
}
func assertJaegerProcessorCorrectness(t *testing.T, collector *testutils.MockCollector, metricsFactory *metricstest.Factory) {
	sizeF := func() int {
		return len(collector.GetJaegerBatches())
	}
	nameF := func() string {
		return collector.GetJaegerBatches()[0].Spans[0].OperationName
	}
	assertCollectorReceivedData(t, metricsFactory, sizeF, nameF, "jaeger")
}

func assertZipkinProcessorCorrectness(t *testing.T, collector *testutils.MockCollector, metricsFactory *metricstest.Factory) {
	sizeF := func() int {
		return len(collector.GetJaegerBatches())
	}
	nameF := func() string {
		return collector.GetJaegerBatches()[0].Spans[0].OperationName
	}
	assertCollectorReceivedData(t, metricsFactory, sizeF, nameF, "zipkin")
}

// Simplify the metrics assertions as needed
func assertCollectorReceivedData(
	t *testing.T,
	metricsFactory *metricstest.Factory,
	sizeF func() int,
	nameF func() string,
	format string,
) {
	require.Eventually(t,
		func() bool { return sizeF() == 1 },
		5*time.Second,
		time.Millisecond,
		"server should have received spans")
	assert.Equal(t, testSpanName, nameF())

	// Only check server packet metrics since we removed the reporter metrics
	metricsFactory.AssertCounterMetrics(t, []metricstest.ExpectedMetric{
		{Name: "thrift.udp.server.packets.processed", Value: 1},
	}...)
}