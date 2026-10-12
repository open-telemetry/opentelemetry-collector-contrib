// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udpserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/udpserver"

import (
	"net"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/agent"
	"github.com/stretchr/testify/require"
)

// TestProcessor_EmptyDatagram verifies that a zero-length UDP datagram does
// not crash the processor goroutine. bytes.Buffer.WriteTo skips the
// destination's Write method when the source buffer is empty, so a freshly
// pooled TBufferedReadTransport keeps a nil readBuf and panics on the first
// Read. Regression test for the nil-pointer dereference reported in
// open-telemetry/opentelemetry-collector-contrib#51973.
func TestProcessor_EmptyDatagram(t *testing.T) {
	for _, test := range []struct {
		factory thrift.TProtocolFactory
		name    string
	}{
		{compactFactory, "compact"},
		{binaryFactory, "binary"},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := &fakeAgentHandler{}

			hostPort, _ := createProcessor(t, test.factory, agent.NewAgentProcessor(h))

			conn, err := net.Dial("udp", hostPort)
			require.NoError(t, err)
			defer conn.Close()

			// A zero-length datagram: enqueued, then handed to the processor.
			_, err = conn.Write([]byte{})
			require.NoError(t, err)

			// The datagram is enqueued ahead of any later one, so the
			// processor consumes the empty payload first. Pre-fix this
			// panics in the unrecovered goroutine and kills the test
			// process.
			client, clientCloser, err := newJaegerThriftUDPClient(hostPort, test.factory)
			require.NoError(t, err)
			defer clientCloser.Close()

			require.NoError(t, client.EmitBatch(t.Context(), batch))
			require.Eventually(t,
				func() bool { return len(h.GetJaegerBatches()) == 1 },
				5*time.Second,
				10*time.Millisecond,
				"server should still process batches after an empty datagram")
		})
	}
}
