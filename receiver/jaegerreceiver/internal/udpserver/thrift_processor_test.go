// Copyright The OpenTelemetry Authors
// Copyright (c) 2019 The Jaeger Authors.
// Copyright (c) 2017 Uber Technologies, Inc.
// SPDX-License-Identifier: Apache-2.0

package udpserver

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/agent"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/jaeger"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/zipkincore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/udpserver/thriftudp"
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

func createProcessor(t *testing.T, tFactory thrift.TProtocolFactory, handler AgentProcessor) (string, *ThriftProcessor) {
	transport, err := thriftudp.NewTUDPServerTransport("127.0.0.1:0")
	require.NoError(t, err)

	queueSize := 10
	maxPacketSize := 65000
	server := NewUDPServer(transport, queueSize, maxPacketSize)

	numProcessors := 1
	processor, err := NewThriftProcessor(server, numProcessors, tFactory, handler, zaptest.NewLogger(t))
	require.NoError(t, err)

	go processor.Serve()
	for i := 0; i < 1000; i++ {
		if processor.IsServing() {
			break
		}
		time.Sleep(10 * time.Microsecond)
	}
	require.True(t, processor.IsServing(), "processor must be serving")

	t.Cleanup(func() {
		processor.Stop()
	})

	return transport.Addr().String(), processor
}

func TestNewThriftProcessor_ZeroCount(t *testing.T) {
	_, err := NewThriftProcessor(nil, 0, nil, nil, zaptest.NewLogger(t))
	require.EqualError(t, err, "number of processors must be greater than 0, called with 0")
}

type failingHandler struct {
	err error
}

func (h failingHandler) Process(context.Context, thrift.TProtocol /* iprot */, thrift.TProtocol /* oprot */) (success bool, err thrift.TException) {
	return false, thrift.NewTApplicationException(0, h.err.Error())
}

func TestProcessor_HandlerError(t *testing.T) {
	handler := failingHandler{err: errors.New("doh")}

	hostPort, processor := createProcessor(t, compactFactory, handler)
	defer processor.Stop()

	client, clientCloser, err := newJaegerThriftUDPClient(hostPort, compactFactory)
	require.NoError(t, err)
	defer clientCloser.Close()

	err = client.EmitBatch(context.Background(), &jaeger.Batch{
		Process: jaeger.NewProcess(),
		Spans:   []*jaeger.Span{{OperationName: testSpanName}},
	})
	require.NoError(t, err)
}

// NewJaegerThriftUDPClient creates a new jaeger agent client that works like Jaeger client
func newJaegerThriftUDPClient(hostPort string, protocolFactory thrift.TProtocolFactory) (*agent.AgentClient, io.Closer, error) {
	clientTransport, err := thriftudp.NewTUDPClientTransport(hostPort, "")
	if err != nil {
		return nil, nil, err
	}

	client := agent.NewAgentClientFactory(clientTransport, protocolFactory)
	return client, clientTransport, nil
}

type fakeAgentHandler struct {
	mux     sync.Mutex
	batches []*jaeger.Batch
}

func (*fakeAgentHandler) EmitZipkinBatch(_ context.Context, _ []*zipkincore.Span) error {
	return errors.ErrUnsupported
}

func (h *fakeAgentHandler) EmitBatch(_ context.Context, batch *jaeger.Batch) error {
	h.mux.Lock()
	defer h.mux.Unlock()
	h.batches = append(h.batches, batch)
	return nil
}

func (h *fakeAgentHandler) GetJaegerBatches() []*jaeger.Batch {
	h.mux.Lock()
	defer h.mux.Unlock()
	return append([]*jaeger.Batch{}, h.batches...)
}

// TestJaegerProcessor instantiates a real UDP receiver and a real gRPC collector
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
			h := &fakeAgentHandler{}

			hostPort, _ := createProcessor(t, test.factory, agent.NewAgentProcessor(h))

			client, clientCloser, err := newJaegerThriftUDPClient(hostPort, test.factory)
			require.NoError(t, err)
			defer clientCloser.Close()

			require.NoError(t, client.EmitBatch(context.Background(), batch))
			require.Eventually(t,
				func() bool { return len(h.GetJaegerBatches()) == 1 },
				5*time.Second,
				time.Millisecond,
				"server should have received spans")
			assert.Equal(t, testSpanName, h.GetJaegerBatches()[0].Spans[0].OperationName)
		})
	}
}
