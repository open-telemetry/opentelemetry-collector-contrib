// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerreceiver

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/jaegertracing/jaeger-idl/model/v1"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/agent"
	jaegerthrift "github.com/jaegertracing/jaeger-idl/thrift-gen/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	jaegerconvert "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger/jaegerthriftcoverter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/udpserver/thriftudp"
)

var jaegerAgent = component.NewIDWithName(metadata.Type, "agent_test")

func TestJaegerAgentUDP_ThriftCompact(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	testJaegerAgent(t, addr, Protocols{
		ThriftCompactUDP: &ProtocolUDP{
			Endpoint:        addr,
			ServerConfigUDP: defaultServerConfigUDP(),
		},
	})
}

func TestJaegerAgentUDP_ThriftCompact_InvalidPort(t *testing.T) {
	config := Protocols{
		ThriftCompactUDP: &ProtocolUDP{
			Endpoint:        "0.0.0.0:999999",
			ServerConfigUDP: defaultServerConfigUDP(),
		},
	}
	set := receivertest.NewNopSettings(metadata.Type)
	jr, err := newJaegerReceiver(jaegerAgent, config, nil, set)
	require.NoError(t, err)

	assert.Error(t, jr.Start(context.Background(), componenttest.NewNopHost()), "should not have been able to startTraceReception")

	require.NoError(t, jr.Shutdown(context.Background()))
}

func TestJaegerAgentUDP_ThriftBinary(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	testJaegerAgent(t, addr, Protocols{
		ThriftBinaryUDP: &ProtocolUDP{
			Endpoint:        addr,
			ServerConfigUDP: defaultServerConfigUDP(),
		},
	})
}

func TestJaegerAgentUDP_ThriftBinary_PortInUse(t *testing.T) {
	// This test confirms that the thrift binary port is opened correctly.  This is all we can test at the moment.  See above.
	addr := testutil.GetAvailableLocalAddress(t)

	config := Protocols{
		ThriftBinaryUDP: &ProtocolUDP{
			Endpoint:        addr,
			ServerConfigUDP: defaultServerConfigUDP(),
		},
	}
	set := receivertest.NewNopSettings(metadata.Type)
	jr, err := newJaegerReceiver(jaegerAgent, config, nil, set)
	require.NoError(t, err)

	assert.NoError(t, jr.startAgent(), "Start failed")
	t.Cleanup(func() { require.NoError(t, jr.Shutdown(context.Background())) })

	l, err := net.Listen("udp", addr)
	assert.Error(t, err, "should not have been able to listen to the port")

	if l != nil {
		l.Close()
	}
}

func TestJaegerAgentUDP_ThriftBinary_InvalidPort(t *testing.T) {
	config := Protocols{
		ThriftBinaryUDP: &ProtocolUDP{
			Endpoint:        "0.0.0.0:999999",
			ServerConfigUDP: defaultServerConfigUDP(),
		},
	}
	set := receivertest.NewNopSettings(metadata.Type)
	jr, err := newJaegerReceiver(jaegerAgent, config, nil, set)
	require.NoError(t, err)

	assert.Error(t, jr.Start(context.Background(), componenttest.NewNopHost()), "should not have been able to startTraceReception")

	require.NoError(t, jr.Shutdown(context.Background()))
}

func testJaegerAgent(t *testing.T, agentEndpoint string, receiverConfig Protocols) {
	// 1. Create the Jaeger receiver aka "server"
	sink := new(consumertest.TracesSink)
	set := receivertest.NewNopSettings(metadata.Type)
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
	jexp, err := newClientUDP(agentEndpoint, jr.config.ThriftBinaryUDP != nil)
	require.NoError(t, err, "Failed to create the Jaeger OpenTelemetry exporter for the live application")

	// 3. Now finally send some spans
	td := generateTraceData()
	batches := jaeger.ProtoFromTraces(td)
	for _, batch := range batches {
		require.NoError(t, jexp.EmitBatch(context.Background(), modelToThrift(batch)))
	}

	require.Eventually(t, func() bool {
		return sink.SpanCount() > 0
	}, 10*time.Second, 5*time.Millisecond)

	gotTraces := sink.AllTraces()
	require.Len(t, gotTraces, 1)
	assert.Equal(t, td, gotTraces[0])
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
