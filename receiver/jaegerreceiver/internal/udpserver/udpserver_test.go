// Copyright The OpenTelemetry Authors
// Copyright (c) 2019 The Jaeger Authors.
// Copyright (c) 2017 Uber Technologies, Inc.
// SPDX-License-Identifier: Apache-2.0

package udpserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/udpserver/thriftudp"
)

func TestTBufferedServerSendReceive(t *testing.T) {
	transport, err := thriftudp.NewTUDPServerTransport("127.0.0.1:0")
	require.NoError(t, err)

	const maxPacketSize = 65000
	const maxQueueSize = 100
	server := NewUDPServer(transport, maxQueueSize, maxPacketSize)
	go server.Serve()
	defer server.Stop()

	client, err := thriftudp.NewTUDPClientTransport(transport.Addr().String(), "")
	require.NoError(t, err)
	defer client.Close()

	// keep sending packets until the server receives one
	for range 1000 {
		n, err := client.Write([]byte("span1"))
		require.NoError(t, err)
		require.Equal(t, 5, n)
		require.NoError(t, client.Flush(context.Background()))

		select {
		case buf := <-server.DataChan():
			assert.Positive(t, buf.Len())
			assert.Equal(t, "span1", buf.String())
			return // exit test on successful receipt
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	t.Fatal("server did not receive packets")
}
