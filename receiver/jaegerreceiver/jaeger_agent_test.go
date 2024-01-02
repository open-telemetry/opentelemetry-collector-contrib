// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerreceiver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/jaegertracing/jaeger/cmd/agent/app/servers/thriftudp"
	"github.com/jaegertracing/jaeger/model"
	jaegerconvert "github.com/jaegertracing/jaeger/model/converter/thrift/jaeger"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/jaegertracing/jaeger/thrift-gen/agent"
	jaegerthrift "github.com/jaegertracing/jaeger/thrift-gen/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/metadata"
)

var jaegerAgent = component.NewIDWithName(metadata.Type, "agent_test")

func TestJaegerAgentUDP_ThriftCompact(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	testJaegerAgent(t, addr, &configuration{
		AgentCompactThrift: ProtocolUDP{
			Endpoint:        addr,
			ServerConfigUDP: defaultServerConfigUDP(),
		},
	})
}

func TestJaegerAgentUDP_ThriftCompact_InvalidPort(t *testing.T) {
	config := &configuration{
		AgentCompactThrift: ProtocolUDP{
			Endpoint:        "0.0.0.0:999999",
			ServerConfigUDP: defaultServerConfigUDP(),
		},
	}
	set := receivertest.NewNopCreateSettings()
	jr, err := newJaegerReceiver(jaegerAgent, config, nil, set)
	require.NoError(t, err)

	assert.Error(t, jr.Start(context.Background(), componenttest.NewNopHost()), "should not have been able to startTraceReception")

	require.NoError(t, jr.Shutdown(context.Background()))
}

func TestJaegerAgentUDP_ThriftBinary(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	testJaegerAgent(t, addr, &configuration{
		AgentBinaryThrift: ProtocolUDP{
			Endpoint:        addr,
			ServerConfigUDP: defaultServerConfigUDP(),
		},
	})
}

func TestJaegerAgentUDP_ThriftBinary_PortInUse(t *testing.T) {
	// This test confirms that the thrift binary port is opened correctly.  This is all we can test at the moment.  See above.
	addr := testutil.GetAvailableLocalAddress(t)

	config := &configuration{
		AgentBinaryThrift: ProtocolUDP{
			Endpoint:        addr,
			ServerConfigUDP: defaultServerConfigUDP(),
		},
	}
	set := receivertest.NewNopCreateSettings()
	jr, err := newJaegerReceiver(jaegerAgent, config, nil, set)
	require.NoError(t, err)

	assert.NoError(t, jr.startAgent(componenttest.NewNopHost()), "Start failed")
	t.Cleanup(func() { require.NoError(t, jr.Shutdown(context.Background())) })

	l, err := net.Listen("udp", addr)
	assert.Error(t, err, "should not have been able to listen to the port")

	if l != nil {
		l.Close()
	}
}

func TestJaegerAgentUDP_ThriftBinary_InvalidPort(t *testing.T) {
	config := &configuration{
		AgentBinaryThrift: ProtocolUDP{
			Endpoint:        "0.0.0.0:999999",
			ServerConfigUDP: defaultServerConfigUDP(),
		},
	}
	set := receivertest.NewNopCreateSettings()
	jr, err := newJaegerReceiver(jaegerAgent, config, nil, set)
	require.NoError(t, err)

	assert.Error(t, jr.Start(context.Background(), componenttest.NewNopHost()), "should not have been able to startTraceReception")

	require.NoError(t, jr.Shutdown(context.Background()))
}

func initializeGRPCTestServer(t *testing.T, beforeServe func(server *grpc.Server), opts ...grpc.ServerOption) (*grpc.Server, net.Addr) {
	server := grpc.NewServer(opts...)
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	beforeServe(server)
	go func() {
		err := server.Serve(lis)
		require.NoError(t, err)
	}()
	return server, lis.Addr()
}

type mockSamplingHandler struct {
}

func (*mockSamplingHandler) GetSamplingStrategy(context.Context, *api_v2.SamplingStrategyParameters) (*api_v2.SamplingStrategyResponse, error) {
	return &api_v2.SamplingStrategyResponse{StrategyType: api_v2.SamplingStrategyType_PROBABILISTIC}, nil
}

func TestJaegerHTTP(t *testing.T) {
	s, _ := initializeGRPCTestServer(t, func(s *grpc.Server) {
		api_v2.RegisterSamplingManagerServer(s, &mockSamplingHandler{})
	})
	defer s.GracefulStop()

	endpoint := testutil.GetAvailableLocalAddress(t)
	config := &configuration{
		AgentHTTPEndpoint: endpoint,
	}
	set := receivertest.NewNopCreateSettings()
	jr, err := newJaegerReceiver(jaegerAgent, config, nil, set)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, jr.Shutdown(context.Background())) })

	assert.NoError(t, jr.Start(context.Background(), componenttest.NewNopHost()), "Start failed")

	// allow http server to start
	assert.Eventually(t, func() bool {
		var conn net.Conn
		conn, err = net.Dial("tcp", endpoint)
		if err == nil && conn != nil {
			conn.Close()
			return true
		}
		return false
	}, 10*time.Second, 5*time.Millisecond, "failed to wait for the port to be open")

	resp, err := http.Get(fmt.Sprintf("http://%s/sampling?service=test", endpoint))
	assert.NoError(t, err, "should not have failed to make request")
	if resp != nil {
		assert.Equal(t, 500, resp.StatusCode, "should have returned 200")
		return
	}
	t.Fail()
}

func testJaegerAgent(t *testing.T, agentEndpoint string, receiverConfig *configuration) {
	// 1. Create the Jaeger receiver aka "server"
	sink := new(consumertest.TracesSink)
	set := receivertest.NewNopCreateSettings()
	jr, err := newJaegerReceiver(jaegerAgent, receiverConfig, sink, set)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, jr.Shutdown(context.Background())) })

	for i := 0; i < 3; i++ {
		err = jr.Start(context.Background(), componenttest.NewNopHost())
		if err == nil {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}
	require.NoError(t, err, "Start failed")

	// 2. Then send spans to the Jaeger receiver.
	jexp, err := newClientUDP(agentEndpoint, jr.config.AgentBinaryThrift.Endpoint != "")
	require.NoError(t, err, "Failed to create the Jaeger OpenTelemetry exporter for the live application")

	// 3. Now finally send some spans
	td := generateTraceData()
	batches, err := jaeger.ProtoFromTraces(td)
	require.NoError(t, err)
	for _, batch := range batches {
		require.NoError(t, jexp.EmitBatch(context.Background(), modelToThrift(batch)))
	}

	require.Eventually(t, func() bool {
		return sink.SpanCount() > 0
	}, 10*time.Second, 5*time.Millisecond)

	gotTraces := sink.AllTraces()
	require.Equal(t, 1, len(gotTraces))
	assert.EqualValues(t, td, gotTraces[0])
}

func newClientUDP(hostPort string, binary bool) (*agent.AgentClient, error) {
	clientTransport, err := thriftudp.NewTUDPClientTransport(hostPort, "")
	if err != nil {
		return nil, err
	}
	var protocolFactory thrift.TProtocolFactory
	if binary {
		protocolFactory = thrift.NewTBinaryProtocolFactoryConf(nil)
	} else {
		protocolFactory = thrift.NewTCompactProtocolFactoryConf(nil)
	}
	return agent.NewAgentClientFactory(clientTransport, protocolFactory), nil
}

// Cannot use the testdata because timestamps are nanoseconds.
func generateTraceData() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr(conventions.AttributeServiceName, "test")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7})
	span.SetTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 7, 6, 5, 4, 3, 2, 1, 0})
	span.SetStartTimestamp(1581452772000000000)
	span.SetEndTimestamp(1581452773000000000)
	return td
}

func modelToThrift(batch *model.Batch) *jaegerthrift.Batch {
	return &jaegerthrift.Batch{
		Process: processModelToThrift(batch.Process),
		Spans:   jaegerconvert.FromDomain(batch.Spans),
	}
}

func processModelToThrift(process *model.Process) *jaegerthrift.Process {
	if process == nil {
		return nil
	}
	return &jaegerthrift.Process{
		ServiceName: process.ServiceName,
	}
}
